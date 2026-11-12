package runner_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

type fakeReconciler struct{ days []contracts.Day }

func (f *fakeReconciler) Reconcile(_ context.Context, day contracts.Day) error {
	f.days = append(f.days, day)
	return nil
}

func TestReconcilerContract(t *testing.T) {
	t.Parallel()
	f := &fakeReconciler{}
	var newFn runner.NewFunc = func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job) runner.Reconciler { return f }
	r := newFn(nil, nil, nil, nil)
	if err := r.Reconcile(context.Background(), "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(f.days) != 1 || f.days[0] != "2026-11-04" {
		t.Fatalf("days = %v", f.days)
	}
}

func TestWindowRuleContract(t *testing.T) {
	t.Parallel()
	var got runner.WindowRule
	f := &fakeReconciler{}
	var newFn runner.NewWithRuleFunc = func(_ store.OpsLedger, _ clock.Calendar, _ clock.Clock, _ []contracts.Job, rule runner.WindowRule) runner.Reconciler {
		got = rule
		return f
	}
	want := runner.WindowRule{CalendarID: "X", OpenOffset: time.Minute, CloseOffset: time.Hour}
	r := newFn(nil, nil, nil, nil, want)
	if got != want {
		t.Fatalf("rule = %+v, want %+v", got, want)
	}
	if err := r.Reconcile(context.Background(), "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(f.days) != 1 {
		t.Fatalf("days = %v", f.days)
	}
}

type fakeBackfiller struct {
	result runner.BackfillResult
	gotDay contracts.Day
	gotIDs []string
}

func (f *fakeBackfiller) Backfill(_ context.Context, day contracts.Day, datasets []string) (runner.BackfillResult, error) {
	f.gotDay, f.gotIDs = day, datasets
	return f.result, nil
}

// describe is a caller: it branches on Refused, never on an error.
func describe(res runner.BackfillResult) string {
	switch res.Refused {
	case runner.RefusedNoPlan:
		return "no plan"
	case runner.RefusedNoKey:
		return "no key"
	default:
		return "done"
	}
}

func TestBackfillerRefusalIsDataWithNilError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		refused runner.BackfillRefusal
		wantStr string
		wantTxt string
	}{
		{"no plan", runner.RefusedNoPlan, "no_plan", "no plan"},
		{"not refused", "", "", "done"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeBackfiller{result: runner.BackfillResult{Refused: tc.refused}}
			var newFn runner.NewBackfillerWithRuleFunc = func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, runner.WindowRule) runner.Backfiller {
				return f
			}
			b := newFn(nil, nil, nil, nil, runner.WindowRule{})
			res, err := b.Backfill(context.Background(), "2026-11-06", []string{"DGS10"})
			if err != nil {
				t.Fatalf("a refusal carries a nil error, got %v", err)
			}
			if string(res.Refused) != tc.wantStr {
				t.Fatalf("Refused = %q, want %q", res.Refused, tc.wantStr)
			}
			if got := describe(res); got != tc.wantTxt {
				t.Fatalf("describe = %q, want %q", got, tc.wantTxt)
			}
			if f.gotDay != "2026-11-06" || len(f.gotIDs) != 1 || f.gotIDs[0] != "DGS10" {
				t.Fatalf("arguments = %v %v", f.gotDay, f.gotIDs)
			}
		})
	}
}

type fakeKeyGapSyncer struct {
	gotDay contracts.Day
	err    error
}

func (f *fakeKeyGapSyncer) SyncKeys(_ context.Context, day contracts.Day) error {
	f.gotDay = day
	return f.err
}

func TestBackfillInWindowAndKeyGapSyncerContract(t *testing.T) {
	t.Parallel()
	inWindow := []contracts.ExpectedKey{{DatasetID: "DGS10", SubjectKey: "x"}}
	f := &fakeBackfiller{result: runner.BackfillResult{InWindow: inWindow}}
	res, err := f.Backfill(context.Background(), "2026-11-09", nil)
	if err != nil || res.Refused != "" || len(res.InWindow) != 1 || res.InWindow[0] != inWindow[0] {
		t.Fatalf("InWindow = %v, refused %q, err %v", res.InWindow, res.Refused, err)
	}

	g := &fakeKeyGapSyncer{}
	var newFn runner.NewKeyGapSyncerFunc = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyGapSyncer {
		return g
	}
	s := newFn(nil, nil, clockwork.NewFakeClock())
	if err := s.SyncKeys(context.Background(), "2026-11-09"); err != nil {
		t.Fatalf("SyncKeys: %v", err)
	}
	if g.gotDay != "2026-11-09" {
		t.Fatalf("day = %q", g.gotDay)
	}
}

