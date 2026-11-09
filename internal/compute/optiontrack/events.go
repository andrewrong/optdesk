package optiontrack

import (
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const kindGuard = "held_call_guard"

// eventFields are the dates the page shows for one underlying (Decision 7).
type eventFields struct {
	nextEarnings, nextExDate contracts.Day
	exAmount                 string
	found                    bool
}

// newestEvent reads the rows of the underlying that carry the newest Day; each field is
// the first non-empty value among those rows.
func newestEvent(events []trackapi.TrackEvent, underlying string) eventFields {
	var newest contracts.Day
	for _, e := range events {
		if e.Underlying == underlying && e.Day > newest {
			newest = e.Day
		}
	}
	var out eventFields
	for _, e := range events {
		if e.Underlying != underlying || e.Day != newest {
			continue
		}
		out.found = true
		if out.nextEarnings == "" {
			out.nextEarnings = e.NextEarnings
		}
		if out.nextExDate == "" {
			out.nextExDate, out.exAmount = e.NextExDate, e.ExAmount
		}
	}
	return out
}

// inLife is true when day lies between asOf and expiry, inclusive.
func inLife(day, asOf, expiry contracts.Day) bool {
	return day != "" && day >= asOf && day <= expiry
}

// guardRow is the newest held_call_guard row of a call's terms.
func guardRow(events []trackapi.TrackEvent, underlying string, expiry contracts.Day, strike string) (trackapi.TrackEvent, bool) {
	var best trackapi.TrackEvent
	found := false
	for _, e := range events {
		if e.Kind != kindGuard || e.Underlying != underlying || e.Expiry != expiry || !sameStrike(e.Strike, strike) {
			continue
		}
		if r := side(e.Right); r != "" && r != "C" {
			continue
		}
		if !found || e.Day > best.Day {
			best, found = e, true
		}
	}
	return best, found
}
