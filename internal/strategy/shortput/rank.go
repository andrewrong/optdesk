package shortput

import (
	"math/big"
	"sort"
	"strconv"

	"github.com/andrewrong/optdesk/internal/strategy"
)

// Rank sets Rank of the ok rows: annualized yield descending (exact), then DTE, strike and instrument id
// ascending, the order of rankRun in the job. It returns the ok rows in that order, then the others in their input
// order with Rank 0. An ok row whose yield or strike is not a number is not ranked. The input is not changed.
func (s *shortPut) Rank(in []strategy.Evaluation) []strategy.Evaluation {
	type keyed struct {
		e      strategy.Evaluation
		yield  *big.Rat
		strike *big.Rat
		dte    int
	}
	var ok []keyed
	var rest []strategy.Evaluation
	for _, e := range in {
		e.Rank = 0
		k := keyed{e: e}
		valid := e.Status == "ok" && len(e.Legs) == 1
		if valid {
			var y, st bool
			k.yield, y = parseDecimal(e.Facts["annualized_yield"])
			k.strike, st = parseDecimal(e.Legs[0].Strike)
			d, err := strconv.Atoi(e.Facts["dte"])
			k.dte = d
			valid = y && st && err == nil
		}
		if valid {
			ok = append(ok, k)
		} else {
			rest = append(rest, e)
		}
	}
	sort.SliceStable(ok, func(a, b int) bool {
		x, y := ok[a], ok[b]
		if c := x.yield.Cmp(y.yield); c != 0 {
			return c > 0
		}
		if x.dte != y.dte {
			return x.dte < y.dte
		}
		if c := x.strike.Cmp(y.strike); c != 0 {
			return c < 0
		}
		return idLess(x.e.Legs[0].InstrumentID, y.e.Legs[0].InstrumentID)
	})
	out := make([]strategy.Evaluation, 0, len(in))
	for i, k := range ok {
		k.e.Rank = i + 1
		out = append(out, k.e)
	}
	return append(out, rest...)
}

// idLess compares instrument ids as numbers (the job's ids are integers), else as text.
func idLess(a, b string) bool {
	x, ea := strconv.Atoi(a)
	y, eb := strconv.Atoi(b)
	if ea == nil && eb == nil {
		return x < y
	}
	return a < b
}

// parseDecimal parses plain decimal text (optional sign, digits, one point) exactly.
func parseDecimal(s string) (*big.Rat, bool) {
	if s == "" {
		return nil, false
	}
	for i, c := range s {
		if (c == '-' || c == '+') && i == 0 {
			continue
		}
		if (c < '0' || c > '9') && c != '.' {
			return nil, false
		}
	}
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}
