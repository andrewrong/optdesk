package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// noticesFixture is an app whose optionNoticesPage fake builds {"schema":"option_notices_page"} and records
// the arguments and the days it was given.
type noticesFixture struct {
	a       *app
	builds  atomic.Int32
	days    []contracts.Day
	main    string
	aliases []string
	acct    store.AccountReader
	cands   store.CandidateReader
	earn    store.EarningsDateReader
	params  compute.NoticeParams
	err     error
}

func newNoticesFixture(t *testing.T, env string) *noticesFixture {
	t.Helper()
	f := &noticesFixture{}
	f.a, _ = testApp(t, env)
	f.a.deps.newOptionNotices = func() compute.OptionNotices { return demoNoticeRule{} }
	f.a.deps.noticeParams = func() compute.NoticeParams { return compute.NoticeParams{ExpiryDays: 9} }
	f.a.deps.optionNoticesPage = func(_ *time.Location, _, _, _, main string, aliases []string, acct store.AccountReader,
		cands store.CandidateReader, earn store.EarningsDateReader, _ compute.OptionNotices, p compute.NoticeParams) pages.Page {
		f.main, f.aliases, f.acct, f.cands, f.earn, f.params = main, aliases, acct, cands, earn, p
		build := func(_ context.Context, _ store.SeriesReader, day contracts.Day, _ time.Time) ([]byte, error) {
			f.builds.Add(1)
			f.days = append(f.days, day)
			if f.err != nil {
				return nil, f.err
			}
			return []byte(`{"schema":"option_notices_page"}`), nil
		}
		return pages.Page{Meta: pages.Meta{Name: noticesName}, Build: build}
	}
	return f
}

