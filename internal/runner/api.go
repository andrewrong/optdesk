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
