package optionopen

import "github.com/andrewrong/optdesk/internal/contracts"

// outsideBand is the number of rows the live band dropped at the open (Decision 1 of option-open-band): the rows
// with OutsideBand true. The postcondition counts them apart from the planned items, in the US and the JP pull.
func outsideBand(rows []contracts.OptionOpen) int {
	n := 0
	for _, r := range rows {
		if r.OutsideBand {
			n++
		}
	}
	return n
}

// allOutside is the end of a batch whose every item the live band dropped (planned == 0 once they are counted
// apart): NoValue, the runner's gap and not a failure.
func allOutside(res contracts.JobResult, outside int) contracts.JobResult {
	return withErr(res, newErr(contracts.NoValue, "all outside band (outside_band=%d)", outside))
}

// postcondition ends a run: planned is the not-deferred plan items, of which the outside-band rows are taken apart.
// All of them outside is NoValue; else at least share of the rest, less the rows the broker answered with no quote, must be ok.
func postcondition(res contracts.JobResult, share float64, planned, ok, unread, partial, outside, noQuote, added int) contracts.JobResult {
	planned -= outside
	if planned == 0 && outside > 0 {
		return allOutside(res, outside)
	}
	planned -= noQuote
	if float64(ok) < share*float64(planned) {
		return underShare(res, ok, planned, unread, partial, outside, added, share)
	}
	return res
}
