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
// There is no cursor test: a day with a stored plan is filled whether it is behind or
// ahead of the cursor. A key whose window has not closed is not run and is listed in
// InWindow. The lease is taken under an owner of its own, never the owner of Reconcile,
// so a Reconcile and a Backfill exclude each other.
//
// A refusal is data: the day is refused when BackfillResult.Refused is set, the error
// is nil and every other field is empty. An error is kept for failures (lease, ledger,
// unknown dataset, key without a job). A backfill writes only the missing windows of a
// stored day and the keys its plan lacks, and never moves the cursor. A key that is
// already present is not run: it is listed in AlreadyPresent.
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
	// InWindow keys were missing or no_value but were not run: their window had not
	// closed.
	InWindow []contracts.ExpectedKey
	// Keys is one line per key the call looked at, in the order of the plan.
	Keys []KeyRun
}

// The group of a KeyRun: what the call did with the key.
const (
	// GroupAlreadyPresent: the key was present and a plain backfill did not run it.
	GroupAlreadyPresent = "already_present"
	// GroupNotDue: the key is held, the slot of its job has not come; it was not run
	// and DueAt is that slot instant. A key inside its window that is not held is run.
	GroupNotDue = "not_due"
	// GroupRerun: the key was present and a forced call ran it again.
	GroupRerun = "rerun"
	// GroupRan: the key was missing or no_value and was run.
	GroupRan = "ran"
	// GroupLeaseHeld: another run held the key's lease; it was not run.
	GroupLeaseHeld = "lease_held"
	// GroupNotReached: the call ended (a hold, a cancel or an error) before the key.
	GroupNotReached = "not_reached"
)

// KeyRun is the line of one key in BackfillResult.Keys. Status is the string form of
// store.RunStatus for GroupRan and empty otherwise; Mark is the string form of the
// mark of the key after the call; DueAt is the slot instant of the job, set for
// GroupNotDue.
type KeyRun struct {
	Key                                 contracts.ExpectedKey
	Group                               string
	DueAt                               time.Time
	Status, ErrorCode, ReasonCode, Mark string
	Returned, Usable, Attempt           int
}

// BackfillOptions narrows and forces a backfill.
type BackfillOptions struct {
	// Force runs a key that is already present.
	Force bool
	// Job limits the call to the keys of one job; empty means every job of the
	// listed datasets.
	Job string
	// Hold, when set, is asked before every key after the first; a true answer ends
	// the call and the keys left are listed as GroupNotReached.
	Hold SlotHold
}

// SlotHold tells whether a scheduled start is near. bound is how far ahead to look.
// The first start of an enabled job in (now, now+bound], or an earlier start not yet
// marked covered, answers held true with that job and instant; a schedule that has not
// been read answers held true with an empty job.
type SlotHold func(bound time.Duration) (jobID string, at time.Time, held bool)

// ForceBackfiller is a Backfiller that can force present keys, narrow the call to one
// job and hold before each key. A key refused for want of a key on the day comes back
// as RefusedNoKey with a nil error.
type ForceBackfiller interface {
	BackfillWith(ctx context.Context, day contracts.Day, datasets []string, opt BackfillOptions) (BackfillResult, error)
}

// ConsoleTurner is the optional method of the Reconciler given to NewSchedulerFunc: one
// turn of the console's request queue. It runs on the worker, so it never overlaps a
// Reconcile, and took tells whether a request was handled. A cancelled ctx ends the turn
// interrupted. The turn has no bound of its own; each key has the RunBound of its job.
type ConsoleTurner interface {
	ConsoleTurn(ctx context.Context, hold SlotHold) (took bool, err error)
}

// BackfillRefusal names why a day was refused before any write. The empty value
// means the day was not refused. A refusal comes with a nil error, the cursor is not
// moved and nothing is run.
type BackfillRefusal string

