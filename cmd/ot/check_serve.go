package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
)

func init() { checkServices["serve"] = checkServe }

// serveRoute is one call of the check: a route of siteHandler and the page it must serve.
type serveRoute struct {
	label string // the check name after "serve."
	path  string // path and query
	page  string // the schema name the body declares
}

// serveRoutes are the routes of siteHandler that serve a page of the table in check_blocks.json. /report is
// called for every variant: market us|jp and slot open|close. macro_page has no route and is not here.
var serveRoutes = []serveRoute{
	{"report_page.us.open", "/report?market=us&slot=open", "report_page"},
	{"report_page.us.close", "/report?market=us&slot=close", "report_page"},
	{"report_page.jp.open", "/report?market=jp&slot=open", "report_page"},
	{"report_page.jp.close", "/report?market=jp&slot=close", "report_page"},
	{"market_page", "/market", "market_page"},
	{"macro_rates", "/macro", "macro_rates"},
	{"option_candidates_page", "/" + siteCandidatesRoute, "option_candidates_page"},
	{"option_notices_page", "/" + siteNoticesRoute, "option_notices_page"},
	{"option_tracking_page", "/" + siteTrackingRoute, "option_tracking_page"},
	{"option_data_page", "/options/data", "option_data_page"},
	{"settings_page", "/" + siteSettingsRoute, "settings_page"},
	{"underlyings", "/" + siteUnderlyingsRoute, "underlyings"},
}

// serveWindowDays is how many calendar days back the closed windows are read for the freshness lines.
const serveWindowDays = 7

// siteMaker builds the site handler around the check's own NewHandlerFunc.
type siteMaker func(newHandler pages.NewHandlerFunc) http.Handler

// diagnoseFunc builds the page of a route again and returns the build error, which siteHandler hides behind a
// fixed 500 text. It is called only for a route that answered 500.
type diagnoseFunc func(ctx context.Context, rt serveRoute) error

// freshness is the reference of the freshness lines: the latest closed window, or why there is none.
type freshness struct {
	latest contracts.Day
	err    error
}

// checkServe is the service `ot check serve`: the readers of the pages role, the siteHandler on them, called in
// process (no port), one set of lines per route.
func checkServe(ctx context.Context, e *checkEnv) {
	a := e.a
	if a.profile.Name == "demo" {
		e.c.emit(Result{Status: statusFail, Check: "serve", Reason: "the demo profile is refused: it serves synthetic data"})
		return
	}
	rd, closeFn, err := openSiteReaders(ctx, a)
	if err != nil {
		e.c.emit(Result{Status: statusFail, Check: "serve", Reason: err.Error()})
		return
	}
	defer closeFn()
	clk := a.deps.newClock()
	mk := func(nh pages.NewHandlerFunc) http.Handler {
		return siteHandler(a, clk, "", rd, nh, http.NotFoundHandler())
	}
	diagnose := func(ctx context.Context, rt serveRoute) error {
		_, err := buildRoute(ctx, a, clk, rd, rt)
		return err
	}
	latest, ferr := latestClosedExpected(ctx, e, a.deps.newFwClock().Now())
	repo := e.opts.RepoDir
	if repo == "" {
		repo = "."
	}
	checkServeRoutes(ctx, e, repo, freshness{latest, ferr}, mk, diagnose)
}

// buildRoute runs the builder the route of siteHandler runs, with the arguments siteHandler gives it.
func buildRoute(ctx context.Context, a *app, clk clock.Clock, rd siteReaders, rt serveRoute) ([]byte, error) {
	switch rt.page {
	case "report_page":
		u, err := url.Parse(rt.path)
		if err != nil {
			return nil, err
		}
		return buildReport(ctx, a, clk, "", rd.view, u.Query())
	case "market_page":
		return buildMarket(ctx, a, clk, "", rd)
	case "macro_rates":
		return buildMacro(ctx, a, clk, "", rd)
	case "option_candidates_page":
		return buildCandidates(ctx, a, clk, "", rd)
	case "option_notices_page":
		return buildNotices(ctx, a, clk, "", rd)
	case "option_tracking_page":
		return buildTracking(ctx, a, clk, "", rd)
	case "option_data_page":
		return buildSite(ctx, a, clk, "", rd, 0, false)
	case "settings_page":
		return buildSettings(ctx, a, clk, "", rd, savedNotice(url.Values{}))
	case "underlyings":
		return buildUnderlyingsPage(ctx, a, clk, "", rd, underlyingsExtras{})
	}
	return nil, fmt.Errorf("no builder for %s", rt.page)
}

// latestClosedExpected is the latest ExpectedDay of the windows of the last serveWindowDays calendar days, in the
// zone of OT_ZONE, that are closed at now (DueBy not after now, and no closed-day row). The due time of a window
// already includes the lateness of its dataset, so a page older than this day is stale. It is an error when there
// is none: a dead scheduler.
func latestClosedExpected(ctx context.Context, e *checkEnv, now time.Time) (contracts.Day, error) {
	g, err := e.openGaps(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = g.Close() }()
	loc := e.a.profile.Zone
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	to := contracts.Day(local.Format(dayLayout))
	from := contracts.Day(local.AddDate(0, 0, -serveWindowDays).Format(dayLayout))
	exps, err := g.Expectations(ctx, from, to)
	if err != nil {
		return "", errors.New("read expectations failed")
	}
	var latest contracts.Day
	for _, x := range exps {
		if !x.Closed && !x.DueBy.After(now) && x.ExpectedDay > latest {
			latest = x.ExpectedDay
		}
	}
	if latest == "" {
		return "", errors.New("no closed window in the last 7 days")
	}
	return latest, nil
}

