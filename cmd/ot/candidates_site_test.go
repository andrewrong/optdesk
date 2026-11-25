package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
	"github.com/andrewrong/optdesk/internal/compute/ruleparam"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/andrewrong/optdesk/internal/store/optionday"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/store/strategyresultpg"
)

// candidatesFixture is an app whose optionCandidatesPage fake builds {"schema":"option_candidates_page"} and
// records the reader and the days it was given.
type candidatesFixture struct {
	a        *app
	builds   atomic.Int32
	days     []contracts.Day
	got      store.CandidateReader
	gotNames store.InstrumentNameReader
	gotAll   candidateReaders
	err      error
}

func newCandidatesFixture(t *testing.T, env string) *candidatesFixture {
	t.Helper()
	f := &candidatesFixture{}
	f.a, _ = testApp(t, env)
	f.a.deps.newRuleParams = ruleparam.New
	f.a.deps.optionCandidatesPage = func(_ *time.Location, _, _, _ string, r candidateReaders) pages.Page {
		f.gotAll = r
		f.got = r.Cands
		f.gotNames = r.Names
		build := func(_ context.Context, _ store.SeriesReader, day contracts.Day, _ time.Time) ([]byte, error) {
			f.builds.Add(1)
			f.days = append(f.days, day)
			if f.err != nil {
				return nil, f.err
			}
			return []byte(`{"schema":"option_candidates_page"}`), nil
		}
		return pages.Page{Meta: pages.Meta{Name: candidatesName}, Build: build}
	}
	return f
}

func candidatesGet(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return rec
}

var candidatesClock = fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}

func TestCandidatesRouteServesPage(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	reader := demoCandidates{}
	h := siteHandler(f.a, candidatesClock, "2026-11-05", siteReaders{cands: reader}, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_candidates_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	if f.got != reader || len(f.days) != 1 || f.days[0] != "2026-11-05" {
		t.Fatalf("reader %v days %v", f.got, f.days)
	}
	// Without --day the day is today in the zone of the clock.
	h = siteHandler(f.a, candidatesClock, "", siteReaders{cands: reader}, echoHandler, http.NotFoundHandler())
	if rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute); rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	if len(f.days) != 2 || f.days[1] != "2026-11-06" {
		t.Fatalf("days %v", f.days)
	}
}

func TestCandidatesRouteOnlyGet(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	h := siteHandler(f.a, candidatesClock, "2026-11-05", siteReaders{cands: demoCandidates{}}, echoHandler, http.NotFoundHandler())
	if rec := candidatesGet(t, h, http.MethodHead, "/"+siteCandidatesRoute); rec.Code != 200 {
		t.Errorf("HEAD: %d", rec.Code)
	}
	before := f.builds.Load()
	if rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute+"/x"); rec.Code != 404 {
		t.Errorf("sub path: %d", rec.Code)
	}
	rec := candidatesGet(t, h, http.MethodPost, "/"+siteCandidatesRoute)
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" || f.builds.Load() != before {
		t.Errorf("POST: %d allow %q builds %d", rec.Code, rec.Header().Get("Allow"), f.builds.Load())
	}
}

