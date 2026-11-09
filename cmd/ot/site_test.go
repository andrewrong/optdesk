package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeCatalog answers entries or err and records the since it was asked with.
type fakeCatalog struct {
	entries []store.OptionEodLatest
	err     error
	since   contracts.Day
}

func (f *fakeCatalog) OptionEodLatest(_ context.Context, since contracts.Day) ([]store.OptionEodLatest, error) {
	f.since = since
	return f.entries, f.err
}

// siteFixture is a site handler over a fake catalog and a fake page that records its input.
type siteFixture struct {
	cat       *fakeCatalog
	inputs    []sitePageInput
	buildErr  error
	macroErr  error
	macroDays []contracts.Day
	h         http.Handler
}

func newSiteFixture(t *testing.T, entries ...store.OptionEodLatest) *siteFixture {
	t.Helper()
	a, _ := testApp(t, "")
	f := &siteFixture{cat: &fakeCatalog{entries: entries}}
	a.deps.pageList = []pages.Page{{
		Meta: pages.Meta{Name: "macro_rates"},
		Build: func(_ context.Context, _ store.SeriesReader, d contracts.Day, _ time.Time) ([]byte, error) {
			f.macroDays = append(f.macroDays, d)
			if f.macroErr != nil {
				return nil, f.macroErr
			}
			return []byte("MACRO"), nil
		},
	}}
	a.deps.optionDataPage = func(_ *time.Location, _ string, in sitePageInput) pages.Page {
		f.inputs = append(f.inputs, in)
		p := fakePage("option_data_page", "PAGE")
		if f.buildErr != nil {
			p.Build = func(context.Context, store.SeriesReader, contracts.Day, time.Time) ([]byte, error) {
				return nil, f.buildErr
			}
		}
		return p
	}
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	f.h = siteHandler(a, clk, "2026-11-06", siteReaders{catalog: f.cat, names: &fakeNames{}}, func(vm []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vm) })
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ASSET")) }))
	return f
}

func (f *siteFixture) do(method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, path, nil))
	return rec
}

func TestSiteRoutes(t *testing.T) {
	t.Parallel()
	entries := []store.OptionEodLatest{
		{UnderlyingID: 9000, SessionDay: "2026-11-04"},
		{UnderlyingID: 9020, SessionDay: "2026-11-05"},
		{UnderlyingID: 9010, SessionDay: "2026-11-05"},
	}
	t.Run("by id", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t, entries...)
		if rec := f.do(http.MethodGet, "/options/data/9010"); rec.Code != 200 || rec.Body.String() != "PAGE" {
			t.Fatalf("got %d %q", rec.Code, rec.Body)
		}
		if len(f.inputs) != 1 || f.inputs[0].UnderlyingID != 9010 {
			t.Fatalf("inputs %+v", f.inputs)
		}
	})
	t.Run("greatest day, tie to lowest id", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t, entries...)
		f.do(http.MethodGet, "/options/data")
		if len(f.inputs) != 1 || f.inputs[0].UnderlyingID != 9010 {
			t.Fatalf("inputs %+v", f.inputs)
		}
	})
	t.Run("empty catalog is id 0", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t)
		if rec := f.do(http.MethodGet, "/options/data"); rec.Code != 200 {
			t.Fatalf("code %d", rec.Code)
		}
		if len(f.inputs) != 1 || f.inputs[0].UnderlyingID != 0 {
			t.Fatalf("inputs %+v", f.inputs)
		}
	})
	t.Run("root redirects", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t, entries...)
		rec := f.do(http.MethodGet, "/")
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/home" {
			t.Fatalf("got %d %q", rec.Code, rec.Header().Get("Location"))
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		checkNotFound(t, newSiteFixture(t, entries...))
	})
	t.Run("macro and assets", func(t *testing.T) {
		t.Parallel()
		checkMacroAssets(t, newSiteFixture(t, entries...))
	})
	t.Run("method", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t, entries...)
		for _, p := range []string{"/options/data", "/macro", "/home", "/assets/a.js", "/"} {
			if rec := f.do(http.MethodPost, p); rec.Code != 405 {
				t.Fatalf("POST %s code %d", p, rec.Code)
			}
		}
		if rec := f.do(http.MethodHead, "/options/data"); rec.Code != 200 {
			t.Fatalf("HEAD code %d", rec.Code)
		}
	})
	t.Run("errors are 500 without text", func(t *testing.T) {
		t.Parallel()
		checkSiteErrors(t, newSiteFixture(t, entries...))
	})
}

