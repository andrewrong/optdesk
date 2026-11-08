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
	case runner.RefusedAheadOfCursor:
		return "run-once first"
	case runner.RefusedNoPlan:
		return "no plan"
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
		{"ahead of cursor", runner.RefusedAheadOfCursor, "ahead_of_cursor", "run-once first"},
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
