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
