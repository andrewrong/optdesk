package main

import (
	"context"
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
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
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
			want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh"}
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
			want := []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries, jobOptionEod, jobQuoteSnapshot}
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

func (m *memEvents) InsertEarningsDate(_ context.Context, r contracts.EarningsDate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, r)
	return nil
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

// reconcilerOf runs the jobs it was built with on every Reconcile, as the ticker's reconciler does.
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
