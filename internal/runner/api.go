// Package runner reconciles expected keys against present keys.
package runner

import (
	"context"

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
