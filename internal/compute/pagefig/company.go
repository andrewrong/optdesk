package pagefig

import (
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// companyStates are the price states a company row can carry, in display order.
var companyStates = []string{"band", "owner override", "awaiting approval", "expired", "unvalued"}

// glance is the figure of the companies glance tiles: one line per state, in the order of
// companyStates, its Amount the count of rows in that state (0 when none). A row with another
// state is not counted.
func glance(rows []blockapi.CompanyState) []blockapi.MoneyLine {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.State]++
	}
	out := make([]blockapi.MoneyLine, 0, len(companyStates))
	for _, s := range companyStates {
		out = append(out, blockapi.MoneyLine{Name: s, Amount: compute.FigureValue{Value: strconv.Itoa(counts[s])}})
	}
	return out
}

// changes is the figure of the companies changes block: a line for each row whose band today
// differs from its band of the earlier day, in input order. A row without a band on either day,
// or with the same band, says nothing.
func changes(rows []blockapi.CompanyState) []blockapi.ChangeLine {
	var out []blockapi.ChangeLine
	for _, r := range rows {
		if r.BandToday == "" || r.BandThen == "" || r.BandToday == r.BandThen {
			continue
		}
		out = append(out, blockapi.ChangeLine{Symbol: r.Symbol, Text: r.BandThen + " → " + r.BandToday})
	}
	return out
}
