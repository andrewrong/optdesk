package optionstruct

import (
	"cmp"
	"math/big"
	"slices"

	"github.com/andrewrong/optdesk/internal/compute"
)

// The kinds of a structure (Decision 4).
const (
	kindPutSpread  = "put_spread"
	kindCallSpread = "call_spread"
	kindCovered    = "covered_call"
	kindNakedCall  = "naked_call"
	kindShortPut   = "short_put"
	kindLongPut    = "long_put"
	kindLongCall   = "long_call"
	kindOther      = "other"
)

// leg is an open position with its numbers parsed. qty is signed: negative is short.
type leg struct {
	p                        compute.LedgerPosition
	strike, qty, mult, price *big.Rat
}

func (l leg) short() bool { return l.qty.Sign() < 0 }

// unit is one recognised structure: its kind, its legs ordered by strike then position id
// and, for a covered call, the cost basis per share of the cycle that holds the shares.
type unit struct {
	kind  string
	legs  []leg
	basis *big.Rat
}

// recognise groups the open positions of the input into units. A position is never split;
// a position whose numbers do not parse is an `other` unit of its own.
func recognise(in compute.StructureInput) []unit {
	groups := map[string][]leg{}
	var keys []string
	var out []unit
	for _, p := range in.Positions {
		if p.Status != "open" {
			continue
		}
		l, ok := parseLeg(p)
		if !ok {
			out = append(out, unit{kind: kindOther, legs: []leg{l}})
			continue
		}
		k := p.Underlying + "|" + p.Currency
		if _, seen := groups[k]; !seen {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], l)
	}
	for _, k := range keys {
		out = append(out, recogniseGroup(groups[k], in.Cycles)...)
	}
	for i := range out {
		sortLegs(out[i].legs)
	}
	return out
}

// parseLeg parses a position; on failure the numbers are zero and ok is false.
func parseLeg(p compute.LedgerPosition) (l leg, ok bool) {
	l = leg{p: p}
	var good [4]bool
	l.strike, good[0] = parseDec(p.Strike)
	l.qty, good[1] = parseDec(p.Quantity)
	l.mult, good[2] = parseDec(p.Multiplier)
	l.price, good[3] = parseDec(p.OpenPrice)
	for _, r := range []**big.Rat{&l.strike, &l.qty, &l.mult, &l.price} {
		if *r == nil {
			*r = new(big.Rat)
		}
	}
	ok = good[0] && good[1] && good[2] && good[3] && l.qty.Sign() != 0 &&
		(p.Right == "P" || p.Right == "C")
	return l, ok
}

func sortLegs(ls []leg) {
	slices.SortStableFunc(ls, func(a, b leg) int {
		if c := a.strike.Cmp(b.strike); c != 0 {
			return c
		}
		return cmp.Compare(a.p.PositionID, b.p.PositionID)
	})
}

// recogniseGroup works on the legs of one underlying and currency.
func recogniseGroup(legs []leg, cycles []compute.LedgerCycle) []unit {
	slices.SortStableFunc(legs, func(a, b leg) int { return cmp.Compare(a.p.PositionID, b.p.PositionID) })
	used := make([]bool, len(legs))
	var out []unit
	// (a) a sold and a bought piece of the same right and expiry with the same open quantity
	// pair; credit pairs first, so a debit pair never takes the leg of a credit pair.
	for _, credit := range []bool{true, false} {
		for i, s := range legs {
			if used[i] || !s.short() {
				continue
			}
			for j, b := range legs {
				if used[j] || b.short() || !pairable(s, b) || isCredit(s, b) != credit {
					continue
				}
				used[i], used[j] = true, true
				out = append(out, pairUnit(s, b, credit))
				break
			}
		}
	}
	var rest []leg
	for i, l := range legs {
		if !used[i] {
			rest = append(rest, l)
		}
	}
	// A diagonal, a calendar, a straddle, a collar or a pair that no longer matches is other.
	tangled, singles := entangled(rest)
	for _, comp := range tangled {
		out = append(out, unit{kind: kindOther, legs: comp})
	}
	return append(out, classifySingles(singles, cycles)...)
}

