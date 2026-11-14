package main

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
	"github.com/andrewrong/optdesk/internal/symbol"
	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jonboulle/clockwork"
)

func TestMissingKeyDisablesOnlyEarnings(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ env, line string }{
		"no key": {strings.Replace(fullEnv, "ALPHA_VANTAGE_API_KEY=", "ALPHA_VANTAGE_API_KEY=\n#", 1), "ALPHA_VANTAGE_API_KEY is not set"},
		"no dsn": {strings.Replace(fullEnv, "OT_PG_DSN_EVENTS=events-dsn\n", "", 1), "OT_PG_DSN_EVENTS is not set"},
	} {
		t.Run(name+" run-once", func(t *testing.T) {
			t.Parallel()
			r := newRig(t, tc.env)
			if err := runWired(r.a, []string{"--day", "2026-11-04"}); err != nil {
				t.Fatal(err)
			}
			want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "option_eod_index", "calendar_refresh"}
			if got := jobIDs(r.jobs); !slices.Equal(got, want) {
				t.Fatalf("jobs %v, want %v", got, want)
			}
			if log := r.log.String(); !strings.Contains(log, "earnings_refresh") || !strings.Contains(log, tc.line) {
				t.Fatalf("log %q lacks the named line", log)
			}
		})
		t.Run(name+" scheduler", func(t *testing.T) {
			t.Parallel()
			r := newSchedRig(t, tc.env)
			done := r.start()
			r.waitStarted(t, done)
			want := []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries, jobOptionEod, jobOptionEodIndex, jobQuoteSnapshot}
			if got := r.lastBuild(); !slices.Equal(got, want) {
				t.Fatalf("reconciler jobs %v, want %v", got, want)
			}
			if len(r.sched.disabled) != 1 || r.sched.disabled[jobEarningsRefresh] != tc.line {
				t.Fatalf("disabled %v", r.sched.disabled)
			}
			// The lines are read once at start, so a tick does not bring the job back.
			r.tick()
			if got := r.lastBuild(); !slices.Equal(got, want) {
				t.Fatalf("after a tick: %v", got)
			}
			_ = r.stop(t, done)
		})
	}

	// Neither a value nor a path reaches the log line.
	r := newRig(t, strings.Replace(strings.Replace(fullEnv, "OT_PG_DSN_EVENTS=events-dsn\n", "", 1), "-test-alpha", "-secret-key", 1))
	if err := runWired(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if log := r.log.String(); strings.Contains(log, "-secret-key") || strings.Contains(log, r.a.getenv("OT_HOME")) {
		t.Fatalf("log %q", log)
	}
}

// countSource is an EarningsSource that counts its Fetch calls.
type countSource struct{ calls atomic.Int32 }

func (c *countSource) Fetch(context.Context) ([]sources.EarningsListing, error) {
	c.calls.Add(1)
	return []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-11-20"}}, nil
}

// memEvents is one writer and reader of earnings rows over a slice.
type memEvents struct {
	store.EarningsDateStore
	mu   sync.Mutex
	rows []contracts.EarningsDate
}

var _ fundkey.EarningsUpserter = (*memEvents)(nil)

func (m *memEvents) InsertEarningsDate(ctx context.Context, r contracts.EarningsDate) error {
	_, err := m.UpsertEarnings(ctx, []contracts.EarningsDate{r}, "")
	return err
}

// UpsertEarnings keeps the rows by (Issuer, ObservedOn): a row of a stored key replaces it.
func (m *memEvents) UpsertEarnings(_ context.Context, rows []contracts.EarningsDate, _ string) (fundkey.WriteCount, error) {
	return m.put(rows, true), nil
}

// ReplayEarnings is UpsertEarnings that keeps a stored row of the same key.
func (m *memEvents) ReplayEarnings(_ context.Context, rows []contracts.EarningsDate, _ string) (fundkey.WriteCount, error) {
	return m.put(rows, false), nil
}

func (m *memEvents) put(rows []contracts.EarningsDate, replace bool) fundkey.WriteCount {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n fundkey.WriteCount
	for _, r := range rows {
		i := slices.IndexFunc(m.rows, func(x contracts.EarningsDate) bool {
			return x.Issuer == r.Issuer && x.ObservedOn.Format("2006-01-02") == r.ObservedOn.Format("2006-01-02")
		})
		switch {
		case i < 0:
			m.rows = append(m.rows, r)
			n.Written++
		case replace:
			m.rows[i] = r
			n.Written++
		default:
			n.Unchanged++
		}
	}
	return n
}

