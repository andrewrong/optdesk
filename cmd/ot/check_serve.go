package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

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
}

// checkRepoDirEnv names the checkout whose contracts/schemas the check loads (default "."). The flag of the
// design (--repo-dir) is parsed in check.go, which this change does not edit.
const checkRepoDirEnv = "OT_REPO_DIR"

// serveWindowDays is how many calendar days back the closed windows are read for the freshness lines.
const serveWindowDays = 7

// siteMaker builds the site handler around the check's own NewHandlerFunc.
type siteMaker func(newHandler pages.NewHandlerFunc) http.Handler

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
	latest, ferr := latestClosedExpected(ctx, e, a.deps.newFwClock().Now())
	repo := a.getenv(checkRepoDirEnv)
	if repo == "" {
		repo = "."
	}
	checkServeRoutes(ctx, e, repo, latest, ferr, mk)
}

// latestClosedExpected is the latest ExpectedDay of the windows of the last serveWindowDays days that are closed
// at now (DueBy not after now, and no closed-day row). The due time of a window already includes the lateness
// of its dataset, so a page older than this day is stale. It is an error when there is none: a dead scheduler.
func latestClosedExpected(ctx context.Context, e *checkEnv, now time.Time) (contracts.Day, error) {
	g, err := e.openGaps(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = g.Close() }()
	to := contracts.Day(now.UTC().Format(dayLayout))
	from := contracts.Day(now.UTC().AddDate(0, 0, -serveWindowDays).Format(dayLayout))
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

// checkServeRoutes calls every route on the handler mk builds and emits its lines. latest and ferr come from
// latestClosedExpected.
func checkServeRoutes(ctx context.Context, e *checkEnv, repoDir string, latest contracts.Day, ferr error, mk siteMaker) {
	h := mk(func(vm []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(vm)
		})
	})
	sch := newSchemaSet(repoDir)
	for _, rt := range serveRoutes {
		checkOneRoute(ctx, e, h, sch, rt, latest, ferr)
	}
}

func checkOneRoute(ctx context.Context, e *checkEnv, h http.Handler, sch *schemaSet, rt serveRoute, latest contracts.Day, ferr error) {
	name := "serve." + rt.label
	fail := func(suffix, reason string) {
		e.c.emit(Result{Status: statusFail, Check: name + suffix, Reason: reason})
	}
	if rt.page == "report_page" && e.a.profile.Zone == nil {
		fail(".status", "report_page needs OT_ZONE")
		return
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, rt.path, nil))
	if rec.Code != http.StatusOK {
		fail(".status", fmt.Sprintf("%s answered %d: %s", rt.page, rec.Code, strings.TrimSpace(rec.Body.String())))
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
	} else if err := sch.validate(declared, top["schema_version"], body); err != nil {
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
	emitFresh(e, name+".fresh", day, haveDay, latest, ferr)
}

// emitFresh is the freshness line of a page: its data day against the latest closed window.
func emitFresh(e *checkEnv, name string, day contracts.Day, haveDay bool, latest contracts.Day, ferr error) {
	switch {
	case ferr != nil:
		e.c.emit(Result{Status: statusFail, Check: name, Day: string(day), Reason: ferr.Error()})
	case !haveDay:
		e.c.emit(Result{Status: statusExplained, Check: name, Reason: "no_view: the page declares no data day"})
	case day < latest:
		e.c.emit(Result{Status: statusFail, Check: name, Day: string(day), Reason: "older than the latest closed window " + string(latest)})
	default:
		e.c.emit(Result{Status: statusPass, Check: name, Day: string(day), Reason: "not older than the latest closed window " + string(latest)})
	}
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