func checkNotFound(t *testing.T, f *siteFixture) {
	t.Helper()
	paths := []string{
		"/options/data/0", "/options/data/-1", "/options/data/x", "/options/data/1/2", "/nope",
		"/options/data/+5", "/options/data/05", "/home/x", "/macro/x", "/assets", "/assets/",
	}
	for _, p := range paths {
		if rec := f.do(http.MethodGet, p); rec.Code != 404 {
			t.Errorf("%s: code %d", p, rec.Code)
		}
	}
	if len(f.inputs) != 0 || len(f.macroDays) != 0 {
		t.Fatalf("a page was built: %+v %v", f.inputs, f.macroDays)
	}
}

func checkMacroAssets(t *testing.T, f *siteFixture) {
	t.Helper()
	if rec := f.do(http.MethodGet, "/macro"); rec.Code != 200 || rec.Body.String() != "MACRO" {
		t.Fatalf("macro %d %q", rec.Code, rec.Body)
	}
	if len(f.macroDays) != 1 || f.macroDays[0] != "2026-11-06" || len(f.inputs) != 0 {
		t.Fatalf("macro days %v inputs %v", f.macroDays, f.inputs)
	}
	if rec := f.do(http.MethodGet, "/assets/a.js"); rec.Code != 200 || rec.Body.String() != "ASSET" {
		t.Fatalf("asset %d %q", rec.Code, rec.Body)
	}
	f.macroErr = errors.New("dsn sentinel-secret")
	if rec := f.do(http.MethodGet, "/macro"); rec.Code != 500 || strings.Contains(rec.Body.String(), "sentinel") {
		t.Fatalf("macro error %d %q", rec.Code, rec.Body)
	}
}

func checkSiteErrors(t *testing.T, f *siteFixture) {
	t.Helper()
	f.cat.err = errors.New("dsn sentinel-secret")
	rec := f.do(http.MethodGet, "/options/data")
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "sentinel") {
		t.Fatalf("catalog: %d %q", rec.Code, rec.Body)
	}
	f.cat.err = nil
	f.buildErr = errors.New("dsn sentinel-secret")
	rec = f.do(http.MethodGet, "/options/data/9000")
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "sentinel") {
		t.Fatalf("build: %d %q", rec.Code, rec.Body)
	}
}

func TestSiteChoices(t *testing.T) {
	t.Parallel()
	f := newSiteFixture(t,
		store.OptionEodLatest{UnderlyingID: 9000, SessionDay: "2026-11-04"},
		store.OptionEodLatest{UnderlyingID: 9010, SessionDay: "2026-11-05"})
	f.do(http.MethodGet, "/options/data/9000")
	if f.cat.since != "2026-10-30" {
		t.Fatalf("since %q", f.cat.since)
	}
	want := []siteChoice{
		{9000, "Underlying 9000", "/options/data/9000"},
		{9010, "Underlying 9010", "/options/data/9010"},
	}
	got := f.inputs[0].Choices
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("choices %+v", got)
	}
}

func TestSiteDemoOpensNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	a.deps.openOptionCatalog = func(context.Context, string) (store.OptionEodCatalogStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	golden, err := os.ReadFile("../../contracts/fixtures/page.macro_rates.v3.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	a.deps.pageList = []pages.Page{{
		Meta: pages.Meta{Name: "macro_rates"},
		Build: func(context.Context, store.SeriesReader, contracts.Day, time.Time) ([]byte, error) {
			return golden, nil
		},
	}}
	a.deps.optionDataPage = newOptionDataPage
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, a.deps.newClock(), "", rd, echoHandler, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/macro", nil))
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), golden) {
		t.Fatalf("macro: %d %.200q", rec.Code, rec.Body)
	}
	for _, p := range []string{"/options/data", "/options/data/9010"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, p, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_data_page"`) {
			t.Fatalf("%s: %d %.200q", p, rec.Code, rec.Body)
		}
	}
	got, err := rd.names.InstrumentNames(t.Context(), []int64{9000, 9010, 5})
	if err != nil || len(got) != 2 || got[0].InstrumentID != 9000 || got[1].InstrumentID != 9010 || got[0].Name == "" {
		t.Fatalf("demo names %+v %v", got, err)
	}
}

