package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// backfillApp is a real-profile app with fake stores and the given fake backfiller.
func backfillApp(t *testing.T, env string, f *fakeBackfiller) (a *app, opened, closed *atomic.Int32, errb interface{ String() string }) {
	t.Helper()
	app, log := testApp(t, env)
	opened, closed = new(atomic.Int32), new(atomic.Int32)
	var got contracts.Day
	runOnceFakes(app, opened, closed, &got)
	fakeBackfillDeps(app, f, &fakeKeySyncer{})
	return app, opened, closed, log
}

// fakeKeySyncer records the days SyncKeys was called with.
type fakeKeySyncer struct {
	mu   sync.Mutex
	days []contracts.Day
	err  error
}

func (g *fakeKeySyncer) SyncKeys(_ context.Context, day contracts.Day) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.days = append(g.days, day)
	return g.err
}

// fakeBackfillDeps wires the deps ot backfill reads: the reconciler pair returning f, the scheduler ledger and the key gap syncer.
func fakeBackfillDeps(a *app, f *fakeBackfiller, g *fakeKeySyncer) {
	a.deps.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClock() }
	a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return &fakeSchedLedger{}, nil }
	a.deps.newKeyRunner = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyRunner { return nil }
	a.deps.newReconciler = func(
		store.OpsLedger, store.SchedulerLedger, clock.Calendar, clockwork.Clock, []contracts.Job, runner.WindowRule, runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		return nil, f
	}
	a.deps.newKeyGapSyncer = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyGapSyncer { return g }
}

type retryableErr struct{}

func (retryableErr) Error() string             { return "lease" }
func (retryableErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestBackfillFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
		demo bool
		want string
	}{
		{"no day", nil, false, "--day"},
		{"bad day", []string{"--day", "2026-13-40"}, false, "2026-13-40"},
		{"demo", []string{"--day", "2026-11-04"}, true, "backfill needs a real profile; the demo profile opens no store"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeBackfiller{}
			a, opened, _, _ := backfillApp(t, fullEnv, f)
			if tc.demo {
				a.profile.Name = "demo"
			}
			err := backfill(a, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want text %q", err, tc.want)
			}
			if opened.Load() != 0 || len(f.days) != 0 {
				t.Fatalf("opened %d calls %d", opened.Load(), len(f.days))
			}
		})
	}
}

func TestBackfillCallsRunner(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		f := &fakeBackfiller{res: runner.BackfillResult{Filled: []contracts.ExpectedKey{{DatasetID: "d", SubjectKey: "s"}}}}
		a, opened, closed, errb := backfillApp(t, fullEnv, f)
		var stdout bytes.Buffer
		a.stdout = &stdout
		if err := backfill(a, []string{"--day", "2026-11-04"}); err != nil {
			t.Fatal(err)
		}
		if len(f.days) != 1 || f.days[0] != "2026-11-04" || f.datasets[0] != nil {
			t.Fatalf("calls %v %v", f.days, f.datasets)
		}
		log := errb.String()
		if !strings.Contains(log, "backfill 2026-11-04 done") || !strings.Contains(log, "status=ok") {
			t.Fatalf("log %q", log)
		}
		if opened.Load() == 0 || opened.Load() != closed.Load() {
			t.Fatalf("opened %d closed %d", opened.Load(), closed.Load())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout %q", stdout.String())
		}
	})
	t.Run("dataset", func(t *testing.T) {
		f := &fakeBackfiller{}
		a, _, _, _ := backfillApp(t, fullEnv, f)
		if err := backfill(a, []string{"--day", "2026-11-04", "--dataset", "fred_series"}); err != nil {
			t.Fatal(err)
		}
		if len(f.datasets) != 1 || len(f.datasets[0]) != 1 || f.datasets[0][0] != "fred_series" {
			t.Fatalf("datasets %v", f.datasets)
		}
	})
	for name, f := range map[string]*fakeBackfiller{
		"runner error": {err: errors.New("boom")},
		"refusal":      {res: runner.BackfillResult{Refused: runner.RefusedNoPlan}},
	} {
		t.Run(name, func(t *testing.T) {
			a, opened, closed, _ := backfillApp(t, fullEnv, f)
			if err := backfill(a, []string{"--day", "2026-11-04"}); err == nil {
				t.Fatal("want an error")
			}
			if opened.Load() == 0 || opened.Load() != closed.Load() {
				t.Fatalf("opened %d closed %d", opened.Load(), closed.Load())
			}
		})
	}
}

func TestBackfillErrors(t *testing.T) {
	t.Parallel()
	key := contracts.ExpectedKey{DatasetID: "d", SubjectKey: "s"}
	for _, tc := range []struct {
		name    string
		res     runner.BackfillResult
		wantErr []string
	}{
		{"no plan", runner.BackfillResult{Refused: runner.RefusedNoPlan}, []string{"2026-11-04", "not planned yet"}},
		{"in window only", runner.BackfillResult{InWindow: []contracts.ExpectedKey{key}}, nil},
		{"missing", runner.BackfillResult{Missing: []contracts.ExpectedKey{key, key}}, []string{"2 key"}},
		{"absent only", runner.BackfillResult{Absent: []contracts.ExpectedKey{key}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _, _, _ := backfillApp(t, fullEnv, &fakeBackfiller{res: tc.res})
			err := backfill(a, []string{"--day", "2026-11-04"})
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("err %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("want an error")
			}
			for _, w := range tc.wantErr {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("err %q lacks %q", err, w)
				}
			}
		})
	}
}

