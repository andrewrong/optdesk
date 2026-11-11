package optionresult

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// months groups the rows by the month of ClosedOn (YYYY-MM), oldest first, with the
// running sum. A month is Over when it ends before the month of asOf; with no asOf every
// month is over. The Premium of a month is the sum of the opening premiums of its rows (all
// four endings, no commissions).
func months(rows []row, asOf contracts.Day) []compute.ResultMonth {
	sums := map[string]*big.Rat{}
	prem := map[string]*big.Rat{}
	for _, r := range rows {
		m := string(r.pos.ClosedOn[:7])
		if sums[m] == nil {
			sums[m] = new(big.Rat)
			prem[m] = new(big.Rat)
		}
		prem[m].Add(prem[m], r.premium)
		sums[m].Add(sums[m], r.realized)
	}
	keys := make([]string, 0, len(sums))
	for m := range sums {
		keys = append(keys, m)
	}
	sort.Strings(keys)
	run := new(big.Rat)
	out := make([]compute.ResultMonth, 0, len(keys))
	for _, m := range keys {
		run.Add(run, sums[m])
		over := len(asOf) < 7 || m < string(asOf[:7])
		out = append(out, compute.ResultMonth{Month: m, Realized: fixed(sums[m], 4), Premium: fixed(prem[m], 4), Running: fixed(run, 4), Over: over})
	}
	return out
}
