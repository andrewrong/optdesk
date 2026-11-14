package option_boards

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// maxChainExpiries keeps the chain inside the 20 columns of the schema (one is the strike).
const maxChainExpiries = 19

// buildChain is the strike by expiry card of the first card's company (Decision 7); nil when there is no card
// or the company has no row.
func buildChain(rows, cards []store.Candidate, names displayNames, qs quoteSource) *chain {
	if len(cards) == 0 {
		return nil
	}
	first := cards[0]
	var mine []store.Candidate
	for _, r := range rows {
		if r.Issuer == first.Issuer {
			mine = append(mine, r)
		}
	}
	if len(mine) == 0 {
		return nil
	}

	expiries := sortedUnique(mine, func(r store.Candidate) string { return string(r.Expiry) })
	if len(expiries) > maxChainExpiries {
		expiries = expiries[:maxChainExpiries]
	}
	strikes := sortedUnique(mine, func(r store.Candidate) string { return r.Strike })
	sort.SliceStable(strikes, func(i, j int) bool {
		c, err := compareDecimal(strikes[i], strikes[j])
		if err != nil {
			return strikes[i] < strikes[j]
		}
		return c < 0
	})

	cols := []column{{ID: "strike", Label: "Strike", Align: "left"}}
	for _, e := range expiries {
		label, err := monthDay(contracts.Day(e))
		if err != nil {
			label = e
		}
		cols = append(cols, column{ID: "e" + e, Label: label, Align: "right"})
	}
	bids := map[pairKey]string{}
	for _, r := range mine {
		k := pairKey{r.Strike, string(r.Expiry)}
		if _, ok := bids[k]; !ok {
			bids[k] = r.BidEod
		}
	}
	out := make([]row, 0, len(strikes))
	for _, s := range strikes {
		cells := []cellAt{{Column: "strike", Cell: textCell{Kind: "text", Text: s}}}
		for _, e := range expiries {
			cells = append(cells, cellAt{Column: "e" + e, Cell: bidCell(bids, pairKey{s, e})})
		}
		out = append(out, row{ID: "k" + strings.ReplaceAll(s, ".", "_"), Cells: cells, Quote: qs.chainQuote()})
	}

	md, _ := monthDay(first.Expiry)
	name := names.of(first)
	return &chain{
		Title:   "Option chain · " + name,
		Summary: fmt.Sprintf("Selected: %s %s strike $%s Put", name, strings.ReplaceAll(md, minus, "/"), first.Strike),
		Table:   table{Columns: cols, Rows: out},
	}
}

// pairKey is one strike and expiry of the chain.
type pairKey struct{ strike, expiry string }

// bidCell is the stored bid of the pair as UsdAmount, or the gap no_data when no row holds the pair or the bid
// is not a decimal.
func bidCell(bids map[pairKey]string, k pairKey) any {
	bid, ok := bids[k]
	if !ok {
		return mkGap("no_data", gapText)
	}
	n, ok := numberLiteral(bid)
	if !ok {
		return mkGap("no_data", gapText)
	}
	return usdAmount{Value: n, Text: moneyText(bid), Unit: "usd"}
}

// sortedUnique is the distinct keys of the rows, ascending as text (dates sort as text; strikes are re-sorted
// by the caller).
func sortedUnique(rows []store.Candidate, key func(store.Candidate) string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range rows {
		if k := key(r); !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