// echoHandler writes the view model as the body, so a test needs no built web bundle.
func echoHandler(vm []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vm) })
}

func TestSiteMissingDSN(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_RUNNER=postgres://sentinel-secret\n")
	a.profile.Name = "real"
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	err := runServe(a, []string{"--addr", "127.0.0.1:0"})
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvPages) || strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("err %v", err)
	}
}

// fakeNames answers names, records the ids of each read and fails with err.
type fakeNames struct {
	mu    sync.Mutex
	names []store.InstrumentName
	err   error
	reads [][]int64
}

func (f *fakeNames) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, append([]int64(nil), ids...))
	return f.names, f.err
}

// noRows holds no option data at all.
type noRows struct{}

func (noRows) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	return nil, nil
}

func (noRows) QuoteSnapshots(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.QuoteSnapshot, error) {
	return nil, nil
}

func (noRows) OptionTerms(context.Context, []int64) ([]store.OptionTerms, error) { return nil, nil }

// realSite serves the real option_data_page over the catalog, the names and empty rows; it returns the handler
// and the log of the app.
func realSite(t *testing.T, cat *fakeCatalog, names *fakeNames) (http.Handler, *bytes.Buffer) {
	t.Helper()
	a, log := testApp(t, "")
	a.deps.optionDataPage = newOptionDataPage
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	rd := siteReaders{eod: noRows{}, quotes: noRows{}, catalog: cat, terms: noRows{}, names: names}
	return siteHandler(a, clk, "2026-11-06", rd, func(vm []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vm) })
	}, http.NotFoundHandler()), log
}

type sitePageJSON struct {
	RunStatus  string   `json:"run_status"`
	Notes      []string `json:"notes"`
	Gaps       []any    `json:"gaps"`
	Underlying struct {
		Symbol *string `json:"symbol"`
		Label  string  `json:"label"`
	} `json:"underlying"`
	Choices []struct {
		Label string `json:"label"`
	} `json:"underlyings"`
}

func getSite(t *testing.T, h http.Handler, path string) (int, sitePageJSON) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	var p sitePageJSON
	if rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("json: %v: %.300q", err, rec.Body)
		}
	}
	return rec.Code, p
}

const emptyNote = "No option data is stored yet; collection has not run."

func TestSiteNames(t *testing.T) {
	t.Parallel()
	cat := &fakeCatalog{entries: []store.OptionEodLatest{
		{UnderlyingID: 9000, SessionDay: "2026-11-05"}, {UnderlyingID: 9010, SessionDay: "2026-11-04"},
	}}
	names := &fakeNames{names: []store.InstrumentName{
		{InstrumentID: 9000, Type: "stock", Name: "AAA"}, {InstrumentID: 9010, Type: "stock", Name: "BBB"},
	}}
	h, _ := realSite(t, cat, names)
	code, p := getSite(t, h, "/options/data")
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	if p.Underlying.Label != "AAA" || p.Underlying.Symbol == nil || *p.Underlying.Symbol != "AAA" {
		t.Fatalf("underlying %+v", p.Underlying)
	}
	if len(p.Choices) != 2 || p.Choices[0].Label != "AAA" || p.Choices[1].Label != "BBB" {
		t.Fatalf("choices %+v", p.Choices)
	}
	if len(names.reads) != 1 {
		t.Fatalf("%d name reads", len(names.reads))
	}
	if len(p.Notes) != 0 {
		t.Fatalf("notes %v", p.Notes)
	}

	t.Run("no name or option type keeps the id", func(t *testing.T) {
		t.Parallel()
		names := &fakeNames{names: []store.InstrumentName{{InstrumentID: 9010, Type: "option", Name: "OPT"}}}
		h, _ := realSite(t, cat, names)
		_, p := getSite(t, h, "/options/data")
		if p.Underlying.Label != "Underlying 9000" || p.Underlying.Symbol != nil {
			t.Fatalf("underlying %+v", p.Underlying)
		}
		if p.Choices[0].Label != "Underlying 9000" || p.Choices[1].Label != "Underlying 9010" {
			t.Fatalf("choices %+v", p.Choices)
		}
	})
	t.Run("at most 50 ids", func(t *testing.T) {
		t.Parallel()
		var entries []store.OptionEodLatest
		for i := int64(1); i <= 80; i++ {
			entries = append(entries, store.OptionEodLatest{UnderlyingID: 100 + i, SessionDay: "2026-11-05"})
		}
		names := &fakeNames{}
		h, _ := realSite(t, &fakeCatalog{entries: entries}, names)
		if code, _ := getSite(t, h, "/options/data/999"); code != 200 {
			t.Fatalf("code %d", code)
		}
		if len(names.reads) != 1 || len(names.reads[0]) != 50 {
			t.Fatalf("reads %v", names.reads)
		}
	})
}

