// Package techstate reads the closes of NDX and SPX and answers LEADING, IN LINE, LAGGING or LAGGING HARD from
// the 1-month and 3-month gap. Pure: no I/O, no clock, no float; all arithmetic is exact (math/big).
package techstate

import (
	"math/big"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	leading     = "LEADING"
	inLine      = "IN LINE"
	lagging     = "LAGGING"
	laggingHard = "LAGGING HARD"
)

var rankOf = map[string]int{leading: 1, inLine: 0, lagging: -1, laggingHard: -2}

// TrialParams is the one source of the trial constants (design market-state-tech Decision 6).
func TrialParams() blockapi.TechParams {
	return blockapi.TechParams{Band: "1.00", Hard: "5.00", Short: 21, Long: 63, Fold: 10, MaxAgeDays: 5}
}

// Classify is a blockapi.TechFunc.
var _ blockapi.TechFunc = Classify

// pair holds the two closes of one common day.
type pair struct{ ndx, spx *big.Rat }

// Classify decides the tech state of c0, the newest day both series have a close on. The state is decided on exact
// values; the texts are rounded for evidence only. The published state folds the raw states of the newest
// Params.Fold days that have Long+1 closes behind them, oldest first: it starts with the first raw state and takes a
// raw state only when it equals the raw state of the day before.
func Classify(in blockapi.TechIn) blockapi.TechResult {
	p := in.Params
	band, ok1 := decimal(p.Band)
	hard, ok2 := decimal(p.Hard)
	if !ok1 || !ok2 || p.Short < 1 || p.Long <= p.Short || p.Fold < 1 {
		return blockapi.TechResult{Reason: compute.BadInput}
	}
	days, closes, reason := common(in.NDX, in.SPX)
	if reason != "" || len(days) == 0 {
		if reason == "" {
			reason = compute.NoInput
		}
		return blockapi.TechResult{Reason: reason}
	}
	res := blockapi.TechResult{DataDay: days[0]}
	c0, err1 := time.Parse("2006-01-02", string(days[0]))
	asOf, err2 := time.Parse("2006-01-02", string(in.Day))
	if err1 != nil || err2 != nil {
		res.Reason = compute.BadInput
		return res
	}
	if int(asOf.Sub(c0).Hours()/24) > p.MaxAgeDays {
		res.Reason = compute.StaleInput
		return res
	}
	n := len(days) - p.Long // days that have Long closes behind them
	if n < 1 {
		res.Reason = compute.ShortHistory
		return res
	}
	k := min(p.Fold, n)
	raws := make([]string, k)
	var g1, g3 *big.Rat
	for i := range raws {
		a, b := gaps(closes, i, p)
		raws[i] = rawState(a, b, band, hard)
		if i == 0 {
			g1, g3 = a, b
		}
	}
	state := raws[k-1]
	for i := k - 2; i >= 0; i-- {
		if raws[i] == raws[i+1] {
			state = raws[i]
		}
	}
	res.State, res.Raw, res.Rank, res.RawDays = state, raws[0], rankOf[state], k
	res.NDX1m, res.SPX1m = ratText(ret(closes, 0, p.Short, true)), ratText(ret(closes, 0, p.Short, false))
	res.NDX3m, res.SPX3m = ratText(ret(closes, 0, p.Long, true)), ratText(ret(closes, 0, p.Long, false))
	res.Gap1m, res.Gap3m = ratText(g1), ratText(g3)
	return res
}

// rawState is the raw state of one day from its two gaps (percentage points).
func rawState(g1, g3, band, hard *big.Rat) string {
	nb, nh := new(big.Rat).Neg(band), new(big.Rat).Neg(hard)
	switch {
	case g1.Cmp(band) >= 0 && g3.Cmp(band) >= 0:
		return leading
	case g1.Cmp(nb) <= 0 && g3.Cmp(nh) <= 0:
		return laggingHard
	case g1.Cmp(nb) <= 0 && g3.Cmp(nb) <= 0:
		return lagging
	}
	return inLine
}

// ret is (c_i / c_{i+n} - 1) * 100 of NDX or SPX, exact.
func ret(c []pair, i, n int, ndx bool) *big.Rat {
	a, b := c[i].spx, c[i+n].spx
	if ndx {
		a, b = c[i].ndx, c[i+n].ndx
	}
	r := new(big.Rat).Quo(a, b)
	r.Sub(r, big.NewRat(1, 1))
	return r.Mul(r, big.NewRat(100, 1))
}

// gaps are the 1-month and 3-month gaps of the day i sessions back, NDX minus SPX.
func gaps(c []pair, i int, p blockapi.TechParams) (*big.Rat, *big.Rat) {
	g := func(n int) *big.Rat { return new(big.Rat).Sub(ret(c, i, n, true), ret(c, i, n, false)) }
	return g(p.Short), g(p.Long)
}

// common returns the days both series have a close on, newest first, with their parsed closes. Any close of a
// series that is not a positive decimal gives bad_input; either series without a close gives no_input.
func common(a, b blockapi.CloseIn) ([]contracts.Day, []pair, compute.Reason) {
	if len(a.Closes) == 0 || len(b.Closes) == 0 {
		return nil, nil, compute.NoInput
	}
	if len(a.Closes) != len(a.Days) || len(b.Closes) != len(b.Days) {
		return nil, nil, compute.BadInput
	}
	spx := make(map[contracts.Day]*big.Rat, len(b.Closes))
	for i, d := range b.Days {
		v, ok := decimal(b.Closes[i])
		if !ok || v.Sign() <= 0 {
			return nil, nil, compute.BadInput
		}
		if _, dup := spx[d]; !dup {
			spx[d] = v
		}
	}
	var days []contracts.Day
	var out []pair
	for i, d := range a.Days {
		v, ok := decimal(a.Closes[i])
		if !ok || v.Sign() <= 0 {
			return nil, nil, compute.BadInput
		}
		if s, both := spx[d]; both {
			days, out = append(days, d), append(out, pair{v, s})
			delete(spx, d)
		}
	}
	return days, out, ""
}

// decimal parses plain decimal text (digits with at most one point, optional leading minus); big.Rat would also
// accept fractions and exponents, which a close never is.
func decimal(s string) (*big.Rat, bool) {
	digits, points := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			digits++
		case c == '.':
			points++
		case c == '-' && i == 0:
		default:
			return nil, false
		}
	}
	if digits == 0 || points > 1 {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}

// ratText is x rounded half-even to 2 places, as signed decimal text ("-3.12"; a value that rounds to zero is
// "0.00").
func ratText(x *big.Rat) string {
	h := new(big.Rat).Mul(x, big.NewRat(100, 1))
	q, r := new(big.Int).QuoRem(h.Num(), h.Denom(), new(big.Int))
	// QuoRem truncates toward zero; compare twice the remainder with the denominator.
	twice := new(big.Int).Abs(r)
	twice.Lsh(twice, 1)
	cmp := twice.Cmp(h.Denom())
	if cmp > 0 || (cmp == 0 && q.Bit(0) == 1) {
		if h.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	neg := q.Sign() < 0
	q.Abs(q)
	s := q.String()
	for len(s) < 3 {
		s = "0" + s
	}
	s = s[:len(s)-2] + "." + s[len(s)-2:]
	if neg {
		s = "-" + s
	}
	return s
}
