// Package optionevidence is the candidate evidence rule: the expected move, the break-even below the
// buy price, the bid against a price made from realised volatility, the skew and the soft labels, as a
// pure function of one row. Decimal text only: no float64, no clock, no I/O.
package optionevidence

import (
	"fmt"
	"math/big"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
)

// Reasons an empty figure carries.
const (
	whyNoIV        = "no_iv"
	whyNoForecast  = "no_forecast"
	whyNoRate      = "no_rate"
	whyNoFairPrice = "no_fair_price"
	whyEarnings    = "earnings_before_expiry"
	whyNoATM       = "no_atm"
	whyNoBuyPrice  = "no_buy_price"
	whyZeroDTE     = "zero_dte"
)

const (
	sqrtPrec    = 128 // bits of the square root
	sqrtPlaces  = 12  // decimal places it is formatted to
	ratioPlaces = 4   // places of a share or percent
	defaultMult = "100"
)

var hundred = decimal.NewFromInt(100)

type evaluator struct {
	v optionevidenceapi.Valuer
	p optionevidenceapi.Params
}

var _ optionevidenceapi.Evaluator = (*evaluator)(nil)

// DefaultParams has the trial values: delta band 0.15 to 0.30, 365 days in a year.
func DefaultParams() optionevidenceapi.Params {
	return optionevidenceapi.Params{DeltaLow: "0.15", DeltaHigh: "0.30", DaysInYear: 365}
}

// New returns the Evaluator; v prices the fair value and may be nil (then no_fair_price).
func New(v optionevidenceapi.Valuer, p optionevidenceapi.Params) optionevidenceapi.Evaluator {
	if p.DaysInYear <= 0 {
		p.DaysInYear = 365
	}
	return &evaluator{v: v, p: p}
}

func parse(s string) (decimal.Decimal, bool) {
	if s == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(s)
	return d, err == nil
}

// Evaluate computes the figures of one row; labels never change a figure.
func (e *evaluator) Evaluate(r optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
	spot, okSpot := parse(r.Spot)
	strike, okStrike := parse(r.Strike)
	bid, okBid := parse(r.Bid)
	if !okSpot || !okStrike || !okBid || !spot.IsPositive() || !strike.IsPositive() || bid.IsNegative() || r.DTE < 0 {
		return optionevidenceapi.Evidence{}, fmt.Errorf("evaluate: %w", optionevidenceapi.ErrBadInput)
	}
	ev := optionevidenceapi.Evidence{ForecastErr: r.ForecastErr}
	breakEven := strike.Sub(bid)
	ev.BreakEven = breakEven.String()

	e.belowBuy(&ev, r, breakEven)
	e.move(&ev, r, spot, breakEven)
	e.edge(&ev, r, spot, strike, bid)
	skew(&ev, r)
	fillPercents(&ev, r)
	ev.Labels = e.labels(r, ev)
	return ev, nil
}

func (e *evaluator) belowBuy(ev *optionevidenceapi.Evidence, r optionevidenceapi.Row, breakEven decimal.Decimal) {
	buy, ok := parse(r.BuyPrice)
	if !ok || !buy.IsPositive() {
		ev.BelowBuyWhy = whyNoBuyPrice
		return
	}
	below := buy.Sub(breakEven)
	ev.BelowBuy = below.String()
	ev.BelowBuyPct = below.Div(buy).Mul(hundred).StringFixed(ratioPlaces)
}

// years is DTE / days-in-year at 12 places.
func (e *evaluator) years(dte int) decimal.Decimal {
	return decimal.NewFromInt(int64(dte)).DivRound(decimal.NewFromInt(int64(e.p.DaysInYear)), sqrtPlaces)
}

func (e *evaluator) move(ev *optionevidenceapi.Evidence, r optionevidenceapi.Row, spot, breakEven decimal.Decimal) {
	iv, ok := parse(r.IV)
	if !ok || !iv.IsPositive() {
		ev.MoveWhy = whyNoIV
		return
	}
	ratio := new(big.Float).SetPrec(sqrtPrec).SetInt64(int64(r.DTE))
	ratio.Quo(ratio, new(big.Float).SetPrec(sqrtPrec).SetInt64(int64(e.p.DaysInYear)))
	root, err := decimal.NewFromString(ratio.Sqrt(ratio).Text('f', sqrtPlaces))
	if err != nil {
		ev.MoveWhy = whyNoIV
		return
	}
	sigma := spot.Mul(iv).Mul(root)
	if !sigma.IsPositive() { // DTE 0: the move is zero, so no figure of it has a meaning
		ev.MoveWhy = whyZeroDTE
		return
	}
	ev.OneSigma = sigma.StringFixed(ratioPlaces)
	ev.OneSigmaPct = sigma.Div(spot).Mul(hundred).StringFixed(ratioPlaces)
	ev.BreakEvenInSigmas = spot.Sub(breakEven).Div(sigma).StringFixed(ratioPlaces)
}

func (e *evaluator) edge(ev *optionevidenceapi.Evidence, r optionevidenceapi.Row, spot, strike, bid decimal.Decimal) {
	if r.EarningsBeforeExpiry {
		ev.EdgeWhy = whyEarnings
		return
	}
	fc, ok := parse(r.Forecast)
	if !ok || !fc.IsPositive() {
		ev.EdgeWhy = whyNoForecast
		return
	}
	rate, ok := parse(r.RatePercent)
	if !ok {
		ev.EdgeWhy = whyNoRate
		return
	}
	if e.v == nil {
		ev.EdgeWhy = whyNoFairPrice
		return
	}
	fairText, err := e.v.Value("P", spot.String(), strike.String(), e.years(r.DTE).String(),
		rate.Div(hundred).String(), fc.String())
	fair, ok := parse(fairText)
	if err != nil || !ok {
		ev.EdgeWhy = whyNoFairPrice
		return
	}
	ev.Fair = fair.String()
	edge := bid.Sub(fair)
	ev.EdgePerShare = edge.String()
	mult, ok := parse(r.Multiplier)
	if !ok {
		mult, _ = parse(defaultMult)
	}
	ev.EdgePerContract = edge.Mul(mult).StringFixed(2)
}

func skew(ev *optionevidenceapi.Evidence, r optionevidenceapi.Row) {
	iv, ok := parse(r.IV)
	if !ok {
		ev.SkewWhy = whyNoIV
		return
	}
	atm, ok := parse(r.ATMIV)
	if !ok {
		ev.SkewWhy = whyNoATM
		return
	}
	ev.Skew = iv.Sub(atm).String()
}