func TestSiteEmptyStore(t *testing.T) {
	t.Parallel()
	names := &fakeNames{}
	h, _ := realSite(t, &fakeCatalog{}, names)
	code, p := getSite(t, h, "/options/data")
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	if p.RunStatus != "partial" || len(p.Gaps) != 1 || len(p.Notes) != 1 || p.Notes[0] != emptyNote ||
		p.Underlying.Label != "No underlying yet" {
		t.Fatalf("page %+v", p)
	}
	if len(names.reads) != 0 {
		t.Fatalf("name reads %v", names.reads)
	}
}

func TestSiteNoteOnlyWhenCatalogEmpty(t *testing.T) {
	t.Parallel()
	cat := &fakeCatalog{entries: []store.OptionEodLatest{{UnderlyingID: 9000, SessionDay: "2026-11-05"}}}
	h, _ := realSite(t, cat, &fakeNames{})
	for _, path := range []string{"/options/data", "/options/data/9000", "/options/data/777"} {
		code, p := getSite(t, h, path)
		if code != 200 || len(p.Notes) != 0 || len(p.Gaps) != 1 {
			t.Fatalf("%s: %d %+v", path, code, p)
		}
		if b, _ := json.Marshal(p.Gaps); !strings.Contains(string(b), "no option data stored") {
			t.Fatalf("%s: gaps %s", path, b)
		}
	}
}

func TestSiteReaderErrors(t *testing.T) {
	t.Parallel()
	cat := &fakeCatalog{entries: []store.OptionEodLatest{{UnderlyingID: 9000, SessionDay: "2026-11-05"}}}
	for name, tc := range map[string]struct {
		cat   *fakeCatalog
		names *fakeNames
	}{
		"catalog": {&fakeCatalog{err: errors.New("dsn sentinel-secret")}, &fakeNames{}},
		"names":   {cat, &fakeNames{err: errors.New("dsn sentinel-secret")}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, log := realSite(t, tc.cat, tc.names)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/options/data", nil))
			if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != "internal error" {
				t.Fatalf("%d %q", rec.Code, rec.Body)
			}
			if !strings.Contains(log.String(), "component=ot") || !strings.Contains(log.String(), "status=failed") ||
				strings.Contains(log.String(), "sentinel") {
				t.Fatalf("log %q", log)
			}
		})
	}
}

func TestDemoReaderSeven(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"DGS10": "percent", "DGS3MO": "percent", "SP500": "index_points", "VIXCLS": "index_points",
		"VXNCLS": "index_points", "RVXCLS": "index_points", "VXVCLS": "index_points",
	}
	for id, unit := range want {
		pts, err := demoReader{}.SeriesPoints(t.Context(), id, "2026-11-01", "2026-11-30")
		if err != nil || len(pts) != 3 {
			t.Fatalf("%s: %d points, err %v", id, len(pts), err)
		}
		for _, p := range pts {
			if p.SeriesID != id || string(p.Unit) != unit {
				t.Errorf("%s: point %+v", id, p)
			}
		}
	}
	if pts, _ := (demoReader{}).SeriesPoints(t.Context(), "NOPE", "2026-11-01", "2026-11-30"); len(pts) != 0 {
		t.Fatalf("unknown series: %v", pts)
	}
}

// closeCounter counts Close calls.
type closeCounter struct{ n *atomic.Int32 }

func (c closeCounter) Close() error { c.n.Add(1); return nil }

type cSeries struct {
	store.SeriesStore
	closeCounter
}

func (c cSeries) Close() error { return c.closeCounter.Close() }

type cView struct {
	store.ReadingViewReader
	closeCounter
}

func (c cView) Close() error { return c.closeCounter.Close() }

type cEod struct {
	store.OptionEodStore
	closeCounter
}

func (c cEod) Close() error { return c.closeCounter.Close() }

type cQuotes struct {
	store.QuoteSnapshotStore
	closeCounter
}

