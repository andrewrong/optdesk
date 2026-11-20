package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
	"github.com/jonboulle/clockwork"
)

// fakeJobRunStore records the calls the scheduler makes at start.
type fakeJobRunStore struct {
	jobrun.JobRunLedger
	jobrun.JobRunReader
	mu       sync.Mutex
	dsn      string
	cutAt    []time.Time
	released bool
}

func (f *fakeJobRunStore) CloseOpen(_ context.Context, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cutAt = append(f.cutAt, at)
	return 2, nil
}

func (f *fakeJobRunStore) Release() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = true
	return nil
}

// The scheduler opens the job-run store with the runner line and closes the rows the previous process left
// open as cut, once, at the start and at the clock of the scheduler.
func TestStartClosesOpenRunsCut(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	st := &fakeJobRunStore{}
	r.a.deps.openJobRun = func(_ context.Context, dsn string) (jobRunStore, error) {
		st.mu.Lock()
		st.dsn = dsn
		st.mu.Unlock()
		return st, nil
	}
	want := r.fw.Now()
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	_ = r.stop(t, done)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.dsn != "run-dsn" {
		t.Fatalf("store opened with %q, want the runner line", st.dsn)
	}
	if len(st.cutAt) != 1 || !st.cutAt[0].Equal(want) {
		t.Fatalf("CloseOpen calls %v, want one at %v", st.cutAt, want)
	}
	if !st.released {
		t.Fatal("the store was not released when the scheduler stopped")
	}
}

type countSlow struct{ n *int }

func (c countSlow) NotifySlowRuns(context.Context) (int, error) { *c.n++; return 0, nil }

// The slow-run pass runs in the notify turn of each tick, over the job-run store.
func TestSchedulerRunsSlowPassEachTick(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.a.deps.openJobRun = func(context.Context, string) (jobRunStore, error) { return &fakeJobRunStore{}, nil }
	var passes int
	r.a.deps.newSlowNotifier = func(d notify.SlowDeps, _ notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.SlowNotifier, error) {
		if d.Runs == nil || d.Mark == nil || d.Figures == nil {
			t.Error("slow deps incomplete")
		}
		return countSlow{&passes}, nil
	}
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	r.tick()
	_ = r.stop(t, done)
	if passes != 2 {
		t.Fatalf("slow passes %d, want 2", passes)
	}
}

// runningStore answers the open runs of the site pages.
type runningStore struct {
	fakeJobRunStore
	open []jobrun.JobRun
}

func (r *runningStore) Running(context.Context) ([]jobrun.JobRun, error) { return r.open, nil }

func (r *runningStore) Usuals(context.Context) ([]jobrun.UsualStat, error) { return nil, nil }

// siteWithRuns opens the reader through the opener of deps (the pages line) and serves path with it.
func siteWithRuns(t *testing.T, open openJobRunFunc, path string) string {
	t.Helper()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	a.deps.openJobRun = open
	rd := siteReaders{jobRuns: openJobRunReader(t.Context(), a, "postgres://pages", func(c io.Closer, err error) bool {
		if err == nil {
			t.Cleanup(func() { _ = c.Close() })
		}
		return err == nil
	})}
	h := siteHandler(a, fakeClock{dataHealthDay()}, "", rd, echoHandler, http.NotFoundHandler())
	rec := settingsGet(t, h, http.MethodGet, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status %d", path, rec.Code)
	}
	return rec.Body.String()
}

func openRunning(dsn *string) openJobRunFunc {
	started := dataHealthDay().Add(-15 * time.Minute)
	return func(_ context.Context, d string) (jobRunStore, error) {
		*dsn = d
		return &runningStore{open: []jobrun.JobRun{{ID: "r1", JobID: "compute_figures", StartedAt: started, KeysDone: 3, KeysTotal: 9, Step: "figures/US:AAPL"}}}, nil
	}
}

