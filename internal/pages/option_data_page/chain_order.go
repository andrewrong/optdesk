package option_data_page

import (
	"cmp"
	"slices"
)

// sortChainRows lists the chain by expiry ascending (a null expiry last), then right (call before
// put, a null right last), then strike ascending by its figure value (a gap strike last). The
// sort is stable, so rows equal on the three keys keep their input (open-interest) order.
func sortChainRows(rows []chainRow) {
	slices.SortStableFunc(rows, func(a, b chainRow) int {
		if c := cmpNullLast(a.Expiry, b.Expiry, cmp.Compare[string]); c != 0 {
			return c
		}
		if c := cmpNullLast(a.Right, b.Right, func(x, y string) int { return cmp.Compare(rightRank(x), rightRank(y)) }); c != 0 {
			return c
		}
		return cmpStrike(a.Strike, b.Strike)
	})
}

func rightRank(r string) int {
	if r == "call" {
		return 0
	}
	return 1
}

func cmpNullLast(a, b *string, f func(x, y string) int) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}
	return f(*a, *b)
}

func cmpStrike(a, b any) int {
	fa, aok := a.(figure)
	fb, bok := b.(figure)
	switch {
	case !aok && !bok:
		return 0
	case !aok:
		return 1
	case !bok:
		return -1
	}
	return cmp.Compare(fa.Value, fb.Value)
}
