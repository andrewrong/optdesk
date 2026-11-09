package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// marketSiteApp is an app whose marketPage fake builds {"schema":"market_page","indices":"<value>"} from the
// reader it is given, and counts its builds and the days it was built for.
func marketSiteApp(t *testing.T, env string, builds *atomic.Int32, days *[]contracts.Day, got *compute.ReadingFunc) *app {
	t.Helper()
	a, _ := testApp(t, env)
	a.deps.marketPage = func(_ *time.Location, _, _, _ string, readings compute.ReadingFunc) pages.Page {
		if got != nil {
			*got = readings
		}
		build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, _ time.Time) ([]byte, error) {
			builds.Add(1)
			if days != nil {
				*days = append(*days, day)
			}
			rows, err := readings(ctx, "market.vix_pctile_3y", "market", "VIXCLS", day, day)
			if err != nil {
				return nil, err
			}
			v := "none"
			if len(rows) > 0 {
				v = *rows[0].Value
			}
			return []byte(`{"schema":"market_page","indices":"` + v + `"}`), nil
		}
		return pages.Page{Meta: pages.Meta{Name: "market_page"}, Build: build}
	}
	return a
}

func marketGet(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return rec
}

func TestSiteMarketRoute(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	var days []contracts.Day
	a := marketSiteApp(t, "", &builds, &days, nil)
	view := &fakeView{}
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	h := siteHandler(a, clk, "2026-11-05", siteReaders{view: view}, echoHandler, http.NotFoundHandler())

	view.set(viewRow("2026-11-05", "0.25", "2026-11-05"))
	first := marketGet(t, h, http.MethodGet, "/market")
	view.set(viewRow("2026-11-05", "0.75", "2026-11-05"))
	second := marketGet(t, h, http.MethodGet, "/market")
	if first.Code != 200 || second.Code != 200 || builds.Load() != 2 {
		t.Fatalf("codes %d %d builds %d", first.Code, second.Code, builds.Load())
	}
	if !strings.Contains(first.Body.String(), `"schema":"market_page"`) ||
		!strings.Contains(first.Body.String(), `"indices":"0.25"`) || !strings.Contains(second.Body.String(), `"indices":"0.75"`) {
		t.Fatalf("bodies %q %q", first.Body, second.Body)
	}
	if len(days) != 2 || days[0] != "2026-11-05" || days[1] != "2026-11-05" {
		t.Fatalf("days %v", days)
	}
	if rec := marketGet(t, h, http.MethodHead, "/market"); rec.Code != 200 {
		t.Errorf("HEAD: %d", rec.Code)
	}
	before := builds.Load()
	if rec := marketGet(t, h, http.MethodGet, "/market/x"); rec.Code != 404 {
		t.Errorf("market/x: %d", rec.Code)
	}
	rec := marketGet(t, h, http.MethodPost, "/market")
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" || builds.Load() != before {
		t.Errorf("POST: %d builds %d", rec.Code, builds.Load())
	}

	bad := &fakeView{err: errors.New("dsn sentinel-secret")}
	h = siteHandler(a, clk, "2026-11-05", siteReaders{view: bad}, echoHandler, http.NotFoundHandler())
	rec = marketGet(t, h, http.MethodGet, "/market")
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != "internal error" {
		t.Fatalf("reader error: %d %q", rec.Code, rec.Body)
	}
}

func TestMarketSiteOpensPagesRoleOnly(t *testing.T) {
	t.Parallel()
	const env = store.DSNEnvPages + "=pages-dsn\nOT_PG_DSN_READINGS=readings-dsn\nOT_PG_DSN_LOADER=loader-dsn\n"
	var builds atomic.Int32
	a := marketSiteApp(t, env, &builds, nil, nil)
	a.profile.Name = "real"
	view := &fakeView{}
	view.set(viewRow("2026-11-05", "0.25", "2026-11-05"))
	var dsns []string
	var closed atomic.Int32
	cc := closeCounter{&closed}
	a.deps.openSeries = func(_ context.Context, d string) (store.SeriesStore, error) {
		dsns = append(dsns, d)
		return cSeries{closeCounter: cc}, nil
	}
	a.deps.openOptionEod = func(_ context.Context, d string) (store.OptionEodStore, error) {
		dsns = append(dsns, d)
		return cEod{closeCounter: cc}, nil
	}
	a.deps.openQuoteSnapshot = func(_ context.Context, d string) (store.QuoteSnapshotStore, error) {
		dsns = append(dsns, d)
		return cQuotes{closeCounter: cc}, nil
	}
	a.deps.openOptionCatalog = func(_ context.Context, d string) (store.OptionEodCatalogStore, error) {
		dsns = append(dsns, d)
		return cCatalog{closeCounter: cc}, nil
	}
	a.deps.openOptionTerms = func(_ context.Context, d string) (store.OptionTermsStore, error) {
		dsns = append(dsns, d)
		return cTerms{closeCounter: cc}, nil
	}
	a.deps.openInstrumentName = func(_ context.Context, d string) (store.InstrumentNameStore, error) {
		dsns = append(dsns, d)
		return cNames{closeCounter: cc}, nil
	}
	a.deps.openCandidate = func(_ context.Context, d string) (candidateStore, error) {
		dsns = append(dsns, d)
		return cCands{closeCounter: cc}, nil
	}
	viewOpens := 0
	a.deps.openReadingView = func(_ context.Context, d string) (store.ReadingViewReader, error) {
		viewOpens++
		dsns = append(dsns, d)
		return view, nil
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	for range 3 {
		if rec := marketGet(t, h, http.MethodGet, "/market"); rec.Code != 200 {
			t.Fatalf("code %d", rec.Code)
		}
	}
	if viewOpens != 1 || builds.Load() != 3 {
		t.Fatalf("view opens %d builds %d", viewOpens, builds.Load())
	}
	for _, d := range dsns {
		if d != "pages-dsn" {
			t.Fatalf("dsn %q", d)
		}
	}

	const secret = "secret-dsn-text"
	b := marketSiteApp(t, "OT_PG_DSN_READINGS="+secret+"\n", &builds, nil, nil)
	b.profile.Name = "real"
	b.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("view opened without the pages line")
		return nil, nil
	}
	_, _, err = openSiteReaders(t.Context(), b)
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvPages) || strings.Contains(err.Error(), secret) {
		t.Fatalf("err %v", err)
	}
}

func TestMarketSiteDemoOpensNoStore(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	var got compute.ReadingFunc
	a := marketSiteApp(t, "", &builds, nil, &got)
	a.profile.Name = "demo"
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("series store opened")
		return nil, errors.New("opened")
	}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("view opened")
		return nil, errors.New("opened")
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := marketGet(t, h, http.MethodGet, "/market")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"market_page"`) || got == nil {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	for _, f := range []string{"market.vix_pctile_3y", "market.vxn_pctile_3y", "market.rvx_pctile_3y", "market.sp500_vs_200d", "market.vix_term_ratio"} {
		rows, err := got(t.Context(), f, "market", "VIXCLS", "2026-11-05", "2026-11-05")
		if err != nil || len(rows) != 1 || rows[0].Value == nil || len(rows[0].Parameters) != 1 ||
			rows[0].Parameters[0].Param != "data_day" || *rows[0].Parameters[0].ValueTag != "2026-11-05" {
			t.Fatalf("%s: %+v %v", f, rows, err)
		}
	}
	if rows, _ := got(t.Context(), "macro.other", "market", "X", "2026-11-05", "2026-11-05"); len(rows) != 0 {
		t.Fatalf("other figure has rows %+v", rows)
	}
}
