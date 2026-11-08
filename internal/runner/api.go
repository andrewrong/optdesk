// Package runner reconciles expected keys against present keys.
package runner

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// Reconciler runs the registered jobs for a day and reconciles their expected keys.
type Reconciler interface {
	Reconcile(ctx context.Context, day contracts.Day) error
}

// NewFunc is the constructor of the reconciler (internal/runner/reconcile.New).
type NewFunc func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job) Reconciler

// WindowRule turns a session into the window of a dataset: the window opens
// OpenOffset after the session open and closes CloseOffset after the session
// close, both instants taken from the calendar. A non-session day has no window.
type WindowRule struct {
	// CalendarID names the calendar; empty means the default calendar, XNYS.
	CalendarID  string
	OpenOffset  time.Duration
	CloseOffset time.Duration
}

// NewWithRuleFunc is the constructor of the reconciler with an explicit window rule
// (internal/runner/reconcile.NewWithRule).
type NewWithRuleFunc func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, WindowRule) Reconciler

// Backfiller fills one past day of the plan the runner stored for it. It runs the
// keys of the day that are not yet present, for the datasets given (nil means every
// key of the day).
//
// A refusal is data: the day is refused when BackfillResult.Refused is set, the error
// is nil and every other field is empty. An error is kept for failures (lease, ledger,
// unknown dataset, key without a job). A backfill never moves the cursor, and never
// writes a plan or a window. A key that is already present is not run: it is listed
// in AlreadyPresent.
type Backfiller interface {
	Backfill(ctx context.Context, day contracts.Day, datasets []string) (BackfillResult, error)
}

// BackfillResult is what Backfill did for a day.
type BackfillResult struct {
	Day contracts.Day
	// Refused is empty when the day was not refused; see BackfillRefusal.
	Refused BackfillRefusal
	// AlreadyPresent keys were present and were not run.
	AlreadyPresent []contracts.ExpectedKey
	// Filled keys are now present (they were missing or no_value).
	Filled []contracts.ExpectedKey
	// Absent keys got no value or an empty answer after the window close and are
	// stored no_value.
	Absent []contracts.ExpectedKey
	// Missing keys have no usable answer and are left unmarked (a job error).
	Missing []contracts.ExpectedKey
}

// BackfillRefusal names why a day was refused before any write. The empty value
// means the day was not refused. A refusal comes with a nil error, the cursor is not
// moved and nothing is run.
type BackfillRefusal string

const (
	// RefusedAheadOfCursor: the cursor is empty or the day is after the cursor day.
	RefusedAheadOfCursor BackfillRefusal = "ahead_of_cursor"
	// RefusedNoPlan: the day is at or before the cursor but its stored plan is empty.
	RefusedNoPlan BackfillRefusal = "no_plan"
)

// NewBackfillerWithRuleFunc is the constructor of the backfiller with an explicit
// window rule (internal/runner/reconcile.NewBackfillerWithRule).
type NewBackfillerWithRuleFunc func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, WindowRule) Backfiller

// Windowed is the optional method of a job: the window rule of every dataset the job plans.
// The values are plain so that a job package need not import runner. The window opens
// openOffset after the session open and closes closeOffset after the session close; an empty
// calendarID means the calendar of the reconciler. A job without this method gets the rule
// the reconciler was built with.
type Windowed interface {
	Window() (calendarID string, openOffset, closeOffset time.Duration)
}

// Slot is the plain schedule of one job with the job id: the values of
// Scheduled.Schedule, which the scheduler turns into start instants.
type Slot struct {
	JobID      string
	CalendarID string
	Anchor     string
	Zone       string
	Offset     time.Duration
}

// Scheduled is the optional method of a job: its schedule as plain values.
type Scheduled interface {
	Schedule() (calendarID, anchor, zone string, offset time.Duration)
}

// Bounded is the optional method of a job: the longest time one run of a key may take.
type Bounded interface {
	RunBound() time.Duration
}

// Postcondition is the optional method of a job: whether the rows of a key exist now.
// RunKey calls it after a run that was not cut and marks the key present only on true
// with a nil error. Standard-library types only, so a job package need not import runner.
type Postcondition interface {
	Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)
}

// RestartSensitive is the optional method of a job: true when a restart between its
// slot start and its window close must be avoided.
type RestartSensitive interface {
	RestartSensitive() bool
}

// KeyRunner is the one path that runs a job for one key. attempt makes the
// deterministic Run.ID, closeAt is the window close, reason is "late" or empty, and
// the run's context ends at deadline.
type KeyRunner interface {
	RunKey(ctx context.Context, job contracts.Job, day contracts.Day, key contracts.ExpectedKey, attempt int, closeAt, deadline time.Time, reason string) (store.Run, error) //nolint:lll // the issue fixes the full signature on one line
}

// Scheduler starts the jobs at their slots until ctx ends. A library may implement
// it. LastTick is the instant of the last loop turn, or of construction before the
// first turn; FenceHeld is false once the instance fence was lost. Both are the
// health surface that cmd/ot reads.
type Scheduler interface {
	Run(ctx context.Context) error
	LastTick() time.Time
	FenceHeld() bool
}

// GapSyncer writes and clears the gaps of a day. disabled is the map of left-out jobs
// (job id to reason), the one given to NewSchedulerFunc; nil means none.
type GapSyncer interface {
	Sync(ctx context.Context, day contracts.Day, disabled map[string]string) error
}

// ExecIdentityFunc returns the identity of the running executable (size, mtime and
// inode as one string).
type ExecIdentityFunc func() (string, error)

// NewSchedulerFunc is the constructor of the scheduler (internal/runner/sched.New). gaps nil means no-op.
type NewSchedulerFunc func(fence store.Fence, sched store.SchedulerLedger, ops store.OpsLedger, cal clock.Calendar, clk clockwork.Clock, rec Reconciler, gaps GapSyncer, jobs []contracts.Job, disabled map[string]string, exe ExecIdentityFunc) Scheduler //nolint:lll // the issue fixes the full signature on one line

// NewKeyRunnerFunc is the constructor of the key runner (internal/runner/reconcile.NewKeyRunner).
type NewKeyRunnerFunc func(ops store.OpsLedger, sched store.SchedulerLedger, clk clockwork.Clock) KeyRunner

// NewReconcilerFunc is the constructor of the reconciler and backfiller pair (internal/runner/reconcile.NewReconciler).
type NewReconcilerFunc func(ops store.OpsLedger, sched store.SchedulerLedger, cal clock.Calendar, clk clockwork.Clock, jobs []contracts.Job, rule WindowRule, kr KeyRunner) (Reconciler, Backfiller) //nolint:lll // the issue fixes the full signature on one line