func (c cQuotes) Close() error { return c.closeCounter.Close() }

type cCatalog struct {
	store.OptionEodCatalogStore
	closeCounter
}

func (c cCatalog) Close() error { return c.closeCounter.Close() }

type cTerms struct {
	store.OptionTermsStore
	closeCounter
}

func (c cTerms) Close() error { return c.closeCounter.Close() }

type cNames struct {
	store.InstrumentNameStore
	closeCounter
}

func (c cNames) Close() error { return c.closeCounter.Close() }

type cCands struct {
	store.CandidateReader
	closeCounter
}

func (c cCands) Close() error { return c.closeCounter.Close() }

func TestSiteStoresClosed(t *testing.T) {
	t.Parallel()
	errOpen := errors.New("open sentinel-secret")
	for failAt := range 8 {
		t.Run(strconv.Itoa(failAt), func(t *testing.T) {
			t.Parallel()
			a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
			a.profile.Name = "real"
			var closed atomic.Int32
			opened := 0
			step := func() error {
				if opened == failAt {
					return errOpen
				}
				opened++
				return nil
			}
			cc := closeCounter{&closed}
			a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
				return cSeries{closeCounter: cc}, step()
			}
			a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
				return cEod{closeCounter: cc}, step()
			}
			a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) {
				return cQuotes{closeCounter: cc}, step()
			}
			a.deps.openOptionCatalog = func(context.Context, string) (store.OptionEodCatalogStore, error) {
				return cCatalog{closeCounter: cc}, step()
			}
			a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) {
				return cTerms{closeCounter: cc}, step()
			}
			a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
				return cNames{closeCounter: cc}, step()
			}
			a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
				return cView{closeCounter: cc}, step()
			}
			a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
				return cCands{closeCounter: cc}, step()
			}
			_, _, err := openSiteReaders(t.Context(), a)
			if err == nil || strings.Contains(err.Error(), "sentinel") {
				t.Fatalf("err %v", err)
			}
			if int(closed.Load()) != failAt {
				t.Fatalf("closed %d want %d", closed.Load(), failAt)
			}
		})
	}
	t.Run("all open, closeFn closes all", func(t *testing.T) {
		t.Parallel()
		a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
		a.profile.Name = "real"
		var closed atomic.Int32
		cc := closeCounter{&closed}
		a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { return cSeries{closeCounter: cc}, nil }
		a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) { return cEod{closeCounter: cc}, nil }
		a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) { return cQuotes{closeCounter: cc}, nil }
		a.deps.openOptionCatalog = func(context.Context, string) (store.OptionEodCatalogStore, error) {
			return cCatalog{closeCounter: cc}, nil
		}
		a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) { return cTerms{closeCounter: cc}, nil }
		a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) { return cNames{closeCounter: cc}, nil }
		a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
			return cView{closeCounter: cc}, nil
		}
		a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
			return cCands{closeCounter: cc}, nil
		}
		_, closeFn, err := openSiteReaders(t.Context(), a)
		if err != nil {
			t.Fatal(err)
		}
		closeFn()
		if closed.Load() != 8 {
			t.Fatalf("closed %d", closed.Load())
		}
	})
}

// reportSiteApp is an app whose report page and risks block are the real ones over rule.
func reportSiteApp(t *testing.T, view store.ReadingViewReader) http.Handler {
	t.Helper()
	a := reportApp(t, "")
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	return siteHandler(a, clk, "2026-11-06", siteReaders{view: view}, echoHandler, http.NotFoundHandler())
}