const (
	// RefusedNoPlan: the stored plan of the day is empty.
	RefusedNoPlan BackfillRefusal = "no_plan"
	// RefusedNoKey: a selected dataset has no key on the day.
	RefusedNoKey BackfillRefusal = "no_key"
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

// Prober is the optional interface of the Reconciler given to NewSchedulerFunc. Before
// each Reconcile the ticker calls Probe for every job left out, on the worker goroutine
// of Reconcile, so a probe never overlaps a Reconcile. An empty reason means the inputs
// of the job are there and the job returns to the map of the enabled; a non-empty reason
// is the reason it stays left out. A Reconciler without Probe means no re-probe.
type Prober interface {
	Probe(ctx context.Context, jobID string) (reason string)
}

// GapSyncer writes and clears the gaps of a day. disabled is the map of left-out jobs
// (job id to reason); nil means none. The ticker owns that map and gives Sync a fresh
// copy on every call: Sync may keep or read it without a race.
type GapSyncer interface {
	Sync(ctx context.Context, day contracts.Day, disabled map[string]string) error
}

// KeyGapSyncer writes and clears only the key gaps of one day, and leaves the job-level
// gaps alone. It is a separate interface so that the implementations of GapSyncer keep
// compiling.
type KeyGapSyncer interface {
	SyncKeys(ctx context.Context, day contracts.Day) error
}

// NewKeyGapSyncerFunc is the constructor of the key gap syncer (internal/runner/sched.NewKeyGapSyncer).
type NewKeyGapSyncerFunc func(ops store.OpsLedger, sched store.SchedulerLedger, clk clockwork.Clock) KeyGapSyncer

// ExecIdentityFunc returns the identity of the running executable (size, mtime and
// inode as one string).
type ExecIdentityFunc func() (string, error)

// NewSchedulerFunc is the constructor of the scheduler (internal/runner/sched.New). gaps nil means no-op.
// disabled (job id to reason) is copied at construction: the ticker owns its copy, updates
// it when a probe changes a job, and gives GapSyncer.Sync a fresh copy; the caller's map is not kept.
type NewSchedulerFunc func(fence store.Fence, sched store.SchedulerLedger, ops store.OpsLedger, cal clock.Calendar, clk clockwork.Clock, rec Reconciler, gaps GapSyncer, jobs []contracts.Job, disabled map[string]string, exe ExecIdentityFunc) Scheduler //nolint:lll // the issue fixes the full signature on one line

// NewKeyRunnerFunc is the constructor of the key runner (internal/runner/reconcile.NewKeyRunner).
type NewKeyRunnerFunc func(ops store.OpsLedger, sched store.SchedulerLedger, clk clockwork.Clock) KeyRunner

// NewReconcilerFunc is the constructor of the reconciler and backfiller pair (internal/runner/reconcile.NewReconciler).
type NewReconcilerFunc func(ops store.OpsLedger, sched store.SchedulerLedger, cal clock.Calendar, clk clockwork.Clock, jobs []contracts.Job, rule WindowRule, kr KeyRunner) (Reconciler, Backfiller) //nolint:lll // the issue fixes the full signature on one line

// Dependent is the optional method of a job: After gives the ids of the jobs of the same
// day that the job reads.
type Dependent interface {
	After() []string
}

// JobLocks is the set of per-job locks. TryLock never blocks: ok is false while the job
// is locked, and otherwise unlock releases it.
type JobLocks interface {
	TryLock(jobID string) (unlock func(), ok bool)
}

// ConsoleRun is one claimed console request, run to its end under ctx.
type ConsoleRun func(ctx context.Context)

// ConsoleClaimer is the optional method of the Reconciler given to NewSchedulerFunc: it
// claims the requests of the console queue. ClaimConsole reads the queue, claims requests
// and returns one ConsoleRun per claimed request; it starts no goroutine. The ticker owns
// every goroutine: it calls ClaimConsole on a console goroutine of its own, one call at a
// time, and runs each ConsoleRun in a goroutine of its own under a console context that it
// drains and cancels.
type ConsoleClaimer interface {
	ClaimConsole(ctx context.Context) ([]ConsoleRun, error)
}
