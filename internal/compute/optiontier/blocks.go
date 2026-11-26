package optiontier

import (
	"cmp"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// bandFolds is the order of the band codes; a row failing several checks takes the first it fails, and
// over_cap comes last in the Why list.
var bandFolds = []string{
	optiontierapi.FoldNoOpenInterest, optiontierapi.FoldOTMLow, optiontierapi.FoldOTMHigh,
	optiontierapi.FoldNoDelta, optiontierapi.FoldDeltaOut, optiontierapi.FoldOverCap,
}

// stateRank is the order of the companies with nothing shown.
var stateRank = map[string]int{
	optiontierapi.StateOutBand: 0, optiontierapi.StateAboveBuy: 1, optiontierapi.StateBlocked: 2,
	optiontierapi.StateNoQuote: 3, optiontierapi.StateNoBuyPrice: 4,
}

// bandFold is the fold code of a fit or check row outside the band, "" when it is inside
// (candidates-by-company.md Decision 5).
func bandFold(s *rowState, p optiontierapi.Params) string {
	if s.look[badgeNoOpenInterest] || s.look[badgeLowOpenInterest] {
		return optiontierapi.FoldNoOpenInterest
	}
	spot, okSpot := parse(s.row.Spot)
	strike, okStrike := parse(s.row.Strike)
	minPct, _ := parse(p.BandOTMMinPct)
	maxPct, _ := parse(p.BandOTMMaxPct)
	if okSpot && okStrike {
		// out of the money percent = (spot - strike) / spot * 100, compared without dividing.
		gap := spot.Sub(strike).Mul(decimal.NewFromInt(100))
		if gap.LessThan(minPct.Mul(spot)) {
			return optiontierapi.FoldOTMLow
		}
		if gap.GreaterThan(maxPct.Mul(spot)) {
			return optiontierapi.FoldOTMHigh
		}
	}
	d, ok := parse(s.row.Delta)
	if !ok {
		return optiontierapi.FoldNoDelta
	}
	lo, _ := parse(p.BandDeltaMin)
	hi, _ := parse(p.DeltaMax)
	if abs := d.Abs(); abs.LessThan(lo) || abs.GreaterThan(hi) {
		return optiontierapi.FoldDeltaOut
	}
	return ""
}

// sortByYield orders by YieldPct descending (empty last), then DTE, strike and key ascending.
func sortByYield(rows []*rowState) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if c := descText(a.v.YieldPct, b.v.YieldPct); c != 0 {
			return c < 0
		}
		if c := cmp.Compare(a.row.DTE, b.row.DTE); c != 0 {
			return c < 0
		}
		if c := strikeCmp(a.row.Strike, b.row.Strike); c != 0 {
			return c < 0
		}
		return a.row.Key < b.row.Key
	})
}

// companyState is the total rule of Decision 3, first match wins.
func companyState(rows []*rowState, shown int) string {
	if shown > 0 {
		return optiontierapi.StateSellable
	}
	grey := map[string]bool{}
	for _, s := range rows {
		if s.v.Tier != tierGrey {
			return optiontierapi.StateOutBand
		}
		grey[s.v.Grey] = true
	}
	switch {
	case grey[greyNoBuyPrice]:
		return optiontierapi.StateNoBuyPrice
	case grey[greyNoSpot] || grey[greyNoBid]:
		return optiontierapi.StateNoQuote
	}
	return optiontierapi.StateBlocked
}

// buildBlocks builds one block per company from the row states, in the order of Decision 2; never nil.
func buildBlocks(states []*rowState, p optiontierapi.Params) []optiontierapi.CompanyBlock {
	out := []optiontierapi.CompanyBlock{}
	var names []string
	by := map[string][]*rowState{}
	for _, s := range states {
		if _, ok := by[s.row.Company]; !ok {
			names = append(names, s.row.Company)
		}
		by[s.row.Company] = append(by[s.row.Company], s)
	}
	for _, name := range names {
		out = append(out, companyBlock(name, by[name], p))
	}
	sort.SliceStable(out, func(i, j int) bool { return blockBefore(out[i], out[j]) })
	return out
}

func companyBlock(name string, rows []*rowState, p optiontierapi.Params) optiontierapi.CompanyBlock {
	var inBand, greys, folded []*rowState
	why := map[string]int{}
	for _, s := range rows {
		if s.v.Tier == tierGrey {
			greys = append(greys, s)
			why[s.v.Grey]++
			continue
		}
		if code := bandFold(s, p); code != "" {
			folded = append(folded, s)
			why[code]++
			continue
		}
		inBand = append(inBand, s)
	}
	sortByYield(inBand)
	shown := inBand
	if len(shown) > p.PerCompany {
		shown = inBand[:p.PerCompany]
		over := inBand[p.PerCompany:]
		folded = append(folded, over...)
		why[optiontierapi.FoldOverCap] = len(over)
	}
	b := optiontierapi.CompanyBlock{
		Company: name, State: companyState(rows, len(shown)),
		Shown: []string{}, Folded: []string{}, Why: []optiontierapi.GreyCount{},
	}
	for i, s := range shown {
		if i == 0 {
			b.Best = s.v.YieldPct
		}
		if s.v.Tier == tierFits {
			b.NFit++
		} else {
			b.NCheck++
		}
		b.Shown = append(b.Shown, s.row.Key)
	}
	b.NOpen = len(folded)
	sortByYield(greys)
	sortByYield(folded)
	for _, s := range append(greys, folded...) {
		b.Folded = append(b.Folded, s.row.Key)
	}
	for _, code := range append(append([]string{}, greyOrder...), bandFolds...) {
		if why[code] > 0 {
			b.Why = append(b.Why, optiontierapi.GreyCount{Code: code, N: why[code]})
		}
	}
	return b
}

// blockGroup: 0 shown rows with a best yield, 1 shown rows without yield, 2 nothing shown.
func blockGroup(b optiontierapi.CompanyBlock) int {
	switch {
	case len(b.Shown) == 0:
		return 2
	case b.Best != "":
		return 0
	}
	return 1
}

func blockBefore(a, b optiontierapi.CompanyBlock) bool {
	ga, gb := blockGroup(a), blockGroup(b)
	if ga != gb {
		return ga < gb
	}
	switch ga {
	case 0:
		if c := descText(a.Best, b.Best); c != 0 {
			return c < 0
		}
	case 2:
		if c := cmp.Compare(stateRank[a.State], stateRank[b.State]); c != 0 {
			return c < 0
		}
	}
	return a.Company < b.Company
}