func pairable(s, b leg) bool {
	return s.p.Right == b.p.Right && s.p.Expiry == b.p.Expiry &&
		new(big.Rat).Abs(s.qty).Cmp(b.qty) == 0
}

// isCredit tells whether the sold strike is above the bought strike for puts, below for calls.
func isCredit(s, b leg) bool {
	if s.p.Right == "P" {
		return s.strike.Cmp(b.strike) > 0
	}
	return s.strike.Cmp(b.strike) < 0
}

func pairUnit(s, b leg, credit bool) unit {
	switch {
	case !credit:
		return unit{kind: kindOther, legs: []leg{s, b}}
	case s.p.Right == "P":
		return unit{kind: kindPutSpread, legs: []leg{s, b}}
	}
	return unit{kind: kindCallSpread, legs: []leg{s, b}}
}

// related tells whether two leftover legs form something that is not a single: opposite sides
// of one right, or the two rights of one expiry.
func related(a, b leg) bool {
	if a.p.Right == b.p.Right {
		return a.short() != b.short()
	}
	return a.p.Expiry == b.p.Expiry
}

// entangled splits legs into components of related legs (two or more legs: other) and singles.
func entangled(legs []leg) (components [][]leg, singles []leg) {
	parent := make([]int, len(legs))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range legs {
		for j := i + 1; j < len(legs); j++ {
			if related(legs[i], legs[j]) {
				parent[find(i)] = find(j)
			}
		}
	}
	members := map[int][]leg{}
	var roots []int
	for i, l := range legs {
		r := find(i)
		if _, seen := members[r]; !seen {
			roots = append(roots, r)
		}
		members[r] = append(members[r], l)
	}
	for _, r := range roots {
		if m := members[r]; len(m) > 1 {
			components = append(components, m)
		} else {
			singles = append(singles, m[0])
		}
	}
	return components, singles
}

// classifySingles names the single legs: a sold call is covered while the shares of the cycle
// last (first in, first out among the calls), else naked; the rest by side and right.
func classifySingles(singles []leg, cycles []compute.LedgerCycle) []unit {
	var out []unit
	var calls []leg
	for _, l := range singles {
		switch {
		case l.p.Right == "C" && l.short():
			calls = append(calls, l)
		case l.p.Right == "P" && l.short():
			out = append(out, unit{kind: kindShortPut, legs: []leg{l}})
		case l.p.Right == "P":
			out = append(out, unit{kind: kindLongPut, legs: []leg{l}})
		default:
			out = append(out, unit{kind: kindLongCall, legs: []leg{l}})
		}
	}
	slices.SortStableFunc(calls, func(a, b leg) int {
		if c := cmp.Compare(a.p.OpenedOn, b.p.OpenedOn); c != 0 {
			return c
		}
		return cmp.Compare(a.p.PositionID, b.p.PositionID)
	})
	shares, basis := cycleShares(calls, cycles)
	for _, c := range calls {
		need := mul(new(big.Rat).Abs(c.qty), c.mult)
		if shares.Cmp(need) >= 0 {
			shares = sub(shares, need)
			out = append(out, unit{kind: kindCovered, legs: []leg{c}, basis: basis})
		} else {
			out = append(out, unit{kind: kindNakedCall, legs: []leg{c}})
		}
	}
	return out
}

// cycleShares is the shares held by the cycles of the underlying of the calls, and the cost
// basis per share of the first cycle that holds any (nil when its text is not a number).
func cycleShares(calls []leg, cycles []compute.LedgerCycle) (*big.Rat, *big.Rat) {
	total := new(big.Rat)
	var basis *big.Rat
	if len(calls) == 0 {
		return total, nil
	}
	for _, c := range cycles {
		if c.Underlying != calls[0].p.Underlying || c.Currency != calls[0].p.Currency {
			continue
		}
		n, ok := parseDec(c.Shares)
		if !ok || n.Sign() <= 0 {
			continue
		}
		if basis == nil {
			basis, _ = parseDec(c.CostBasisPerShare)
		}
		total = add(total, n)
	}
	return total, basis
}
