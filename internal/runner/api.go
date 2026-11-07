// Package runner reconciles expected keys against present keys.
package runner

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
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
