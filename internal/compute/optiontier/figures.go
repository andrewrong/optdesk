package optiontier

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// Ids of the figures in Verdict.Why.
const (
	figMargin     = "margin"
	figBelowSpot  = "below_spot"
	figMoves      = "moves"
	figPerContr   = "per_contract"
	figTimeValue  = "time_value"
	figYield      = "yield"
	whyNoIV       = "no_iv"
	whyZeroDTE    = "zero_dte"
	whyNotEvaled  = "not_judged"
	percentPlaces = 4
	yieldScale    = 16
)

var hundred = decimal.NewFromInt(100)

func pct(d decimal.Decimal) string { return d.StringFixed(percentPlaces) }

// writeFigures fills the figures of v. The margin, the money below the buy price and the moves come from
// the evaluator's answer when the row was evaluated; the others are plain decimal arithmetic on the
// row. An empty figure has a Why code; a row that was not evaluated names its grey reason there.
func writeFigures(v *optiontierapi.Verdict, r optiontierapi.Row, n numbers, p optiontierapi.Params,
	ev optionevidenceapi.Evidence, evaluated bool, grey string) {
	why := v.Why
	if evaluated {
		v.BreakEven, v.BelowBuy, v.MarginPct = ev.BreakEven, ev.BelowBuy, ev.BelowBuyPct
		v.Moves = ev.BreakEvenInSigmas
		if v.MarginPct == "" {
			why[figMargin] = orElse(ev.BelowBuyWhy, whyNotEvaled)
		}
		if v.Moves == "" {
			why[figMoves] = orElse(ev.MoveWhy, whyNoIV)
		}
	} else {
		why[figMargin] = orElse(grey, whyNotEvaled)
		why[figMoves] = orElse(grey, whyNotEvaled)
	}
	if n.okStrike && n.okBid {
		be := n.strike.Sub(n.bid)
		if v.BreakEven == "" {
			v.BreakEven = be.String()
		}
		if n.okSpot && n.spot.IsPositive() {
			v.BelowSpotPct = pct(n.spot.Sub(be).Div(n.spot).Mul(hundred))
		}
	}
	if v.BelowSpotPct == "" {
		why[figBelowSpot] = noFigureWhy(n, grey)
	}
	if n.okBid {
		v.PerContract = n.bid.Mul(n.mult).String()
	} else {
		why[figPerContr] = orElse(grey, greyNoBid)
	}
	writeTimeValue(v, r, n, p, grey)
}

// writeTimeValue fills Intrinsic, TimeValue and YieldPct: the yield is of the time value only, on the
// strike, per year.
func writeTimeValue(v *optiontierapi.Verdict, r optiontierapi.Row, n numbers, p optiontierapi.Params, grey string) {
	if !n.okStrike || !n.okSpot {
		v.Why[figTimeValue] = noFigureWhy(n, grey)
		v.Why[figYield] = noFigureWhy(n, grey)
		return
	}
	intrinsic := decimal.Max(decimal.Zero, n.strike.Sub(n.spot))
	v.Intrinsic = intrinsic.String()
	if !n.okBid {
		v.Why[figTimeValue] = orElse(grey, greyNoBid)
		v.Why[figYield] = orElse(grey, greyNoBid)
		return
	}
	tv := n.bid.Sub(intrinsic)
	v.TimeValue = tv.String()
	if r.DTE <= 0 || !n.strike.IsPositive() {
		v.Why[figYield] = whyZeroDTE
		return
	}
	num := tv.Mul(decimal.NewFromInt(int64(p.DaysInYear))).Mul(hundred)
	den := n.strike.Mul(decimal.NewFromInt(int64(r.DTE)))
	v.YieldPct = pct(num.DivRound(den, yieldScale))
}

func noFigureWhy(n numbers, grey string) string {
	switch {
	case grey != "":
		return grey
	case !n.okSpot:
		return greyNoSpot
	}
	return greyBadInput
}

func orElse(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
