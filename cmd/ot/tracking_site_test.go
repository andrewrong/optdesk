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

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// trackingFixture is an app whose optionTrackingPage fake builds {"schema":"option_tracking_page"} and records
// the readers and the days it was given.
type trackingFixture struct {
	a      *app
	builds atomic.Int32
	days   []contracts.Day
	got    trackingReaders
	err    error
}

func newTrackingFixture(t *testing.T, env string) *trackingFixture {
	t.Helper()
	f := &trackingFixture{}
	f.a, _ = testApp(t, env)
	f.a.deps.optionTrackingPage = func(_ *time.Location, _, _, _ string, r trackingReaders) pages.Page {
		f.got = r
		build := func(_ context.Context, _ store.SeriesReader, day contracts.Day, _ time.Time) ([]byte, error) {
			f.builds.Add(1)
			f.days = append(f.days, day)
			if f.err != nil {
				return nil, f.err
			}
			return []byte(`{"schema":"option_tracking_page"}`), nil
		}
		return pages.Page{Meta: pages.Meta{Name: trackingName}, Build: build}
	}
	return f
}

func trackingGet(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return rec
}

var trackingClock = fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}

func TestTrackingRouteServesPage(t *testing.T) {
	t.Parallel()
	f := newTrackingFixture(t, "")
	led, acct := demoTrackingLedger{}, demoNoticeAccount{}
	rd := siteReaders{optionLedger: led, noticeAccount: acct, cands: demoCandidates{}}
	h := siteHandler(f.a, trackingClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := trackingGet(t, h, http.MethodGet, "/"+siteTrackingRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_tracking_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	if f.got.Ledger != led || f.got.Account != acct || f.got.Sessions != nil || len(f.days) != 1 || f.days[0] != "2026-11-05" {
		t.Fatalf("readers %+v days %v", f.got, f.days)
	}
	h = siteHandler(f.a, trackingClock, "", rd, echoHandler, http.NotFoundHandler())
	if rec := trackingGet(t, h, http.MethodGet, "/"+siteTrackingRoute); rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	if len(f.days) != 2 || f.days[1] != "2026-11-06" {
		t.Fatalf("days %v", f.days)
	}
}

func TestTrackingRouteOnlyGet(t *testing.T) {
	t.Parallel()
	f := newTrackingFixture(t, "")
	h := siteHandler(f.a, trackingClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	if rec := trackingGet(t, h, http.MethodHead, "/"+siteTrackingRoute); rec.Code != 200 {
		t.Errorf("HEAD: %d", rec.Code)
	}
	before := f.builds.Load()
	if rec := trackingGet(t, h, http.MethodGet, "/"+siteTrackingRoute+"/x"); rec.Code != 404 {
		t.Errorf("sub path: %d", rec.Code)
	}
	rec := trackingGet(t, h, http.MethodPost, "/"+siteTrackingRoute)
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" || f.builds.Load() != before {
		t.Errorf("POST: %d allow %q builds %d", rec.Code, rec.Header().Get("Allow"), f.builds.Load())
	}
}

func TestTrackingBuildErrorIs500(t *testing.T) {
	t.Parallel()
	f := newTrackingFixture(t, "")
	f.err = errors.New("dsn sentinel-secret")
	h := siteHandler(f.a, trackingClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := trackingGet(t, h, http.MethodGet, "/"+siteTrackingRoute)
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != "internal error" {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	f.a.profile.Zone = nil
	if _, err := buildTracking(t.Context(), f.a, trackingClock, "", siteReaders{}); err == nil {
		t.Fatal("nil zone built")
	}
}

type tLedgerStore struct {
	store.OptionLedgerStore
	closeCounter
}

func (s tLedgerStore) Close() error { return s.closeCounter.Close() }

func TestTrackingUsesPagesDSN(t *testing.T) {
	t.Parallel()
	const env = store.DSNEnvPages + "=pages-dsn\n" + store.DSNEnvOptionLedger + "=ledger-dsn\n" + store.DSNEnvAccount + "=account-dsn\n"
	f := newTrackingFixture(t, env)
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
	ledOpens := 0
	want := tLedgerStore{closeCounter: cc}
	f.a.deps.openOptionLedgerRO = func(_ context.Context, d string) (store.OptionLedgerStore, error) {
		ledOpens++
		opened(d)
		return want, nil
	}
	rd, closeFn, err := openSiteReaders(t.Context(), f.a)
	if err != nil {
		t.Fatal(err)
	}
	h := siteHandler(f.a, trackingClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	for range 3 {
		if rec := trackingGet(t, h, http.MethodGet, "/"+siteTrackingRoute); rec.Code != 200 {
			t.Fatalf("code %d", rec.Code)
		}
	}
	if ledOpens != 1 || f.builds.Load() != 3 || f.got.Ledger != want {
		t.Fatalf("opens %d builds %d reader %v", ledOpens, f.builds.Load(), f.got.Ledger)
	}
	if len(dsns) != 8 {
		t.Fatalf("opened %v", dsns)
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

func TestTrackingWithoutLedgerSaysNoReader(t *testing.T) {
	t.Parallel()
	f := newTrackingFixture(t, store.DSNEnvPages+"=pages-dsn\n")
	f.a.profile.Name = "real"
	// No ledger opener: the reader reaches the builder as a nil interface.
	rd := siteReaders{}
	if _, err := buildTracking(t.Context(), f.a, trackingClock, "2026-11-05", rd); err != nil {
		t.Fatal(err)
	}
	if f.got.Ledger != nil {
		t.Fatalf("ledger %v", f.got.Ledger)
	}
	// The real builder says so on the page.
	f.a.deps.optionTrackingPage = newOptionTrackingPage
	body, err := buildTracking(t.Context(), f.a, trackingClock, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "no_reader") {
		t.Fatalf("no gap in %s", body)
	}
}

func TestTrackingDemoNeedsNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.optionTrackingPage = newOptionTrackingPage
	a.deps.openOptionLedgerRO = func(context.Context, string) (store.OptionLedgerStore, error) {
		t.Error("ledger store opened")
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
	h := siteHandler(a, trackingClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := trackingGet(t, h, http.MethodGet, "/"+siteTrackingRoute)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"schema":"option_tracking_page"`) || !strings.Contains(body, "DEMO") {
		t.Fatalf("%d %q", rec.Code, body)
	}
}

func TestPageCommandBuildsTracking(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.optionTrackingPage = newOptionTrackingPage
	var out strings.Builder
	a.stdout = &out
	if err := runPage(a, []string{"--name", trackingName, "--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"schema":"option_tracking_page"`) || !strings.Contains(out.String(), `"run_id":"ot-page"`) {
		t.Fatalf("page %s", out.String())
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, a.deps.newClock(), "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := trackingGet(t, h, http.MethodGet, "/"+siteTrackingRoute)
	site := strings.Replace(rec.Body.String(), `"run_id":"ot-serve"`, `"run_id":"ot-page"`, 1)
	if site != out.String() {
		t.Fatalf("site %s\ncommand %s", site, out.String())
	}
}
