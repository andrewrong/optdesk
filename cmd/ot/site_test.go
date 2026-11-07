package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/embed"
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
	f.h = siteHandler(a, clk, "2026-11-06", siteReaders{catalog: f.cat}, func(vm []byte) http.Handler {
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
	a.deps.optionDataPage = newOptionDataPage
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, a.deps.newClock(), "", rd, embed.Handler)
	for _, p := range []string{"/options/data", "/options/data/9010"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, p, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_data_page"`) {
			t.Fatalf("%s: %d %.200q", p, rec.Code, rec.Body)
		}
	}
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
