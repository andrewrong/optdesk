package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// candidatesFixture is an app whose optionCandidatesPage fake builds {"schema":"option_candidates_page"} and
// records the reader and the days it was given.
type candidatesFixture struct {
	a        *app
	builds   atomic.Int32
	days     []contracts.Day
	got      store.CandidateReader
	gotNames store.InstrumentNameReader
	err      error
}

func newCandidatesFixture(t *testing.T, env string) *candidatesFixture {
	t.Helper()
	f := &candidatesFixture{}
	f.a, _ = testApp(t, env)
	f.a.deps.optionCandidatesPage = func(_ *time.Location, _, _, _ string, c store.CandidateReader, n store.InstrumentNameReader) pages.Page {
		f.got = c
		f.gotNames = n
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
	if site != out.String() {
		t.Fatalf("site %s\ncommand %s", site, out.String())
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
