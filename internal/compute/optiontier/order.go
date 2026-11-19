package optiontier

import (
	"cmp"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// assemble folds the badges, orders the rows and writes the Result.
func assemble(states []*rowState, p optiontierapi.Params, built bool) optiontierapi.Result {
	res := optiontierapi.Result{Common: fold(states), CashBuilt: built, Params: p}
	res.Verdicts = make([]optiontierapi.Verdict, 0, len(states))
	res.Order = []string{}
	byTier := map[string][]*rowState{}
	for _, s := range states {
		byTier[s.v.Tier] = append(byTier[s.v.Tier], s)
	}
	for _, tier := range []string{tierFits, tierCheck} {
		sortTier(byTier[tier])
	}
	sortGrey(byTier[tierGrey])
	for _, tier := range []string{tierFits, tierCheck, tierGrey} {
		for i, s := range byTier[tier] {
			s.v.Place = i + 1
			res.Order = append(res.Order, s.v.Key)
		}
	}
	for _, s := range states {
		res.Verdicts = append(res.Verdicts, s.v)
	}
	res.Companies = companies(states)
	res.Blocks = buildBlocks(states, p)
	res.Kept = kept(states)
	return res
}

// descText compares two decimal texts for a descending order, empty last: -1 when a goes first.
func descText(a, b string) int {
	da, oka := parse(a)
	db, okb := parse(b)
	switch {
	case oka && okb:
		return db.Cmp(da)
	case oka:
		return -1
	case okb:
		return 1
	}
	return 0
}

func strikeCmp(a, b string) int {
	da, oka := parse(a)
	db, okb := parse(b)
	if oka && okb {
		return da.Cmp(db)
	}
	return 0
}

// sortTier orders by margin descending, yield descending, then DTE, strike and key ascending.
func sortTier(rows []*rowState) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if c := descText(a.v.MarginPct, b.v.MarginPct); c != 0 {
			return c < 0
		}
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

func greyRank(reason string) int {
	for i, g := range greyOrder {
		if g == reason {
			return i
		}
	}
	return len(greyOrder)
}

// sortGrey orders by reason in the order of precedence, then expiry, strike and key.
func sortGrey(rows []*rowState) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if c := cmp.Compare(greyRank(a.v.Grey), greyRank(b.v.Grey)); c != 0 {
			return c < 0
		}
		if c := cmp.Compare(string(a.row.Expiry), string(b.row.Expiry)); c != 0 {
			return c < 0
		}
		if c := strikeCmp(a.row.Strike, b.row.Strike); c != 0 {
			return c < 0
		}
		return a.row.Key < b.row.Key
	})
}

// companies counts the tiers per company in order of first appearance.
func companies(states []*rowState) []optiontierapi.CompanyCount {
	out := []optiontierapi.CompanyCount{}
	idx := map[string]int{}
	for _, s := range states {
		i, ok := idx[s.row.Company]
		if !ok {
			i = len(out)
			idx[s.row.Company] = i
			out = append(out, optiontierapi.CompanyCount{Company: s.row.Company})
		}
		switch s.v.Tier {
		case tierFits:
			out[i].Fits++
		case tierCheck:
			out[i].Check++
		default:
			out[i].Grey++
		}
	}
	return out
}

// kept counts, in a fixed order, each grey reason and then each look-at badge that kept rows out of
// Fits; a row counts in one grey reason and in every look-at badge it carries. Every is N equal to the
// rows read.
func kept(states []*rowState) []optiontierapi.Kept {
	out := []optiontierapi.Kept{}
	add := func(code string, n int) {
		if n > 0 {
			out = append(out, optiontierapi.Kept{Code: code, N: n, Every: n == len(states)})
		}
	}
	for _, g := range greyOrder {
		n := 0
		for _, s := range states {
			if s.v.Grey == g {
				n++
			}
		}
		add(g, n)
	}
	for _, b := range lookOrder {
		n := 0
		for _, s := range states {
			if s.look[b] {
				n++
			}
		}
		add(b, n)
	}
	return out
}
