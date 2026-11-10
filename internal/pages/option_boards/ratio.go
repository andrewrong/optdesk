package option_boards

import (
	"math/big"
	"strconv"
)

// ratioResult is a percentage computed from stored decimal texts. When OK is false, Missing names the input
// that is absent, zero or unreadable ("spot", "ask", "bid", "mid", "strike") and the other fields are unset.
type ratioResult struct {
	OK      bool
	Text    string // percent rounded half away from zero to one place, "−8.2%"
	Value   int64  // the percent times 100 rounded half away from zero, the sort key
	Missing string
}

func missingRatio(name string) ratioResult { return ratioResult{Missing: name} }

// ratOf reads a stored decimal text as an exact rational; text that is not a plain decimal is refused (big's
// own reader would take "1e-3" and "1/2").
func ratOf(text string) (*big.Rat, bool) {
	if !decimalPattern.MatchString(text) {
		return nil, false
	}
	return new(big.Rat).SetString(text)
}

// positiveRat is ratOf for a text that must be above zero.
func positiveRat(text string) (*big.Rat, bool) {
	r, ok := ratOf(text)
	if !ok || r.Sign() <= 0 {
		return nil, false
	}
	return r, true
}

// strikePct is the strike over the stored spot minus 1, as a percent.
func strikePct(strike string, spot *string) ratioResult {
	if spot == nil {
		return missingRatio("spot")
	}
	sp, ok := positiveRat(*spot)
	if !ok {
		return missingRatio("spot")
	}
	st, ok := ratOf(strike)
	if !ok {
		return missingRatio("strike")
	}
	q := new(big.Rat).Quo(st, sp)
	q.Sub(q, big.NewRat(1, 1))
	return percentOf(q)
}

// spreadPct is (ask minus bid) over mid, mid being (bid plus ask) over 2, as a percent.
func spreadPct(bid string, ask *string) ratioResult {
	if ask == nil {
		return missingRatio("ask")
	}
	a, ok := positiveRat(*ask)
	if !ok {
		return missingRatio("ask")
	}
	b, ok := positiveRat(bid)
	if !ok {
		return missingRatio("bid")
	}
	mid := new(big.Rat).Add(a, b)
	mid.Quo(mid, big.NewRat(2, 1))
	if mid.Sign() <= 0 {
		return missingRatio("mid")
	}
	q := new(big.Rat).Sub(a, b)
	q.Quo(q, mid)
	return percentOf(q)
}

// percentOf turns a ratio into its percent text and sort key.
func percentOf(q *big.Rat) ratioResult {
	pct := new(big.Rat).Mul(q, big.NewRat(100, 1))
	tenths := roundHalfAway(new(big.Rat).Mul(pct, big.NewRat(10, 1)))
	hundredths := roundHalfAway(new(big.Rat).Mul(pct, big.NewRat(100, 1)))
	return ratioResult{OK: true, Text: tenthsText(tenths), Value: hundredths.Int64()}
}

// roundHalfAway rounds to an integer, halves away from zero.
func roundHalfAway(x *big.Rat) *big.Int {
	num := new(big.Int).Abs(x.Num())
	den := x.Denom() // positive
	// floor((2*num + den) / (2*den))
	num.Mul(num, big.NewInt(2)).Add(num, den)
	num.Quo(num, new(big.Int).Mul(den, big.NewInt(2)))
	if x.Sign() < 0 {
		num.Neg(num)
	}
	return num
}

// tenthsText is a count of tenths of a percent as text with one place: -82 is "−8.2%".
func tenthsText(tenths *big.Int) string {
	sign := ""
	if tenths.Sign() < 0 {
		sign = minusSign
	}
	n := new(big.Int).Abs(tenths)
	whole, frac := new(big.Int).QuoRem(n, big.NewInt(10), new(big.Int))
	return sign + whole.String() + "." + strconv.FormatInt(frac.Int64(), 10) + "%"
}
