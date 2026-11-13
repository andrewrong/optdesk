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
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
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
		f := &fakeBackfiller{res: runner.BackfillResult{Keys: []runner.KeyRun{{
			Key: contracts.ExpectedKey{DatasetID: "d", SubjectKey: "s"}, Group: runner.GroupRan, Mark: "present", Usable: 3}}}}
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
		want := "key day=2026-11-04 dataset=d subject=s outcome=written rows=3\n" +
			"backfill 2026-11-04 status=ok written=1 present=0 no_value=0 failed=0 skipped=0\n"
		if stdout.String() != want {
			t.Fatalf("stdout %q, want %q", stdout.String(), want)
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
	check := func(t *testing.T, a *app, errb interface{ String() string }, hidePgx bool) {
		err := backfill(a, []string{"--day", "2026-11-04"})
		if err == nil {
			t.Fatal("want an error")
		}
		a.log.Error("command failed", "err", err.Error())
		secrets := []string{dsn, "sentinel-pw", fredKey}
		if hidePgx {
			secrets = append(secrets, pgxText)
		} else if !strings.Contains(err.Error(), pgxText) {
			t.Fatalf("a plain failure text is not printed: %v", err)
		}
		for _, s := range secrets {
			if strings.Contains(err.Error(), s) || strings.Contains(errb.String(), s) {
				t.Fatalf("leaked %q: %v / %q", s, err, errb.String())
			}
		}
	}
	t.Run("runner error", func(t *testing.T) {
		a, _, _, errb := backfillApp(t, env, &fakeBackfiller{err: errors.New(pgxText + " " + dsn + " key=" + fredKey + " token=sentinel-pw")})
		check(t, a, errb, false)
	})
	t.Run("open error", func(t *testing.T) {
		a, _, _, errb := backfillApp(t, env, &fakeBackfiller{})
		a.deps.openLedger = func(_ context.Context, d string) (store.OpsLedger, error) {
			return nil, errors.New(pgxText + " " + d)
		}
		check(t, a, errb, true)
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

func missingKeys() []contracts.ExpectedKey {
	return []contracts.ExpectedKey{{DatasetID: "option_eod", SubjectKey: "SPY"}, {DatasetID: "fred", SubjectKey: "DGS10"}}
}

func TestBackfillFailedRunPrintsKeyLinesThenError(t *testing.T) {
	t.Parallel()
	keys := []runner.KeyRun{
		{Key: contracts.ExpectedKey{DatasetID: "option_eod", SubjectKey: "SPY"}, Group: runner.GroupRan, Mark: "present", Usable: 5},
		{Key: contracts.ExpectedKey{DatasetID: "fred", SubjectKey: "DGS10"}, Group: runner.GroupRan, ErrorCode: "upstream"},
		{Key: contracts.ExpectedKey{DatasetID: "fred", SubjectKey: "DGS2"}, Group: runner.GroupNotReached},
	}
	a, _, _, _ := backfillApp(t, fullEnv, &fakeBackfiller{res: runner.BackfillResult{Keys: keys}, err: errors.New("plain failure")})
	var stdout bytes.Buffer
	a.stdout = &stdout
	err := backfill(a, []string{"--day", "2026-11-04"})
	if err == nil || err.Error() != "backfill 2026-11-04 failed: plain failure" {
		t.Fatalf("err %v", err)
	}
	want := "key day=2026-11-04 dataset=option_eod subject=SPY outcome=written rows=5\n" +
		"key day=2026-11-04 dataset=fred subject=DGS10 outcome=failed rows=0 reason=upstream\n" +
		"key day=2026-11-04 dataset=fred subject=DGS2 outcome=skipped rows=0 reason=not_reached\n"
	if stdout.String() != want {
		t.Fatalf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestBackfillRefusalPrintsKeyLines(t *testing.T) {
	t.Parallel()
	res := runner.BackfillResult{Refused: runner.RefusedNoPlan, Keys: []runner.KeyRun{
		{Key: contracts.ExpectedKey{DatasetID: "d", SubjectKey: "s"}, Group: runner.GroupAlreadyPresent}}}
	a, _, _, _ := backfillApp(t, fullEnv, &fakeBackfiller{res: res})
	var stdout bytes.Buffer
	a.stdout = &stdout
	if err := backfill(a, []string{"--day", "2026-11-04"}); err == nil {
		t.Fatal("want an error")
	}
	if want := "key day=2026-11-04 dataset=d subject=s outcome=present rows=0\n"; stdout.String() != want {
		t.Fatalf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestBackfillMissingKeysAreFailedLinesAndAnError(t *testing.T) {
	t.Parallel()
	keys := []runner.KeyRun{{Key: missingKeys()[0], Group: runner.GroupRan, ErrorCode: "boom"}}
	a, _, _, _ := backfillApp(t, fullEnv, &fakeBackfiller{res: runner.BackfillResult{Keys: keys, Missing: missingKeys()[:1]}})
	var stdout bytes.Buffer
	a.stdout = &stdout
	err := backfill(a, []string{"--day", "2026-11-04"})
	if err == nil || err.Error() != "backfill 2026-11-04: 1 key(s) are still missing" {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(stdout.String(), "outcome=failed") || strings.Contains(stdout.String(), "missing day=") {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestBackfillNoValueDayIsNothingWritten(t *testing.T) {
	t.Parallel()
	keys := []runner.KeyRun{{Key: missingKeys()[0], Group: runner.GroupRan, Mark: "no_value", ReasonCode: "after_stop"}}
	a, _, _, errb := backfillApp(t, fullEnv, &fakeBackfiller{res: runner.BackfillResult{Keys: keys, InWindow: missingKeys()[1:]}})
	var stdout bytes.Buffer
	a.stdout = &stdout
	if err := backfill(a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatalf("err %v", err)
	}
	out := stdout.String()
	if !strings.HasSuffix(out, "backfill 2026-11-04 status=nothing_written written=0 present=0 no_value=1 failed=0 skipped=1\n") ||
		!strings.Contains(out, "dataset=fred subject=DGS10 outcome=skipped rows=0 reason=in_window") {
		t.Fatalf("stdout %q", out)
	}
	if log := errb.String(); !strings.Contains(log, "status=nothing_written") || strings.Contains(log, "status=ok") {
		t.Fatalf("log %q", log)
	}
}

// bfPlanJob is a fake job with its own id and plan.
type bfPlanJob struct {
	fakeJob
	id   string
	keys []contracts.ExpectedKey
	err  error
}

func (j bfPlanJob) ID() string { return j.id }
func (j bfPlanJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.keys, "", j.err
}

// jobBackfillApp is backfillApp whose calendar_refresh slot holds j: fred_series is the real job now, so
// the hook of another job carries the fake.
func jobBackfillApp(t *testing.T, j bfPlanJob, f *fakeBackfiller, g *fakeKeySyncer) *app {
	t.Helper()
	a, _, _, _ := backfillApp(t, fullEnv, f)
	fakeBackfillDeps(a, f, g)
	a.deps.newCalendarRefresh = func(
		clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, calendarrefresh.CalendarObserver,
	) (contracts.Job, error) {
		return j, nil
	}
	return a
}

func twoDatasetJob() bfPlanJob {
	return bfPlanJob{id: "twoset", keys: []contracts.ExpectedKey{
		{DatasetID: "ds_b", SubjectKey: "x"}, {DatasetID: "ds_a", SubjectKey: "y"}, {DatasetID: "ds_b", SubjectKey: "z"},
	}}
}

func TestBackfillJob(t *testing.T) {
	t.Parallel()
	day := []string{"--day", "2026-11-04"}
	t.Run("datasets of the plan in plan order", func(t *testing.T) {
		f := &fakeBackfiller{}
		a := jobBackfillApp(t, twoDatasetJob(), f, &fakeKeySyncer{})
		if err := backfill(a, append(day, "--job", "twoset")); err != nil {
			t.Fatal(err)
		}
		if len(f.datasets) != 1 || strings.Join(f.datasets[0], ",") != "ds_b,ds_a" {
			t.Fatalf("datasets %v", f.datasets)
		}
	})
	t.Run("no job leaves datasets nil", func(t *testing.T) {
		f := &fakeBackfiller{}
		a := jobBackfillApp(t, twoDatasetJob(), f, &fakeKeySyncer{})
		if err := backfill(a, day); err != nil {
			t.Fatal(err)
		}
		if len(f.datasets) != 1 || f.datasets[0] != nil {
			t.Fatalf("datasets %v", f.datasets)
		}
	})
	t.Run("job and dataset of the job", func(t *testing.T) {
		f := &fakeBackfiller{}
		a := jobBackfillApp(t, twoDatasetJob(), f, &fakeKeySyncer{})
		if err := backfill(a, append(day, "--job", "twoset", "--dataset", "ds_a")); err != nil {
			t.Fatal(err)
		}
		if len(f.datasets) != 1 || strings.Join(f.datasets[0], ",") != "ds_a" {
			t.Fatalf("datasets %v", f.datasets)
		}
	})
	refused := []struct {
		name string
		job  bfPlanJob
		args []string
		want []string
		deny []string
	}{
		{"unknown job", twoDatasetJob(), []string{"--job", "nope"}, []string{"unknown job", "nope", "fake", "fred_series"}, nil},
		{"dataset not of the job", twoDatasetJob(), []string{"--job", "twoset", "--dataset", "other"},
			[]string{"dataset other is not a dataset of job twoset"}, nil},
		{"plan error", bfPlanJob{id: "bad", err: errors.New("postgres://u:sentinel-pw@h/db sentinel-key")}, []string{"--job", "bad"},
			[]string{"job bad", "plan failed", "[redacted]"}, []string{"postgres://", "sentinel-pw"}},
		{"no key", bfPlanJob{id: "empty"}, []string{"--job", "empty"}, []string{"job empty plans no key on the day"}, nil},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			f, g := &fakeBackfiller{}, &fakeKeySyncer{}
			if tc.name == "unknown job" {
				tc.job.id = "fred_series"
			}
			a := jobBackfillApp(t, tc.job, f, g)
			err := backfill(a, append(append([]string{}, day...), tc.args...))
			if err == nil {
				t.Fatal("want an error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("err %q lacks %q", err, w)
				}
			}
			for _, d := range tc.deny {
				if strings.Contains(err.Error(), d) {
					t.Fatalf("err %q holds %q", err, d)
				}
			}
			if len(f.days) != 0 || len(g.days) != 0 {
				t.Fatalf("backfill calls %d sync calls %d", len(f.days), len(g.days))
			}
		})
	}
}

func TestBackfillJobRunnerError(t *testing.T) {
	t.Parallel()
	day := []string{"--day", "2026-11-04"}
	{
		f := &fakeBackfiller{err: errors.New("boom")}
		a := jobBackfillApp(t, twoDatasetJob(), f, &fakeKeySyncer{})
		err := backfill(a, append(day, "--job", "twoset"))
		if err == nil || !strings.Contains(err.Error(), "backfill 2026-11-04 job twoset failed") {
			t.Fatalf("err %v", err)
		}
	}
}