// fakeWindowed is a job-side type: it declares its window with plain values.
type fakeWindowed struct {
	calendarID  string
	openOffset  time.Duration
	closeOffset time.Duration
}

func (f fakeWindowed) Window() (string, time.Duration, time.Duration) {
	return f.calendarID, f.openOffset, f.closeOffset
}

var _ runner.Windowed = fakeWindowed{}

func TestWindowedContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		job  fakeWindowed
	}{
		{"all set", fakeWindowed{"XNYS", time.Minute, 5 * time.Hour}},
		{"empty calendar and negative open offset", fakeWindowed{"", -30 * time.Minute, 96 * time.Hour}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var job any = tc.job
			w, ok := job.(runner.Windowed)
			if !ok {
				t.Fatal("job does not satisfy runner.Windowed")
			}
			cal, open, closeOff := w.Window()
			if cal != tc.job.calendarID || open != tc.job.openOffset || closeOff != tc.job.closeOffset {
				t.Fatalf("Window() = %q %v %v, want %q %v %v", cal, open, closeOff,
					tc.job.calendarID, tc.job.openOffset, tc.job.closeOffset)
			}
		})
	}
}

// plainJob implements contracts.Job and nothing else.
type plainJob struct{}

func (plainJob) ID() string { return "plain" }
func (plainJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (plainJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

// fullJob implements every optional method with standard-library types only.
type fullJob struct{ plainJob }

func (fullJob) Schedule() (string, string, string, time.Duration) {
	return "XNYS", "session_close", "America/New_York", time.Hour
}
func (fullJob) RunBound() time.Duration { return 10 * time.Minute }
func (fullJob) Postcondition(_ context.Context, day, datasetID, subjectKey string) (bool, error) {
	return day == "2026-11-06" && datasetID == "d" && subjectKey == "k", nil
}
func (fullJob) RestartSensitive() bool { return true }

func TestOptionalMethods(t *testing.T) {
	t.Parallel()
	var plain any = plainJob{}
	if _, ok := plain.(runner.Scheduled); ok {
		t.Fatal("plain job must not satisfy Scheduled")
	}
	if _, ok := plain.(runner.Bounded); ok {
		t.Fatal("plain job must not satisfy Bounded")
	}
	if _, ok := plain.(runner.Postcondition); ok {
		t.Fatal("plain job must not satisfy Postcondition")
	}
	if _, ok := plain.(runner.RestartSensitive); ok {
		t.Fatal("plain job must not satisfy RestartSensitive")
	}

	var full any = fullJob{}
	s, ok := full.(runner.Scheduled)
	if !ok {
		t.Fatal("full job does not satisfy Scheduled")
	}
	cal, anchor, zone, off := s.Schedule()
	slot := runner.Slot{JobID: "full", CalendarID: cal, Anchor: anchor, Zone: zone, Offset: off}
	if slot.CalendarID != "XNYS" || slot.Anchor != "session_close" || slot.Zone != "America/New_York" || slot.Offset != time.Hour {
		t.Fatalf("slot = %+v", slot)
	}
	b, ok := full.(runner.Bounded)
	if !ok || b.RunBound() != 10*time.Minute {
		t.Fatal("Bounded not satisfied or wrong bound")
	}
	p, ok := full.(runner.Postcondition)
	if !ok {
		t.Fatal("full job does not satisfy Postcondition")
	}
	if got, err := p.Postcondition(context.Background(), "2026-11-06", "d", "k"); err != nil || !got {
		t.Fatalf("Postcondition = %v %v", got, err)
	}
	if got, _ := p.Postcondition(context.Background(), "2026-11-06", "d", "other"); got {
		t.Fatal("Postcondition true for a missing key")
	}
	r, ok := full.(runner.RestartSensitive)
	if !ok || !r.RestartSensitive() {
		t.Fatal("RestartSensitive not satisfied or false")
	}
}

type fakeKeyRunner struct {
	attempt int
	reason  string
	closeAt time.Time
	bound   time.Time
}

func (f *fakeKeyRunner) RunKey(
	_ context.Context, _ contracts.Job, _ contracts.Day, _ contracts.ExpectedKey,
	attempt int, closeAt, deadline time.Time, reason string,
) (store.Run, error) {
	f.attempt, f.reason, f.closeAt, f.bound = attempt, reason, closeAt, deadline
	return store.Run{}, nil
}

func TestKeyRunnerContract(t *testing.T) {
	t.Parallel()
	f := &fakeKeyRunner{}
	var newFn runner.NewKeyRunnerFunc = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyRunner { return f }
	kr := newFn(nil, nil, clockwork.NewFakeClock())
	closeAt := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	deadline := closeAt.Add(time.Minute)
	if _, err := kr.RunKey(context.Background(), plainJob{}, "2026-11-06", contracts.ExpectedKey{}, 2, closeAt, deadline, "late"); err != nil {
		t.Fatal(err)
	}
	if f.attempt != 2 || f.reason != "late" || !f.closeAt.Equal(closeAt) || !f.bound.Equal(deadline) {
		t.Fatalf("got %+v", f)
	}
}

type fakeGaps struct{ disabled map[string]string }

func (g *fakeGaps) Sync(_ context.Context, _ contracts.Day, disabled map[string]string) error {
	g.disabled = disabled
	return nil
}

type fakeScheduler struct {
	tick time.Time
	held bool
}

func (s *fakeScheduler) Run(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
func (s *fakeScheduler) LastTick() time.Time           { return s.tick }
func (s *fakeScheduler) FenceHeld() bool               { return s.held }

func TestSchedulerContract(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	gaps := &fakeGaps{}
	var gotDisabled map[string]string
	var gotExe runner.ExecIdentityFunc
	sch := &fakeScheduler{tick: clk.Now(), held: true}
	var newFn runner.NewSchedulerFunc = func(
		_ store.Fence, _ store.SchedulerLedger, _ store.OpsLedger, _ clock.Calendar, _ clockwork.Clock,
		_ runner.Reconciler, g runner.GapSyncer, _ []contracts.Job, disabled map[string]string, exe runner.ExecIdentityFunc,
	) runner.Scheduler {
		gotDisabled, gotExe = disabled, exe
		if err := g.Sync(context.Background(), "2026-11-06", disabled); err != nil {
			t.Error(err)
		}
		return sch
	}
	want := map[string]string{"j1": "no key"}
	s := newFn(nil, nil, nil, nil, clk, nil, gaps, nil, want, func() (string, error) { return "id1", nil })
	if gotDisabled["j1"] != "no key" || gaps.disabled["j1"] != "no key" {
		t.Fatalf("disabled = %v %v", gotDisabled, gaps.disabled)
	}
	if id, err := gotExe(); err != nil || id != "id1" {
		t.Fatalf("exe = %q %v", id, err)
	}
	if !s.LastTick().Equal(clk.Now()) || !s.FenceHeld() {
		t.Fatal("health surface wrong")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Run(ctx); err == nil {
		t.Fatal("Run on a cancelled context returned nil")
	}
}

// probingReconciler is a Reconciler that also implements Prober.
type probingReconciler struct {
	fakeReconciler
	reasons map[string]string
}

func (p *probingReconciler) Probe(_ context.Context, jobID string) string { return p.reasons[jobID] }

// probeOf is the caller side: it probes only when rec implements Prober.
func probeOf(ctx context.Context, rec runner.Reconciler, jobID string) (reason string, probed bool) {
	p, ok := rec.(runner.Prober)
	if !ok {
		return "", false
	}
	return p.Probe(ctx, jobID), true
}

func TestProberContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var withProbe runner.Reconciler = &probingReconciler{reasons: map[string]string{"option_eod": "broker dial failed"}}
	if reason, probed := probeOf(ctx, withProbe, "option_eod"); !probed || reason != "broker dial failed" {
		t.Fatalf("probe = %q %v, want the reason", reason, probed)
	}
	if reason, probed := probeOf(ctx, withProbe, "fred_series"); !probed || reason != "" {
		t.Fatalf("passing probe = %q %v, want empty reason", reason, probed)
	}

	var without runner.Reconciler = &fakeReconciler{}
	if _, probed := probeOf(ctx, without, "option_eod"); probed {
		t.Fatal("a Reconciler without Probe must show no probe")
	}
}

// fakeConsole implements ForceBackfiller and ConsoleTurner.
type fakeConsole struct {
	gotOpt  runner.BackfillOptions
	gotHold runner.SlotHold
	took    bool
}

func (f *fakeConsole) BackfillWith(_ context.Context, day contracts.Day, _ []string, opt runner.BackfillOptions) (runner.BackfillResult, error) {
	f.gotOpt = opt
	return runner.BackfillResult{
		Day:     day,
		Refused: runner.RefusedNoKey,
		Keys:    []runner.KeyRun{{Group: runner.GroupNotReached}},
	}, nil
}

func (f *fakeConsole) ConsoleTurn(_ context.Context, hold runner.SlotHold) (bool, error) {
	f.gotHold = hold
	return f.took, nil
}

// forceOne is a caller of ForceBackfiller.
func forceOne(b runner.ForceBackfiller, job string, hold runner.SlotHold) (runner.BackfillResult, error) {
	return b.BackfillWith(context.Background(), "2026-11-06", []string{"DGS10"}, runner.BackfillOptions{Force: true, Job: job, Hold: hold})
}

// turn is a caller of ConsoleTurner.
func turn(c runner.ConsoleTurner, hold runner.SlotHold) (bool, error) {
	return c.ConsoleTurn(context.Background(), hold)
}

func TestForceBackfillerCarriesOptionsAndKeys(t *testing.T) {
	t.Parallel()
	f := &fakeConsole{}
	hold := func(time.Duration) (string, time.Time, bool) { return "j", time.Time{}, true }
	res, err := forceOne(f, "j", hold)
	if err != nil {
		t.Fatal(err)
	}
	if !f.gotOpt.Force || f.gotOpt.Job != "j" || f.gotOpt.Hold == nil {
		t.Fatalf("options = %+v", f.gotOpt)
	}
	if res.Refused != "no_key" || len(res.Keys) != 1 || res.Keys[0].Group != runner.GroupNotReached {
		t.Fatalf("result = %+v", res)
	}
}

func TestConsoleTurnerPassesSlotHold(t *testing.T) {
	t.Parallel()
	f := &fakeConsole{took: true}
	var hold runner.SlotHold = func(b time.Duration) (string, time.Time, bool) { return "", time.Time{}, b > 0 }
	took, err := turn(f, hold)
	if err != nil || !took {
		t.Fatalf("took = %v, err = %v", took, err)
	}
	if _, _, held := f.gotHold(time.Minute); !held {
		t.Fatal("hold not passed through")
	}
}

type fakeLocks struct{ asked []string }

func (f *fakeLocks) TryLock(id string) (func(), bool) {
	f.asked = append(f.asked, id)
	return func() {}, true
}

type fakeDependent struct{}

func (fakeDependent) After() []string { return []string{"fred_daily"} }

// fakeClaimer implements ConsoleClaimer and ConsoleTurner at once.
type fakeClaimer struct{ ran int }

func (f *fakeClaimer) ClaimConsole(context.Context) ([]runner.ConsoleRun, error) {
	return []runner.ConsoleRun{func(context.Context) { f.ran++ }}, nil
}

func (f *fakeClaimer) ConsoleTurn(context.Context, runner.SlotHold) (bool, error) { return false, nil }

func TestJobLocksContract(t *testing.T) {
	t.Parallel()
	f := &fakeLocks{}
	var l runner.JobLocks = f
	unlock, ok := l.TryLock("j")
	if !ok || unlock == nil || len(f.asked) != 1 || f.asked[0] != "j" {
		t.Fatalf("TryLock = %v %v", ok, f.asked)
	}
}

func TestDependentContract(t *testing.T) {
	t.Parallel()
	var d runner.Dependent = fakeDependent{}
	if got := d.After(); len(got) != 1 || got[0] != "fred_daily" {
		t.Fatalf("After = %v", got)
	}
}

func TestConsoleClaimerAndTurnerOnOneType(t *testing.T) {
	t.Parallel()
	f := &fakeClaimer{}
	var c runner.ConsoleClaimer = f
	var turner runner.ConsoleTurner = f
	runs, err := c.ClaimConsole(context.Background())
	if err != nil || len(runs) != 1 {
		t.Fatalf("ClaimConsole = %v %v", runs, err)
	}
	runs[0](context.Background())
	if f.ran != 1 {
		t.Fatalf("ran = %d", f.ran)
	}
	if took, err := turner.ConsoleTurn(context.Background(), nil); took || err != nil {
		t.Fatalf("ConsoleTurn = %v %v", took, err)
	}
}