func TestCandidatesBuildErrorIs500(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	f.err = errors.New("dsn sentinel-secret")
	h := siteHandler(f.a, candidatesClock, "2026-11-05", siteReaders{cands: demoCandidates{}}, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != "internal error" {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestCandidatesUsesPagesDSN(t *testing.T) {
	t.Parallel()
	const env = store.DSNEnvPages + "=pages-dsn\n" + store.DSNEnvCandidate + "=candidate-dsn\n"
	f := newCandidatesFixture(t, env)
	f.a.profile.Name = "real"
	var dsns []string
	var closed atomic.Int32
	cc := closeCounter{&closed}
	opened := func(d string) { dsns = append(dsns, d) }
	f.a.deps.openSeries = func(_ context.Context, d string) (store.SeriesStore, error) {
		opened(d)
		return cSeries{closeCounter: cc}, nil
	}
	f.a.deps.openOptionEod = func(_ context.Context, d string) (store.OptionEodStore, error) {
		opened(d)
		return cEod{closeCounter: cc}, nil
	}
	f.a.deps.openQuoteSnapshot = func(_ context.Context, d string) (store.QuoteSnapshotStore, error) {
		opened(d)
		return cQuotes{closeCounter: cc}, nil
	}
	f.a.deps.openOptionCatalog = func(_ context.Context, d string) (store.OptionEodCatalogStore, error) {
		opened(d)
		return cCatalog{closeCounter: cc}, nil
	}
	f.a.deps.openOptionTerms = func(_ context.Context, d string) (store.OptionTermsStore, error) {
		opened(d)
		return cTerms{closeCounter: cc}, nil
	}
	f.a.deps.openInstrumentName = func(_ context.Context, d string) (store.InstrumentNameStore, error) {
		opened(d)
		return cNames{closeCounter: cc}, nil
	}
	f.a.deps.openReadingView = func(_ context.Context, d string) (store.ReadingViewReader, error) {
		opened(d)
		return cView{closeCounter: cc}, nil
	}
	candOpens := 0
	want := cCands{closeCounter: cc}
	f.a.deps.openCandidate = func(_ context.Context, d string) (candidateStore, error) {
		candOpens++
		opened(d)
		return want, nil
	}
	rd, closeFn, err := openSiteReaders(t.Context(), f.a)
	if err != nil {
		t.Fatal(err)
	}
	h := siteHandler(f.a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	for range 3 {
		if rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute); rec.Code != 200 {
			t.Fatalf("code %d", rec.Code)
		}
	}
	if candOpens != 1 || f.builds.Load() != 3 || f.got != want {
		t.Fatalf("opens %d builds %d reader %v", candOpens, f.builds.Load(), f.got)
	}
	for _, d := range dsns {
		if d != "pages-dsn" {
			t.Fatalf("dsn %q", d)
		}
	}
	closeFn()
	if closed.Load() != 8 {
		t.Fatalf("closed %d", closed.Load())
	}
}

func TestCandidatesDemoNeedsNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.optionCandidatesPage = newOptionCandidatesPage
	a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		t.Error("candidate store opened")
		return nil, errors.New("opened")
	}
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("series store opened")
		return nil, errors.New("opened")
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"schema":"option_candidates_page"`) {
		t.Fatalf("%d %q", rec.Code, body)
	}
	for _, k := range []string{"short_put", "covered_call", "held_call_guard"} {
		if !strings.Contains(body, `"kind":"`+k+`"`) {
			t.Errorf("no group %s", k)
		}
	}
	if !strings.Contains(body, "earnings_before_expiry") || !strings.Contains(body, `"ok":2`) || !strings.Contains(body, `"blocked":1`) {
		t.Errorf("demo rows missing ok or blocked: %s", body)
	}
}

func TestPageCommandBuildsCandidates(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.optionCandidatesPage = newOptionCandidatesPage
	var out strings.Builder
	a.stdout = &out
	if err := runPage(a, []string{"--name", candidatesName, "--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"schema":"option_candidates_page"`) || !strings.Contains(out.String(), `"run_id":"ot-page"`) {
		t.Fatalf("page %s", out.String())
	}
	// The command and the site build the same page for the same day (run id apart).
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, a.deps.newClock(), "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	site := strings.Replace(rec.Body.String(), `"run_id":"ot-serve"`, `"run_id":"ot-page"`, 1)
	assertSamePageProps(t, site, out.String())
}