func (m *memEvents) EarningsDates(_ context.Context, issuer string, from, to contracts.Day) ([]contracts.EarningsDate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []contracts.EarningsDate
	for _, r := range m.rows {
		if d := r.ObservedOn.Format("2006-01-02"); r.Issuer == issuer && d >= string(from) && d <= string(to) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memEvents) Close() error { return nil }

// reconcilerOf runs the jobs it was built with on every Reconcile, as the ticker's reconciler does:
// a key whose Postcondition is true for the day is present and is not run.
type reconcilerOf struct{ jobs []contracts.Job }

func (r reconcilerOf) Reconcile(ctx context.Context, day contracts.Day) error {
	for _, j := range r.jobs {
		if j.ID() != jobEarningsRefresh {
			continue
		}
		keys, _, err := j.Plan(ctx, day)
		if err != nil {
			return err
		}
		if pc, ok := j.(runner.Postcondition); ok {
			var todo []contracts.ExpectedKey
			for _, k := range keys {
				if has, err := pc.Postcondition(ctx, string(day), k.DatasetID, k.SubjectKey); err != nil || !has {
					todo = append(todo, k)
				}
			}
			if len(todo) == 0 {
				continue
			}
			keys = todo
		}
		_ = j.Run(ctx, day, keys)
	}
	return nil
}

func TestEarningsRefreshOncePerDayScheduled(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	body := "[[underlying]]\nissuer = \"cik:0000320193\"\nmarket = \"US\"\nsymbol = \"AAPL\"\n"
	if err := os.WriteFile(filepath.Join(r.a.getenv("OT_HOME"), "watch.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	src, mem := &countSource{}, &memEvents{}
	d := &r.a.deps
	d.openEarningsDates = func(context.Context, string) (store.EarningsDateStore, error) { return mem, nil }
	d.newAlphaVantage = func(sources.EarningsConfig) (sources.EarningsSource, error) { return src, nil }
	d.newEarningsRefresh = func(
		s map[string]sources.EarningsSource, w store.EarningsDateWriter, rd store.EarningsDateReader, e []jobs.WatchEntry, _ clock.Clock, _ symbol.Translator,
	) (contracts.Job, error) {
		return earningsrefresh.New(s, w, rd, e, fakeClock{time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)})
	}
	d.newReconciler = func(
		_ store.OpsLedger, _ store.SchedulerLedger, _ clock.Calendar, _ clockwork.Clock, js []contracts.Job, _ runner.WindowRule, _ runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		return reconcilerOf{js}, nil
	}
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	r.tick()
	if n := src.calls.Load(); n != 1 {
		t.Fatalf("source called %d times on one day, want 1", n)
	}
	mem.mu.Lock()
	rows := len(mem.rows)
	mem.mu.Unlock()
	if rows != 1 {
		t.Fatalf("%d rows, want 1", rows)
	}
	_ = r.stop(t, done)
}

func earningsRow(issuer, day, source string) contracts.EarningsDate {
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	return contracts.EarningsDate{Issuer: issuer, Source: gen.EarningsDateSource(source), ObservedOn: types.SerializableDate{Time: d}}
}

func TestMemEventsUpsertReplacesAndReplayKeeps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := &memEvents{}
	if _, err := m.UpsertEarnings(ctx, []contracts.EarningsDate{earningsRow("a", "2026-11-09", "first")}, "w"); err != nil {
		t.Fatal(err)
	}
	second := []contracts.EarningsDate{earningsRow("a", "2026-11-09", "second"), earningsRow("a", "2026-11-10", "other-day")}
	replayed := []contracts.EarningsDate{earningsRow("a", "2026-11-09", "replayed"), earningsRow("b", "2026-11-09", "other-issuer")}
	if _, err := m.UpsertEarnings(ctx, second, "w"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ReplayEarnings(ctx, replayed, "w"); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range m.rows {
		got[r.Issuer+r.ObservedOn.Format("2006-01-02")] = string(r.Source)
	}
	want := map[string]string{"a2026-11-09": "second", "a2026-11-10": "other-day", "b2026-11-09": "other-issuer"}
	if len(m.rows) != 3 || !maps.Equal(got, want) {
		t.Fatalf("rows %v, want %v", got, want)
	}
	if err := m.InsertEarningsDate(ctx, earningsRow("a", "2026-11-09", "inserted")); err != nil {
		t.Fatal(err)
	}
	if len(m.rows) != 3 {
		t.Fatalf("InsertEarningsDate of a stored key made %d rows, want 3", len(m.rows))
	}
}

type presentJob struct {
	contracts.Job
	runs int
}

func (p *presentJob) ID() string { return jobEarningsRefresh }

func (p *presentJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "d", SubjectKey: "k"}}, "", nil
}

func (p *presentJob) Postcondition(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (p *presentJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	p.runs++
	return contracts.JobResult{}
}

func TestReconcilerOfSkipsPresentKey(t *testing.T) {
	t.Parallel()
	j := &presentJob{}
	if err := (reconcilerOf{jobs: []contracts.Job{j}}).Reconcile(context.Background(), "2026-11-14"); err != nil {
		t.Fatal(err)
	}
	if j.runs != 0 {
		t.Fatalf("runs = %d, want 0", j.runs)
	}
}
