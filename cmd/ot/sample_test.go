package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestSampleRoutes(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../contracts/fixtures/page.sample_page.v1.ok.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	a.profile.Name = "real"
	fail := func() error { t.Error("an opener was called"); return errors.New("opened") }
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { return nil, fail() }
	a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) { return nil, fail() }
	a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) { return nil, fail() }
	a.deps.openOptionCatalog = func(context.Context, string) (store.OptionEodCatalogStore, error) { return nil, fail() }
	a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) { return nil, fail() }
	a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) { return nil, fail() }
	a.deps.pageList = []pages.Page{{
		Meta: pages.Meta{Name: "macro_rates"},
		Build: func(context.Context, store.SeriesReader, contracts.Day, time.Time) ([]byte, error) {
			return []byte(`{"schema":"macro_rates"}`), nil
		},
	}}
	a.deps.optionDataPage = func(*time.Location, string, sitePageInput) pages.Page {
		return fakePage("option_data_page", `{"schema":"option_data_page"}`)
	}
	h := siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "", siteReaders{catalog: &fakeCatalog{}, names: &fakeNames{}},
		func(vm []byte) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vm) })
		}, http.NotFoundHandler())
	get := func(path string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if rec.Code != 200 {
			t.Fatalf("%s: code %d", path, rec.Code)
		}
		var vm map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return vm
	}
	want := map[string]string{
		"/home": "home_page", "/positions": "positions_page",
		"/risk": "household_risk", "/advice": "advice_page",
	}
	for path, schema := range want {
		vm := get(path)
		if len(vm) != len(fixture) || vm["schema"] != fixture["schema"] || vm["schema_version"] != fixture["schema_version"] || vm["page"] != schema {
			t.Errorf("%s: vm %v fixture %v", path, vm, fixture)
		}
	}
	for _, path := range []string{"/macro", "/options/data"} {
		if vm := get(path); vm["schema"] == "sample_page" {
			t.Errorf("%s answered a sample page", path)
		}
	}
}
