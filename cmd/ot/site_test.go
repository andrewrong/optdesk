package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	cat      *fakeCatalog
	inputs   []sitePageInput
	buildErr error
	h        http.Handler
}

func newSiteFixture(t *testing.T, entries ...store.OptionEodLatest) *siteFixture {
	t.Helper()
	a, _ := testApp(t, "")
	f := &siteFixture{cat: &fakeCatalog{entries: entries}}
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
	})
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
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/options/data" {
			t.Fatalf("got %d %q", rec.Code, rec.Header().Get("Location"))
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t, entries...)
		for _, p := range []string{"/options/data/0", "/options/data/-1", "/options/data/x", "/options/data/1/2", "/nope", "/options/data/+5", "/options/data/05"} {
			if rec := f.do(http.MethodGet, p); rec.Code != 404 {
				t.Errorf("%s: code %d", p, rec.Code)
			}
		}
		if len(f.inputs) != 0 {
			t.Fatalf("a page was built: %+v", f.inputs)
		}
	})
	t.Run("method", func(t *testing.T) {
		t.Parallel()
		f := newSiteFixture(t, entries...)
		if rec := f.do(http.MethodPost, "/options/data"); rec.Code != 405 {
			t.Fatalf("POST code %d", rec.Code)
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

func TestServeSiteFlags(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	err := runServe(a, []string{"--site", "--name", "macro_page"})
	if err == nil || !strings.Contains(err.Error(), "--site") || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("err %v", err)
	}
	err = runServe(a, []string{"--name", "option_data_page"})
	if err == nil || !strings.Contains(err.Error(), `unknown page "option_data_page"`) {
		t.Fatalf("err %v", err)
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
	a.deps.optionDataPage = newOptionDataPage
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, a.deps.newClock(), "", rd, echoHandler)
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
	a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	err := runServe(a, []string{"--site", "--addr", "127.0.0.1:0"})
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
	}), log
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
