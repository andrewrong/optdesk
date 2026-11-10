package barsrefresh

import (
	"fmt"
	"math/big"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func parseRat(s string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("barsrefresh: %q is not a decimal", s)
	}
	return r, nil
}

// differs reports whether pulled differs from stored by more than tol, relative to stored
// (Decision 4): |pulled - stored| > tol * |stored|. A stored zero differs from any other value.
func differs(stored, pulled, tol *big.Rat) bool {
	diff := new(big.Rat).Sub(pulled, stored)
	diff.Abs(diff)
	limit := new(big.Rat).Mul(tol, new(big.Rat).Abs(stored))
	return diff.Cmp(limit) > 0
}

// restated reports whether a pulled day that is already stored differs from the stored winner close by
// more than tol. Days not stored never count. A close that is not a decimal is an error.
func restated(stored []contracts.PriceBar, pulled []broker.Bar, tol *big.Rat) (bool, error) {
	byDay := make(map[string]string, len(stored))
	for _, s := range stored {
		byDay[s.SessionDay.Format(dayLayout)] = s.Close
	}
	out := false
	for _, p := range pulled {
		sc, ok := byDay[p.Day]
		if !ok {
			continue
		}
		sr, err := parseRat(sc)
		if err != nil {
			return false, err
		}
		pr, err := parseRat(p.Close)
		if err != nil {
			return false, err
		}
		if differs(sr, pr, tol) {
			out = true
		}
	}
	return out, nil
}

// maxEpoch is the highest AdjustEpoch of rows, 0 for none.
func maxEpoch(rows []contracts.PriceBar) int {
	e := 0
	for _, r := range rows {
		e = max(e, r.AdjustEpoch)
	}
	return e
}