// routeChecker calls the routes of one handler and emits their lines.
type routeChecker struct {
	e        *checkEnv
	h        http.Handler
	sch      *schemaSet
	fresh    freshness
	diagnose diagnoseFunc
}

// checkServeRoutes calls every route on the handler mk builds and emits its lines. diagnose may be nil.
func checkServeRoutes(ctx context.Context, e *checkEnv, repoDir string, fresh freshness, mk siteMaker, diagnose diagnoseFunc) {
	h := mk(func(vm []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(vm)
		})
	})
	rc := &routeChecker{e: e, h: h, sch: newSchemaSet(repoDir), fresh: fresh, diagnose: diagnose}
	for _, rt := range serveRoutes {
		rc.check(ctx, rt)
	}
}

// statusReason is the reason of a route that did not answer 200: the page, the code and, for a 500, the build error.
func (rc *routeChecker) statusReason(ctx context.Context, rt serveRoute, code int, text string) string {
	reason := fmt.Sprintf("%s answered %d: %s", rt.page, code, text)
	if code == http.StatusInternalServerError && rc.diagnose != nil {
		if err := rc.diagnose(ctx, rt); err != nil {
			reason += ": build error: " + err.Error()
		}
	}
	return reason
}

func (rc *routeChecker) check(ctx context.Context, rt serveRoute) {
	e := rc.e
	name := "serve." + rt.label
	fail := func(suffix, reason string) {
		e.c.emit(Result{Status: statusFail, Check: name + suffix, Reason: reason})
	}
	if rt.page == "report_page" && e.a.profile.Zone == nil {
		fail(".status", "report_page needs OT_ZONE")
		return
	}
	rec := httptest.NewRecorder()
	rc.h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, rt.path, nil))
	if rec.Code != http.StatusOK {
		fail(".status", rc.statusReason(ctx, rt, rec.Code, strings.TrimSpace(rec.Body.String())))
		return
	}
	body := rec.Body.Bytes()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		fail(".status", rt.page+" body is not a JSON object")
		return
	}
	e.c.emit(Result{Status: statusPass, Check: name + ".status", Reason: "200"})
	var declared string
	_ = json.Unmarshal(top["schema"], &declared)
	if declared != rt.page {
		fail(".schema", fmt.Sprintf("body declares %q, the route serves %s", declared, rt.page))
	} else if err := rc.sch.validate(declared, top["schema_version"], body); err != nil {
		fail(".schema", err.Error())
	} else {
		e.c.emit(Result{Status: statusPass, Check: name + ".schema", Reason: "valid against " + declared})
	}
	day, haveDay := pageDataDay(rt.page, top)
	diffs := compareBlocks(rt.page, top)
	for _, d := range diffs {
		e.c.emit(Result{Status: statusFail, Check: name + ".blocks", Day: string(day), Reason: d})
	}
	if len(diffs) == 0 {
		e.c.emit(Result{Status: statusPass, Check: name + ".blocks", Day: string(day), Reason: "blocks equal the table"})
	}
	e.c.emit(rc.fresh.line(name+".fresh", day, haveDay))
}

// line is the freshness line of a page: its data day against the latest closed window.
func (f freshness) line(name string, day contracts.Day, haveDay bool) Result {
	switch {
	case f.err != nil:
		return Result{Status: statusFail, Check: name, Day: string(day), Reason: f.err.Error()}
	case !haveDay:
		return Result{Status: statusExplained, Check: name, Reason: "no_view: the page declares no data day"}
	case day < f.latest:
		return Result{Status: statusFail, Check: name, Day: string(day), Reason: "older than the latest closed window " + string(f.latest)}
	}
	return Result{Status: statusPass, Check: name, Day: string(day), Reason: "not older than the latest closed window " + string(f.latest)}
}

// schemaBase is the $id prefix of the schemas of contracts/schemas.
const schemaBase = "https://optdesk.invalid/schemas/"

var schemaNameRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// schemaSet compiles the page schemas of the repository checkout; the binary embeds none.
type schemaSet struct {
	dir string
	c   *jsonschema.Compiler
}

func newSchemaSet(repoDir string) *schemaSet {
	s := &schemaSet{dir: filepath.Join(repoDir, "contracts", "schemas"), c: jsonschema.NewCompiler()}
	s.c.UseLoader(s)
	return s
}

// Load implements jsonschema.URLLoader: a schema URL under schemaBase is a file of the checkout.
func (s *schemaSet) Load(url string) (any, error) {
	name, ok := strings.CutPrefix(url, schemaBase)
	if !ok || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return nil, fmt.Errorf("schema %q is outside contracts/schemas", url)
	}
	f, err := os.Open(filepath.Join(s.dir, name))
	if err != nil {
		return nil, fmt.Errorf("schema file %s: not readable in %s", name, s.dir)
	}
	defer func() { _ = f.Close() }()
	return jsonschema.UnmarshalJSON(f)
}

// validate checks body against page.<schema>.v<version>.json, the version the page declares.
func (s *schemaSet) validate(schema string, version json.RawMessage, body []byte) error {
	var v int
	if err := json.Unmarshal(version, &v); err != nil || v < 1 || !schemaNameRe.MatchString(schema) {
		return errors.New("the page declares no schema version")
	}
	sc, err := s.c.Compile(fmt.Sprintf("%spage.%s.v%d.json", schemaBase, schema, v))
	if err != nil {
		return fmt.Errorf("schema page.%s.v%d: %v", schema, v, err)
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	if err := sc.Validate(inst); err != nil {
		return fmt.Errorf("schema page.%s.v%d: %v", schema, v, err)
	}
	return nil
}
