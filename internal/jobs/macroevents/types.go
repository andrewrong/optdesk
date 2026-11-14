// Package macroevents is the job that stores the scheduled releases and meetings of the macro page in
// obs.macro_event: FRED release dates over net/http and a hand-kept embedded file. Design:
// docs/design/macro-events.md#Decisions.
package macroevents

import (
	"context"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// ReleaseSource returns the release dates of one FRED release.
type ReleaseSource interface {
	// Dates returns the dates of releaseID from `from` on, ascending; a date before from is dropped. Errors are
	// contracts.TypedError and never hold the key or the URL.
	Dates(ctx context.Context, releaseID string, from contracts.Day) ([]contracts.Day, error)
}

// NewMacroEventsFunc is the constructor of the macro_events job (internal/jobs/macroevents.New).
type NewMacroEventsFunc func(ReleaseSource, macroevent.Writer, clock.Clock) (contracts.Job, error)