// The job console page carries the running line of the open run the reader of the site returns.
func TestJobConsolePageShowsRunningLine(t *testing.T) {
	t.Parallel()
	var dsn string
	body := siteWithRuns(t, openRunning(&dsn), "/"+siteJobConsoleRoute)
	if dsn != "postgres://pages" {
		t.Fatalf("reader opened with %q, want the pages line", dsn)
	}
	flat := strings.Join(strings.Fields(body), "")
	for _, want := range []string{`"running":[`, `"keys_done":3`, `"keys_total":9`, "compute_figures"} {
		if !strings.Contains(flat, want) {
			t.Errorf("job console page lacks %q", want)
		}
	}
}

// The data health page carries the same running line.
func TestDataHealthPageShowsRunningLine(t *testing.T) {
	t.Parallel()
	var dsn string
	body := siteWithRuns(t, openRunning(&dsn), "/"+siteDataHealthRoute)
	if dsn != "postgres://pages" {
		t.Fatalf("reader opened with %q, want the pages line", dsn)
	}
	for _, want := range []string{`"running":[`, "compute_figures"} {
		if !strings.Contains(body, want) {
			t.Errorf("data health page lacks %q", want)
		}
	}
}

// A failed open leaves the reader nil: both pages still build, with no running line.
func TestSitePagesBuildWhenJobRunOpenFails(t *testing.T) {
	t.Parallel()
	fail := func(context.Context, string) (jobRunStore, error) { return nil, errors.New("boom") }
	for _, route := range []string{siteJobConsoleRoute, siteDataHealthRoute} {
		body := siteWithRuns(t, fail, "/"+route)
		if strings.Contains(strings.Join(strings.Fields(body), ""), `"running":[`) {
			t.Errorf("%s: running line without a reader", route)
		}
	}
}

// The reader of the site is released when the site closes its stores.
func TestSiteReleasesJobRunReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	st := &runningStore{}
	a.deps.openJobRun = func(context.Context, string) (jobRunStore, error) { return st, nil }
	var closers []io.Closer
	if openJobRunReader(t.Context(), a, "dsn", func(c io.Closer, _ error) bool { closers = append(closers, c); return true }) == nil {
		t.Fatal("no reader")
	}
	for _, c := range closers {
		_ = c.Close()
	}
	if !st.released {
		t.Fatal("store not released")
	}
}

// openSiteReaders opens the job-run store once with the pages line, puts it in siteReaders.jobRuns and releases
// it through the close function it returns.
func TestOpenSiteReadersGivesJobRunReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	a.profile.Name = "real"
	var other atomic.Int32
	cc := closeCounter{&other}
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { return cSeries{closeCounter: cc}, nil }
	a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) { return cEod{closeCounter: cc}, nil }
	a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) {
		return cQuotes{closeCounter: cc}, nil
	}
	a.deps.openOptionCatalog = func(context.Context, string) (store.OptionEodCatalogStore, error) {
		return cCatalog{closeCounter: cc}, nil
	}
	a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) { return cTerms{closeCounter: cc}, nil }
	a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
		return cNames{closeCounter: cc}, nil
	}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) { return cView{closeCounter: cc}, nil }
	st := &runningStore{}
	var dsns []string
	a.deps.openJobRun = func(_ context.Context, dsn string) (jobRunStore, error) {
		dsns = append(dsns, dsn)
		return st, nil
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(dsns) != 1 || dsns[0] != "pages-dsn" {
		t.Fatalf("job-run store opened with %v, want one open with the pages line", dsns)
	}
	if rd.jobRuns != jobrun.JobRunReader(st) {
		t.Fatalf("siteReaders.jobRuns = %v, want the opened store", rd.jobRuns)
	}
	if st.released {
		t.Fatal("released before the close function")
	}
	closeFn()
	if !st.released {
		t.Fatal("the close function of openSiteReaders did not release the job-run store")
	}
}
