// Package optionfig forms the two figures of a short-put candidate: the ITM probability
// and the excess yield over cash. It implements optionfigapi.OptionFigures. Pure: no I/O,
// no clock, no store, jobs, notify or os. Design:
// docs/design/option-candidates-figures.md (Decisions 2, 3 and 4).
//
// option.candidates.itm_probability, version 1:
//
//	p  = N(-d2), N(x) = 0.5 * erfc(-x / sqrt(2))
//	d2 = (ln(S/K) + (r - sigma^2/2) * T) / (sigma * sqrt(T))
//
// S is the stored spot, K the stored strike, sigma the stored contract IV (fraction),
// r the cash rate (percent / 100, continuous, no dividend) and T = DTE / 365. The float
// p is taken exactly (big.Rat.SetFloat64) and rounded half up once: the text is the
// percent to one place, the key is hundredths of a percent. It is not delta.
//
// option.candidates.excess_yield, version 1:
//
//	excess = annual_yield - rate / 100, exact decimal, rounded half up to 6 places
//
// The text is the size as a percent to one place, half up on the magnitude of the
// 6-place value; ExcessAbove is excess >= 0; the key is the signed value in hundredths
// of a percent, rounded the same way. A figure with no usable input is a gap with a
// sentence naming the input; its value fields are empty, never 0%.
package optionfig

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
)

const (
	gapMissing = "missing_input"
	gapStale   = "stale_input"
	// daysInYear is the year length of optioncand/put.go.
	daysInYear = 365.0
	dayLayout  = "2006-01-02"
)

type figures struct{}

// New constructs the implementation.
var New optionfigapi.NewOptionFiguresFunc = func() optionfigapi.OptionFigures { return figures{} }

// Put gives the two figures of one row.
func (figures) Put(in optionfigapi.PutFigureInput) optionfigapi.PutFigures {
	var out optionfigapi.PutFigures
	rate, gap, why := rateOf(in)

	if gap != "" {
		out.ItmGap, out.ItmWhy = gap, "ITM probability: "+why
		out.ExcessGap, out.ExcessWhy = gap, "excess yield: "+why
	}
	if out.ItmGap == "" {
		out.ItmPercent, out.ItmKey, out.ItmWhy = itm(in, rate)
		if out.ItmPercent == "" {
			out.ItmGap = gapMissing
		}
	}
	if out.ExcessGap == "" {
		out.ExcessPercent, out.ExcessAbove, out.ExcessKey, out.ExcessWhy = excess(in, rate)
		if out.ExcessPercent == "" {
			out.ExcessGap = gapMissing
		}
	}
	return out
}

// rateOf parses the rate and checks its age; gap is empty when the rate is usable.
func rateOf(in optionfigapi.PutFigureInput) (rate *big.Rat, gap, why string) {
	if strings.TrimSpace(in.Rate) == "" {
		return nil, gapMissing, "no cash rate"
	}
	rate = decimal(in.Rate)
	if rate == nil {
		return nil, gapMissing, "cash rate is not a number"
	}
	chain, errC := time.Parse(dayLayout, string(in.ChainDay))
	day, errR := time.Parse(dayLayout, string(in.RateDay))
	if errC != nil || errR != nil {
		return nil, gapMissing, "cash rate day is not stored"
	}
	age := int(chain.Sub(day).Hours() / 24)
	if age > in.MaxRateAgeDays {
		return nil, gapStale, fmt.Sprintf("cash rate is %d days old", age)
	}
	return rate, "", ""
}

// decimal parses plain decimal text; nil when it is not one.
func decimal(s string) *big.Rat {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "/eEpP_ ") {
		return nil
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil
	}
	return r
}

// positive parses decimal text that must be above zero, as a float.
func positive(s string) (float64, bool) {
	r := decimal(s)
	if r == nil || r.Sign() <= 0 {
		return 0, false
	}
	f, _ := r.Float64()
	if math.IsInf(f, 0) || f <= 0 {
		return 0, false
	}
	return f, true
}

// itm returns the percent text and key; the text is empty and why names the input
// when the figure cannot be formed.
func itm(in optionfigapi.PutFigureInput, rate *big.Rat) (percent, key, why string) {
	spot, ok := positive(in.Spot)
	if !ok {
		return "", "", "ITM probability: spot not stored"
	}
	strike, ok := positive(in.Strike)
	if !ok {
		return "", "", "ITM probability: strike not stored"
	}
	sigma, ok := positive(in.IV)
	if !ok {
		return "", "", "ITM probability: IV not stored"
	}
	if in.DTE < 1 {
		return "", "", "ITM probability: DTE below 1"
	}
	r, _ := rate.Float64()
	r /= 100
	t := float64(in.DTE) / daysInYear
	d2 := (math.Log(spot/strike) + (r-sigma*sigma/2)*t) / (sigma * math.Sqrt(t))
	p := 0.5 * math.Erfc(d2/math.Sqrt2) // N(-d2)
	exact := new(big.Rat).SetFloat64(p)
	if exact == nil {
		return "", "", "ITM probability: not computable from the inputs"
	}
	tenths := roundHalfUp(new(big.Rat).Mul(exact, big.NewRat(1000, 1)))
	hundredths := roundHalfUp(new(big.Rat).Mul(exact, big.NewRat(10000, 1)))
	return tenthsText(tenths), hundredths.String(), ""
}

// excess returns the percent text, direction and signed key; the text is empty and why
// names the input when the figure cannot be formed.
func excess(in optionfigapi.PutFigureInput, rate *big.Rat) (percent string, above bool, key, why string) {
	y := decimal(in.AnnualYield)
	if y == nil {
		return "", false, "", "excess yield: annual yield not stored"
	}
	e := new(big.Rat).Sub(y, new(big.Rat).Quo(rate, big.NewRat(100, 1)))
	e = roundMagnitude(e, 1000000)
	above = e.Sign() >= 0
	mag := new(big.Rat).Abs(e)
	tenths := roundHalfUp(new(big.Rat).Mul(mag, big.NewRat(1000, 1)))
	hund := roundHalfUp(new(big.Rat).Mul(mag, big.NewRat(10000, 1)))
	if !above && hund.Sign() != 0 {
		hund.Neg(hund)
	}
	return tenthsText(tenths), above, hund.String(), ""
}

// roundHalfUp rounds a rational to the nearest integer, ties up.
func roundHalfUp(r *big.Rat) *big.Int {
	half := new(big.Rat).Add(r, big.NewRat(1, 2))
	// Denominators are positive, so Euclidean division is floor.
	return new(big.Int).Div(half.Num(), half.Denom())
}

// roundMagnitude rounds r to 1/scale, ties away from zero.
func roundMagnitude(r *big.Rat, scale int64) *big.Rat {
	n := roundHalfUp(new(big.Rat).Mul(new(big.Rat).Abs(r), big.NewRat(scale, 1)))
	if r.Sign() < 0 {
		n.Neg(n)
	}
	return new(big.Rat).SetFrac(n, big.NewInt(scale))
}

// tenthsText formats tenths of a percent as "27.5%".
func tenthsText(tenths *big.Int) string {
	q, m := new(big.Int).DivMod(tenths, big.NewInt(10), new(big.Int))
	return q.String() + "." + strconv.FormatInt(m.Int64(), 10) + "%"
}