// assertSamePageProps fails when two pages differ in a top-level property other than short_put_list, which
// only the site's Shortlist reader fills.
func assertSamePageProps(t *testing.T, site, cmd string) {
	t.Helper()
	decode := func(name, page string) map[string]json.RawMessage {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(page), &m); err != nil {
			t.Fatalf("%s page is not a JSON object: %v: %s", name, err, page)
		}
		delete(m, "short_put_list")
		return m
	}
	sm, cm := decode("site", site), decode("command", cmd)
	keys := make([]string, 0, len(sm)+len(cm))
	for k := range sm {
		keys = append(keys, k)
	}
	for k := range cm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sv, sok := sm[k]
		cv, cok := cm[k]
		if sok != cok || string(sv) != string(cv) {
			t.Fatalf("property %q differs\nsite %s (present %v)\ncommand %s (present %v)", k, sv, sok, cv, cok)
		}
	}
}

func TestCandidatesNamesReaderPassed(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	names := zzzNames{}
	h := siteHandler(f.a, candidatesClock, "2026-11-05", siteReaders{cands: demoCandidates{}, names: names}, echoHandler, http.NotFoundHandler())
	if rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute); rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	if f.gotNames != names {
		t.Fatalf("names reader %v", f.gotNames)
	}
	a, _ := testApp(t, "")
	a.deps.newRuleParams = ruleparam.New
	a.deps.optionCandidatesPage = newOptionCandidatesPage
	b, err := buildCandidates(t.Context(), a, candidatesClock, "2026-11-05", siteReaders{cands: demoCandidates{}, names: names})
	if err != nil || !strings.Contains(string(b), "ZZZ") {
		t.Fatalf("%v: ticker missing: %s", err, b)
	}
}

// rateSeries records the series ids it is asked for.
type rateSeries struct {
	mu  sync.Mutex
	ids []string
}

func (r *rateSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, id)
	return nil, nil
}

func TestBuildCandidatesReadsRateSeries(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.deps.optionCandidatesPage = newOptionCandidatesPage
	a.deps.newRuleParams = ruleparam.New
	rs := &rateSeries{}
	if _, err := buildCandidates(t.Context(), a, candidatesClock, "2026-11-05", siteReaders{cands: demoCandidates{}, series: rs}); err != nil {
		t.Fatal(err)
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if len(rs.ids) == 0 {
		t.Fatal("series reader never asked")
	}
	for _, id := range rs.ids {
		if id != "DGS3MO" {
			t.Fatalf("series asked for %v, want only DGS3MO", rs.ids)
		}
	}
}

// emptyCandidates is an old reader with no row.
type emptyCandidates struct{}

func (emptyCandidates) Candidates(context.Context, contracts.Day, string) ([]store.Candidate, error) {
	return nil, nil
}

// shortPutOnly answers its rows for kind short_put and nothing for the other kinds.
type shortPutOnly struct{ rows []store.Candidate }

func (s shortPutOnly) Candidates(_ context.Context, _ contracts.Day, kind string) ([]store.Candidate, error) {
	if kind != "short_put" {
		return nil, nil
	}
	return s.rows, nil
}

// namesNone is a name reader that knows no instrument.
type namesNone struct{}

func (namesNone) InstrumentNames(context.Context, []int64) ([]store.InstrumentName, error) {
	return nil, nil
}

// seriesNone is a series reader with no point.
type seriesNone struct{}

func (seriesNone) SeriesPoints(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SeriesPoint, error) {
	return nil, nil
}

func TestCandidatesPageSameRowsFromEitherReader(t *testing.T) {
	t.Parallel()
	res := &fakeResultStore{rows: []strategyresult.Row{shortPutResultRow("2026-11-05", "r1")}}
	fromResult := strategyresultpg.NewCandidateReader(res, emptyCandidates{})
	rows, err := fromResult.Candidates(t.Context(), "2026-11-05", "short_put")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows %v, %v", rows, err)
	}
	fromOld := shortPutOnly{rows: rows}
	render := func(c store.CandidateReader) string {
		p := newOptionCandidatesPage(time.UTC, "ot-page", "b", "option", candidateReaders{Cands: c, Names: namesNone{}})
		b, err := p.Build(t.Context(), seriesNone{}, "2026-11-05", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	a, b := render(fromResult), render(fromOld)
	if a != b || !strings.Contains(a, `"kind":"short_put"`) {
		t.Fatalf("pages differ:\n%s\n%s", a, b)
	}
}

func TestCandidatesWiresEvidence(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	rd := siteReaders{cands: demoCandidates{}, names: demoOption{}, view: demoView{}, terms: demoOption{}, days: &recordDays{}}
	h := siteHandler(f.a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_candidates_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	g := f.gotAll
	if g.View == nil || g.Terms == nil || g.Days == nil || g.View != store.ReadingViewReader(rd.view) || g.Days != rd.days {
		t.Fatalf("readers %+v", g)
	}
}

func TestCandidatesNamedNoEvidence(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, store.DSNEnvPages+"=pages-dsn\n")
	f.a.profile.Name = "real"
	var closed atomic.Int32
	cc := closeCounter{&closed}
	f.a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		return cCandsReader{demoCandidates: demoCandidates{}, closeCounter: cc}, nil
	}
	f.a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
		return cNames{closeCounter: cc}, nil
	}
	f.a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("reading view opened")
		return nil, errors.New("opened")
	}
	f.a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) {
		t.Error("terms store opened")
		return nil, errors.New("opened")
	}
	f.a.deps.openOptionDay = func(context.Context, string) (optionday.DayReader, error) {
		t.Error("day reader opened")
		return nil, errors.New("opened")
	}
	p, closeFn, err := candidatesPageNamed(t.Context(), f.a)
	if err != nil || p.Meta.Name != candidatesName {
		t.Fatalf("%v %+v", err, p.Meta)
	}
	closeFn()
	g := f.gotAll
	if g.View != nil || g.Terms != nil || g.Days != nil || g.Cands == nil || g.Names == nil || closed.Load() != 2 {
		t.Fatalf("readers %+v closed %d", g, closed.Load())
	}
}