func TestBackfillNoSecretInErrors(t *testing.T) {
	t.Parallel()
	const dsn, fredKey, pgxText = "postgres://u:sentinel-pw@h/db", "sentinel-key", "pgx: sentinel-failure"
	env := "OT_PG_DSN_RUNNER=" + dsn + "\nOT_PG_DSN_SERIES_FRED=s\nFRED_API_KEY=" + fredKey + "\n" +
		"OT_PG_DSN_OPTION_EOD=e\nOT_PG_DSN_QUOTE_SNAPSHOT=q\nOT_PG_DSN_MASTER=m\nOT_PG_DSN_PAGES=p\nOT_PG_DSN_READINGS=rd\nOT_PG_DSN_LOADER=ld\n"
	check := func(t *testing.T, a *app, errb interface{ String() string }) {
		err := backfill(a, []string{"--day", "2026-11-04"})
		if err == nil {
			t.Fatal("want an error")
		}
		a.log.Error("command failed", "err", err.Error())
		for _, s := range []string{dsn, "sentinel-pw", fredKey, pgxText} {
			if strings.Contains(err.Error(), s) || strings.Contains(errb.String(), s) {
				t.Fatalf("leaked %q: %v / %q", s, err, errb.String())
			}
		}
	}
	t.Run("runner error", func(t *testing.T) {
		a, _, _, errb := backfillApp(t, env, &fakeBackfiller{err: errors.New(pgxText + " " + dsn + " " + fredKey)})
		check(t, a, errb)
	})
	t.Run("open error", func(t *testing.T) {
		a, _, _, errb := backfillApp(t, env, &fakeBackfiller{})
		a.deps.openLedger = func(_ context.Context, d string) (store.OpsLedger, error) {
			return nil, errors.New(pgxText + " " + d)
		}
		check(t, a, errb)
	})
}

func TestBackfillAheadDay(t *testing.T) {
	t.Parallel()
	key := contracts.ExpectedKey{DatasetID: "fred_series", SubjectKey: "DGS10"}
	f := &fakeBackfiller{res: runner.BackfillResult{InWindow: []contracts.ExpectedKey{key}}}
	a, _, _, errb := backfillApp(t, fullEnv, f)
	var opened, closed atomic.Int32
	withCursor(a, &opened, &closed, "2026-03-03", nil)
	if err := backfill(a, []string{"--day", "2026-03-09"}); err != nil {
		t.Fatalf("a day past the fake cursor: %v", err)
	}
	if len(f.days) != 1 || f.days[0] != "2026-03-09" {
		t.Fatalf("calls %v", f.days)
	}
	if log := errb.String(); !strings.Contains(log, "backfill 2026-03-09 done") {
		t.Fatalf("log %q", log)
	}
}

func TestBackfillSyncsGaps(t *testing.T) {
	t.Parallel()
	f := &fakeBackfiller{}
	a, _, _, _ := backfillApp(t, fullEnv, f)
	g := &fakeKeySyncer{}
	fakeBackfillDeps(a, f, g)
	if err := backfill(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if len(g.days) != 1 || g.days[0] != "2026-03-04" {
		t.Fatalf("SyncKeys days %v", g.days)
	}
	g2 := &fakeKeySyncer{err: errors.New("sentinel-pw")}
	fakeBackfillDeps(a, f, g2)
	err := backfill(a, []string{"--day", "2026-03-04"})
	if err == nil || strings.Contains(err.Error(), "sentinel-pw") {
		t.Fatalf("sync error %v", err)
	}
}

func TestBackfillLeaseHeld(t *testing.T) {
	t.Parallel()
	f := &fakeBackfiller{err: retryableErr{}}
	a, _, closed, _ := backfillApp(t, fullEnv, f)
	g := &fakeKeySyncer{}
	fakeBackfillDeps(a, f, g)
	err := backfill(a, []string{"--day", "2026-03-04"})
	if err == nil || !strings.Contains(err.Error(), "the runner lease is held, retry in a minute") {
		t.Fatalf("err %v", err)
	}
	if len(g.days) != 0 || closed.Load() == 0 {
		t.Fatalf("sync calls %d closed %d", len(g.days), closed.Load())
	}
}

func TestBackfillNoFence(t *testing.T) {
	t.Parallel()
	f := &fakeBackfiller{}
	a, _, _, _ := backfillApp(t, fullEnv, f)
	a.deps.openFence = func(context.Context, string) (store.InstanceFence, error) {
		t.Error("openFence called")
		return nil, errors.New("no fence")
	}
	if err := backfill(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if len(f.days) != 1 {
		t.Fatalf("calls %v", f.days)
	}
}
