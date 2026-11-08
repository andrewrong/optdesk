// Package gaps turns the scheduler's expectations and open gaps into what a page shows.
// Both functions are pure: the caller reads the store and passes values and the time.
package gaps

import (
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// State is the freshness of one dataset on a page.
type State string

// The freshness states.
const (
	Fresh   State = "FRESH"
	Late    State = "LATE"
	Stale   State = "STALE"
	Unknown State = "UNKNOWN"
	Invalid State = "INVALID"
	Closed  State = "CLOSED"
)

// Freshness gives the state of e at now. Order of the rules: Closed is CLOSED; a zero ExpectedDay
// is UNKNOWN (also a holiday, which has no window row in stage 1); an empty DatasetID or a
// PresentDay after ExpectedDay is INVALID; PresentDay at ExpectedDay is FRESH. Otherwise the
// data is missing: before DueBy it is FRESH (not yet due), after DueBy it is LATE, and STALE once
// the scheduler has recorded a Gap for the key.
func Freshness(e store.Expectation, now time.Time) State {
	switch {
	case e.Closed:
		return Closed
	case e.ExpectedDay == "":
		return Unknown
	case e.DatasetID == "", e.PresentDay > e.ExpectedDay:
		return Invalid
	case e.PresentDay == e.ExpectedDay:
		return Fresh
	case !now.After(e.DueBy):
		return Fresh
	case e.Gap != "":
		return Stale
	default:
		return Late
	}
}