// nameOpenReader is a stored open-row reader that holds no row.
type nameOpenReader struct{}

func (nameOpenReader) OptionOpen(context.Context, contracts.Day) ([]contracts.OptionOpen, error) {
	return nil, nil
}

func TestCandidatesSiteNameOutcomes(t *testing.T) {
	t.Parallel()
	const watch = `
[[underlying]]
issuer = "edinet:E00002"
market = "JP"
symbol = "2413"
sec_type = "STK"
exchange = "TSEJ"
currency = "JPY"
con_id = 910001

[[underlying]]
issuer = "edinet:E00001"
market = "JP"
symbol = "2408"
sec_type = "STK"
exchange = "TSEJ"
currency = "JPY"
con_id = 910002

[[underlying]]
issuer = "cik:0000320193"
market = "US"
symbol = "AAPL"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 265598
`
	f := newCandidatesFixture(t, "")
	f.a.profile.Name = "real"
	f.a.deps.readFile = func(string) ([]byte, error) { return []byte(watch), nil }
	reader := nameOpenReader{}
	build := func(rd siteReaders) candidateReaders {
		h := siteHandler(f.a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
		if rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute); rec.Code != 200 {
			t.Fatalf("code %d", rec.Code)
		}
		return f.gotAll
	}
	g := build(siteReaders{cands: demoCandidates{}, open: reader})
	if g.OpenRows != openquote.OptionOpenReader(reader) || g.NameOpen == nil || len(g.JPNames) != 2 || g.JPNames[0] != "2408" || g.JPNames[1] != "2413" {
		t.Fatalf("with an open-row reader: %+v", g)
	}
	if g.NameOpen().Classify(optioncandapi.NameOpenFacts{}).Outcome != optioncandapi.OutcomeNoRows {
		t.Error("the rule is not the name-open rule")
	}
	g = build(siteReaders{cands: demoCandidates{}})
	if g.OpenRows != nil || g.NameOpen != nil || g.JPNames != nil {
		t.Fatalf("without an open-row reader: %+v", g)
	}
}