// noticesEnv writes a gateways file whose main account is "m" and returns the .env body that names it with
// the sockets m and b (listed in reverse order), the pages line and the lines that must not be read.
func noticesEnv(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gateways.toml")
	if err := os.WriteFile(p, []byte("[[gateway]]\nname = \"g1\"\naccount = \"m\"\nrequired = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return store.DSNEnvPages + "=pages-dsn\n" + store.DSNEnvAccount + "=account-dsn\n" + store.DSNEnvCandidate + "=candidate-dsn\n" +
		store.DSNEnvEvents + "=events-dsn\n" + envIbkrGateways + "=" + p + "\n" + envAccountSockets + "=m=/s/m,b=/s/b\n"
}

var noticesClock = fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}

func TestNoticesRouteServesPage(t *testing.T) {
	t.Parallel()
	f := newNoticesFixture(t, noticesEnv(t))
	f.a.profile.Name = "real"
	rd := siteReaders{cands: demoCandidates{}, noticeAccount: demoNoticeAccount{}}
	h := siteHandler(f.a, noticesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteNoticesRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_notices_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	if f.main != "m" || !reflect.DeepEqual(f.aliases, []string{"b", "m"}) || f.params.ExpiryDays != 9 {
		t.Fatalf("main %q aliases %v params %+v", f.main, f.aliases, f.params)
	}
	if f.acct != (demoNoticeAccount{}) || f.cands != (demoCandidates{}) || f.earn != nil {
		t.Fatalf("readers %v %v %v", f.acct, f.cands, f.earn)
	}
	if len(f.days) != 1 || f.days[0] != "2026-11-05" {
		t.Fatalf("days %v", f.days)
	}
	// Without --day the day is today.
	h = siteHandler(f.a, noticesClock, "", rd, echoHandler, http.NotFoundHandler())
	if rec := candidatesGet(t, h, http.MethodGet, "/"+siteNoticesRoute); rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	if len(f.days) != 2 || f.days[1] != "2026-11-06" {
		t.Fatalf("days %v", f.days)
	}
}

func TestNoticesRouteOnlyGet(t *testing.T) {
	t.Parallel()
	f := newNoticesFixture(t, "")
	f.a.profile.Name = "demo"
	h := siteHandler(f.a, noticesClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	if rec := candidatesGet(t, h, http.MethodHead, "/"+siteNoticesRoute); rec.Code != 200 {
		t.Errorf("HEAD: %d", rec.Code)
	}
	before := f.builds.Load()
	if rec := candidatesGet(t, h, http.MethodGet, "/"+siteNoticesRoute+"/x"); rec.Code != 404 {
		t.Errorf("sub path: %d", rec.Code)
	}
	rec := candidatesGet(t, h, http.MethodPost, "/"+siteNoticesRoute)
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" || f.builds.Load() != before {
		t.Errorf("POST: %d allow %q builds %d", rec.Code, rec.Header().Get("Allow"), f.builds.Load())
	}
}

func TestNoticesBuildErrorIs500(t *testing.T) {
	t.Parallel()
	f := newNoticesFixture(t, "")
	f.a.profile.Name = "demo"
	f.err = errors.New("dsn sentinel-secret")
	h := siteHandler(f.a, noticesClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteNoticesRoute)
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != "internal error" {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestNoticesUsesPagesDSN(t *testing.T) {
	t.Parallel()
	f := newNoticesFixture(t, noticesEnv(t))
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
	wantCands := cCands{closeCounter: cc}
	f.a.deps.openCandidate = func(_ context.Context, d string) (candidateStore, error) {
		opened(d)
		return wantCands, nil
	}
	wantAcct := cAccount{closeCounter: cc}
	acctOpens, earnOpens := 0, 0
	f.a.deps.openAccount = func(_ context.Context, d string) (store.AccountStore, error) {
		acctOpens++
		opened(d)
		return wantAcct, nil
	}
	wantEarn := cEarn{closeCounter: cc}
	f.a.deps.openEarningsDates = func(_ context.Context, d string) (store.EarningsDateStore, error) {
		earnOpens++
		opened(d)
		return wantEarn, nil
	}
	rd, closeFn, err := openSiteReaders(t.Context(), f.a)
	if err != nil {
		t.Fatal(err)
	}
	h := siteHandler(f.a, noticesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	for range 3 {
		if rec := candidatesGet(t, h, http.MethodGet, "/"+siteNoticesRoute); rec.Code != 200 {
			t.Fatalf("code %d", rec.Code)
		}
	}
	if acctOpens != 1 || earnOpens != 1 || f.builds.Load() != 3 || f.acct != wantAcct || f.cands != wantCands || f.earn != wantEarn {
		t.Fatalf("opens %d %d builds %d readers %v %v %v", acctOpens, earnOpens, f.builds.Load(), f.acct, f.cands, f.earn)
	}
	if len(dsns) != 10 {
		t.Fatalf("opened %d stores", len(dsns))
	}
	for _, d := range dsns {
		if d != "pages-dsn" {
			t.Fatalf("dsn %q", d)
		}
	}
	closeFn()
	if closed.Load() != 10 {
		t.Fatalf("closed %d", closed.Load())
	}
}

func TestNoticesDemoNeedsNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.optionNoticesPage = newOptionNoticesPage
	a.deps.newOptionNotices = func() compute.OptionNotices { t.Error("the real rule was used"); return demoNoticeRule{} }
	a.deps.noticeParams = func() compute.NoticeParams { return compute.NoticeParams{} }
	refuse := func(name string) { t.Errorf("%s store opened", name) }
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		refuse("series")
		return nil, errors.New("x")
	}
	a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		refuse("candidate")
		return nil, errors.New("x")
	}
	a.deps.openAccount = func(context.Context, string) (store.AccountStore, error) {
		refuse("account")
		return nil, errors.New("x")
	}
	a.deps.openEarningsDates = func(context.Context, string) (store.EarningsDateStore, error) {
		refuse("earnings")
		return nil, errors.New("x")
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, noticesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteNoticesRoute)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"schema":"option_notices_page"`) {
		t.Fatalf("%d %q", rec.Code, body)
	}
	for _, s := range []string{`"severity":"act"`, `"severity":"warn"`, `"severity":"info"`, `"status":"no_capture"`, `"alias":"second"`} {
		if !strings.Contains(body, s) {
			t.Errorf("demo page lacks %s: %s", s, body)
		}
	}
}

func TestPageCommandBuildsNotices(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.optionNoticesPage = newOptionNoticesPage
	a.deps.newOptionNotices = func() compute.OptionNotices { return demoNoticeRule{} }
	a.deps.noticeParams = func() compute.NoticeParams { return compute.NoticeParams{} }
	var out strings.Builder
	a.stdout = &out
	if err := runPage(a, []string{"--name", noticesName, "--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"schema":"option_notices_page"`) || !strings.Contains(out.String(), `"run_id":"ot-page"`) {
		t.Fatalf("page %s", out.String())
	}
	// The command and the site build the same page for the same day (run id apart).
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, a.deps.newClock(), "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+siteNoticesRoute, nil))
	site := strings.Replace(rec.Body.String(), `"run_id":"ot-serve"`, `"run_id":"ot-page"`, 1)
	if site != out.String() {
		t.Fatalf("site %s\ncommand %s", site, out.String())
	}
}

type cAccount struct {
	store.AccountStore
	closeCounter
}

func (c cAccount) Close() error { return c.closeCounter.Close() }

type cEarn struct {
	store.EarningsDateStore
	closeCounter
}

func (c cEarn) Close() error { return c.closeCounter.Close() }