func TestSiteReportRoute(t *testing.T) {
	t.Parallel()
	get := func(h http.Handler, method, target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
		return rec
	}
	risksOf := func(rec *httptest.ResponseRecorder) string {
		var p struct {
			Blocks struct {
				Risks struct {
					State string `json:"state"`
					Items any    `json:"items"`
				} `json:"risks"`
			} `json:"blocks"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("%v: %q", err, rec.Body)
		}
		items, _ := p.Blocks.Risks.Items.([]any)
		return p.Blocks.Risks.State + "/" + strconv.Itoa(len(items))
	}
	t.Run("built per request", func(t *testing.T) {
		t.Parallel()
		v := &fakeView{}
		h := reportSiteApp(t, v)
		v.set(viewRow("2026-11-06", "0.9", "2026-11-06"))
		rec := get(h, http.MethodGet, "/report?market=us&slot=open")
		if rec.Code != 200 || risksOf(rec) != "ok/1" {
			t.Fatalf("first: %d %q", rec.Code, rec.Body)
		}
		v.set(viewRow("2026-11-06", "0.9", ""))
		rec = get(h, http.MethodGet, "/report?market=us&slot=open")
		if rec.Code != 200 || risksOf(rec) != "gap/0" {
			t.Fatalf("second: %d %q", rec.Code, rec.Body)
		}
	})
	t.Run("bad query is 400", func(t *testing.T) {
		t.Parallel()
		h := reportSiteApp(t, &fakeView{})
		for _, target := range []string{"/report", "/report?market=us", "/report?market=xx&slot=open", "/report?market=us&slot=noon"} {
			rec := get(h, http.MethodGet, target)
			if rec.Code != 400 || strings.TrimSpace(rec.Body.String()) != "bad request" {
				t.Errorf("%s: %d %q", target, rec.Code, rec.Body)
			}
		}
	})
	t.Run("reader error is 500", func(t *testing.T) {
		t.Parallel()
		h := reportSiteApp(t, &fakeView{err: errors.New("dsn sentinel-secret")})
		rec := get(h, http.MethodGet, "/report?market=us&slot=open")
		if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != "internal error" {
			t.Fatalf("%d %q", rec.Code, rec.Body)
		}
	})
	t.Run("path and method", func(t *testing.T) {
		t.Parallel()
		h := reportSiteApp(t, &fakeView{})
		if rec := get(h, http.MethodGet, "/report/x?market=us&slot=open"); rec.Code != 404 {
			t.Errorf("report/x: %d", rec.Code)
		}
		if rec := get(h, http.MethodPost, "/report?market=us&slot=open"); rec.Code != 405 {
			t.Errorf("POST: %d", rec.Code)
		}
	})
	t.Run("other routes as before", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t)
		checkMacroAssets(t, f)
		if rec := f.do(http.MethodGet, "/options/data"); rec.Code != 200 {
			t.Fatalf("options/data: %d", rec.Code)
		}
	})
}

// healthSeries answers SeriesPoints with err and records the series id and the days it was asked with.
type healthSeries struct {
	store.SeriesReader
	err      error
	id       string
	from, to contracts.Day
	deadline bool
}

func (h *healthSeries) SeriesPoints(ctx context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	h.id, h.from, h.to = id, from, to
	_, h.deadline = ctx.Deadline()
	return nil, h.err
}

func healthSite(t *testing.T, s store.SeriesReader) (http.Handler, *bytes.Buffer) {
	t.Helper()
	a, logb := testApp(t, "")
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	return siteHandler(a, clk, "2026-01-01", siteReaders{series: s}, echoHandler, http.NotFoundHandler()), logb
}

func TestSiteHealthOK(t *testing.T) {
	t.Parallel()
	s := &healthSeries{}
	h, _ := healthSite(t, s)
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), m, "/health", nil))
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%s: %d %q", m, rec.Code, rec.Header().Get("Content-Type"))
		}
		if m == http.MethodGet && strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
			t.Fatalf("body %q", rec.Body.String())
		}
	}
	if s.id != "DGS10" || s.from != "2026-11-06" || s.to != "2026-11-06" || !s.deadline {
		t.Fatalf("read %+v", s)
	}
	// the demo reader never fails
	dh, _ := healthSite(t, demoReader{})
	rec := httptest.NewRecorder()
	dh.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))
	if rec.Code != 200 {
		t.Fatalf("demo %d", rec.Code)
	}
}

func TestSiteHealthUnavailable(t *testing.T) {
	t.Parallel()
	h, logb := healthSite(t, &healthSeries{err: errors.New("secret pgx detail")})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))
	if rec.Code != 503 || strings.TrimSpace(rec.Body.String()) != `{"status":"unavailable"}` {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
	out := logb.String()
	if !strings.Contains(out, "serve health failed") || !strings.Contains(out, "component=ot") || !strings.Contains(out, "status=failed") || strings.Contains(out, "secret") {
		t.Fatalf("log %q", out)
	}
}

func TestSiteHealthMethodNotAllowed(t *testing.T) {
	t.Parallel()
	h, _ := healthSite(t, &healthSeries{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/health", nil))
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Allow"))
	}
}
