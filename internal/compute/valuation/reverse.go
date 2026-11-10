package valuation

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
)

// The reverse DCF of Decision 1: ten years of the latest free cash flow growing at the
// unknown rate, a terminal growth of 2% after them, year-end flows, fixed shares, no fade.
const (
	reverseYears      = 10
	solverIterations  = 64
	reasonBadSpot     = "bad_spot"
	reasonBaseNonPos  = "non_positive_base_fcf"
	reasonBelowRange  = "below_solver_range"
	reasonAboveRange  = "above_solver_range"
	primaryRate       = "0.10"
	growthPlaces      = 6
	termGrowthText    = "0.02"
	solverLowText     = "-0.5"
	solverHighText    = "1"
	rateLowText       = "0.08"
	rateHighText      = "0.12"
)

var (
	discountRates  = []string{rateLowText, primaryRate, rateHighText}
	terminalGrowth = decimal.RequireFromString(termGrowthText)
	solverLow      = decimal.RequireFromString(solverLowText)
	solverHigh     = decimal.RequireFromString(solverHighText)
	decTwo         = decimal.NewFromInt(2)
)

// r34 rounds to the 34 significant digits, half-even, that every operation of the old
// system kept.
func r34(x decimal.Decimal) decimal.Decimal {
	q, _ := div(x, decOne)
	return q
}

func add(a, b decimal.Decimal) decimal.Decimal { return r34(a.Add(b)) }
func sub(a, b decimal.Decimal) decimal.Decimal { return r34(a.Sub(b)) }
func mul(a, b decimal.Decimal) decimal.Decimal { return r34(a.Mul(b)) }

// reverseValue is the unrounded value per share of a company whose free cash flow grows at
// growth for ten years, discounted at rate, with a Gordon-growth terminal value.
func reverseValue(base, shares, growth, rate decimal.Decimal) decimal.Decimal {
	f, disc, total := base, decOne, decimal.Zero
	g, d := add(decOne, growth), add(decOne, rate)
	for i := 0; i < reverseYears; i++ {
		f = mul(f, g)
		disc = mul(disc, d)
		q, _ := div(f, disc)
		total = add(total, q)
	}
	num := mul(f, add(decOne, terminalGrowth))
	den := mul(sub(rate, terminalGrowth), disc)
	terminal, _ := div(num, den)
	v, _ := div(add(total, terminal), shares)
	return v
}

// solveGrowth bisects the growth whose value equals price over the solver range. status is
// "" when solved, else below_solver_range or above_solver_range.
func solveGrowth(base, shares, price, rate decimal.Decimal) (growth, status string) {
	lo, hi := solverLow, solverHigh
	if price.LessThan(reverseValue(base, shares, lo, rate)) {
		return "", reasonBelowRange
	}
	if price.GreaterThan(reverseValue(base, shares, hi, rate)) {
		return "", reasonAboveRange
	}
	for i := 0; i < solverIterations; i++ {
		mid, _ := div(add(lo, hi), decTwo)
		if reverseValue(base, shares, mid, rate).LessThan(price) {
			lo = mid
		} else {
			hi = mid
		}
	}
	mid, _ := div(add(lo, hi), decTwo)
	return mid.RoundBank(growthPlaces).StringFixed(growthPlaces), ""
}

// ImpliedGrowth solves, at each discount rate, the growth the spot price implies for the
// latest free cash flow. It implements valuationapi.ImpliedGrowthFunc. Primary is the
// growth at the primary rate (10%); a rate whose growth is outside -0.5 to 1 has an empty
// Growth and a reason "below_solver_range:<rate>" or "above_solver_range:<rate>". State is
// ok when the primary growth is solved; refused inputs, a latest free cash flow at or below
// zero or a spot that is not a positive number give missing_input and no rates.
func ImpliedGrowth(f valuationapi.Filings, spot string) valuationapi.ImpliedResult {
	res := valuationapi.ImpliedResult{State: valuationapi.StateMissingInput}
	in, reasons := readFcfInputs(f)
	if len(reasons) == 0 || len(in.rows) == bandYears {
		if in.rows[0].fcf.Sign() <= 0 {
			reasons = append(reasons, reasonBaseNonPos)
		}
	}
	price, ok := parseValue(spot)
	if !ok || price.Sign() <= 0 {
		reasons = append(reasons, reasonBadSpot)
	}
	if len(reasons) > 0 {
		res.Reasons = reasons
		return res
	}
	for _, rt := range discountRates {
		g, status := solveGrowth(in.rows[0].fcf, in.shares, price, decimal.RequireFromString(rt))
		res.Rates = append(res.Rates, valuationapi.ImpliedRate{Rate: rt, Growth: g})
		if status != "" {
			res.Reasons = append(res.Reasons, status+":"+rt)
		}
		if rt == primaryRate {
			res.Primary = g
		}
	}
	if res.Primary != "" {
		res.State = valuationapi.StateOK
	}
	return res
}

var _ valuationapi.ImpliedGrowthFunc = ImpliedGrowth
