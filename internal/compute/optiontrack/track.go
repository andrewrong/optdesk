// Package optiontrack implements trackapi.OptionTracker, the option tracking rule of
// docs/design/option-tracking.md (Decisions 2 to 9). It is pure: exact math/big.Rat
// arithmetic, no float, no clock, no I/O.
package optiontrack

import (
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
)

type tracker struct{}

// New returns the option tracker.
func New() trackapi.OptionTracker { return tracker{} }

// Track builds the open short rows, the recently closed rows, the long-leg count and the
// reconciliation with the account.
func (tracker) Track(in trackapi.TrackInput, p trackapi.TrackParams) trackapi.TrackResult {
	var res trackapi.TrackResult
	var openShorts []compute.LedgerPosition
	for _, pos := range in.Positions {
		q, ok := dec(pos.Quantity)
		if !ok {
			continue
		}
		switch {
		case pos.Status == "open" && q.Sign() < 0:
			openShorts = append(openShorts, pos)
			res.Open = append(res.Open, openRow(in, p, pos))
		case pos.Status == "open" && q.Sign() > 0:
			res.LongLegsLeftOut++
		case pos.Status != "open" && q.Sign() < 0 && closedWithin(in, p, pos):
			res.Recent = append(res.Recent, recentRow(pos))
		}
	}
	sort.SliceStable(res.Open, func(i, j int) bool {
		a, b := res.Open[i], res.Open[j]
		if a.Expiry != b.Expiry {
			return a.Expiry < b.Expiry
		}
		if a.Underlying != b.Underlying {
			return a.Underlying < b.Underlying
		}
		if c := cmpStrike(a.Strike, b.Strike); c != 0 {
			return c < 0
		}
		return a.PositionID < b.PositionID
	})
	sort.SliceStable(res.Recent, func(i, j int) bool {
		a, b := res.Recent[i], res.Recent[j]
		if a.ClosedOn != b.ClosedOn {
			return a.ClosedOn > b.ClosedOn
		}
		return a.PositionID < b.PositionID
	})
	if in.HeldKnown {
		res.Recon = reconcile(openShorts, in.Held)
	}
	return res
}

// closedWithin is true when the closing day is at most RecentDays before the as-of day.
func closedWithin(in trackapi.TrackInput, p trackapi.TrackParams, pos compute.LedgerPosition) bool {
	n, ok := daysBetween(pos.ClosedOn, in.AsOf)
	return ok && n >= 0 && n <= p.RecentDays
}
