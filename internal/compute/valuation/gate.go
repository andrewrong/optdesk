package valuation

import (
	"time"

	"github.com/shopspring/decimal"
)

// The refusals of the return at a price, named as in the system this ports (ADR 0023).
const (
	refusalTooFewYears    = "TOO_FEW_YEARS"
	refusalNotConsecutive = "NOT_CONSECUTIVE"
	refusalStaleFiling    = "STALE_FILING"
	refusalMissingFigure  = "MISSING_FIGURE"
	refusalShareJump      = "SHARE_JUMP"
	refusalBaseNotPos     = "BASE_NOT_POSITIVE"
)

// retYear is one fiscal year as the return at a price reads it; a nil figure is unknown.
type retYear struct {
	end                                                  time.Time
	revenue, ocf, capex, dep, sbc, shares, plant, equity *decimal.Decimal
}

// known returns the values when none of them is nil.
func known(vs ...*decimal.Decimal) ([]decimal.Decimal, bool) {
	out := make([]decimal.Decimal, len(vs))
	for i, v := range vs {
		if v == nil {
			return nil, false
		}
		out[i] = *v
	}
	return out, true
}

func sumOf(vs []decimal.Decimal) decimal.Decimal {
	total := decimal.Zero
	for _, v := range vs {
		total = add(total, v)
	}
	return total
}

func quo(a, b decimal.Decimal) decimal.Decimal {
	q, _ := div(a, b)
	return q
}

func daysBetween(a, b time.Time) int { return int(b.Sub(a).Hours() / 24) }

// strict is the cash flow after capital spending and stock compensation of a year.
func (y retYear) strict() (decimal.Decimal, bool) {
	v, ok := known(y.ocf, y.capex, y.sbc)
	if !ok {
		return decimal.Zero, false
	}
	return sub(sub(v[0], v[1]), v[2]), true
}

// earnings is the cash flow after depreciation and stock compensation of a year.
func (y retYear) earnings() (decimal.Decimal, bool) {
	v, ok := known(y.ocf, y.dep, y.sbc)
	if !ok {
		return decimal.Zero, false
	}
	return sub(sub(v[0], v[1]), v[2]), true
}

// refusalOf gives the reason the inputs are refused, or "": too few years, years that are
// not consecutive, a latest year older than the stale limit, a missing figure, a jump of the
// share count.
func refusalOf(used []retYear, sharesNow decimal.Decimal, priceDate time.Time) string {
	p := retParams
	if len(used) < p.recordYears {
		return refusalTooFewYears
	}
	for i := 1; i < len(used); i++ {
		if d := daysBetween(used[i-1].end, used[i].end); d < 350 || d > 380 {
			return refusalNotConsecutive
		}
	}
	if daysBetween(used[len(used)-1].end, priceDate) > p.staleDays {
		return refusalStaleFiling
	}
	shares := make([]decimal.Decimal, len(used))
	for i, y := range used {
		s, ok := known(y.shares, y.revenue)
		if !ok || s[0].Sign() <= 0 || s[1].Sign() <= 0 {
			return refusalMissingFigure
		}
		shares[i] = s[0]
	}
	for _, y := range used[len(used)-p.marginYears:] {
		if _, ok := y.strict(); !ok {
			return refusalMissingFigure
		}
	}
	shares = append(shares, sharesNow)
	for i := 1; i < len(shares); i++ {
		if sub(quo(shares[i], shares[i-1]), decOne).Abs().GreaterThan(p.shareJump) {
			return refusalShareJump
		}
	}
	return ""
}

// rOf is the reinvestment return R: the median over the latest rYears of a year's earnings
// over the previous year's equity, at most rMax. ok is false when a figure is unknown or
// an equity is not above zero.
func rOf(used []retYear) (decimal.Decimal, bool) {
	p := retParams
	var ratios []decimal.Decimal
	for i := len(used) - p.rYears; i < len(used); i++ {
		e, ok := used[i].earnings()
		eq := used[i-1].equity
		if !ok || eq == nil || eq.Sign() <= 0 {
			return decimal.Zero, false
		}
		ratios = append(ratios, quo(e, *eq))
	}
	return decimal.Min(median(ratios), p.rMax), true
}

// plantFact is revenue over year-end gross plant over the latest rYears: the median and
// the latest, unrounded.
type plantFact struct{ usual, latest decimal.Decimal }

func plantOf(used []retYear, revenue []decimal.Decimal) (plantFact, bool) {
	p := retParams
	ptrs := make([]*decimal.Decimal, 0, p.rYears)
	for _, y := range used[len(used)-p.rYears:] {
		ptrs = append(ptrs, y.plant)
	}
	plants, ok := known(ptrs...)
	if !ok {
		return plantFact{}, false
	}
	ratios := make([]decimal.Decimal, len(plants))
	for i, pl := range plants {
		if pl.Sign() <= 0 {
			return plantFact{}, false
		}
		ratios[i] = quo(revenue[len(revenue)-p.rYears+i], pl)
	}
	return plantFact{usual: median(ratios), latest: ratios[len(ratios)-1]}, true
}

// Omissions that the spending gate gives.
const (
	omitGateUnknown  = "GATE_UNKNOWN"
	omitGateSpending = "GATE_SPENDING"
)

// gateOf is the spending gate: the first reason that applies and whether plant is reported
// in none of the latest years. The earnings lines are left out when a figure of the gate is
// unknown, or when the capital spending of those years is not above gateRatio times their
// depreciation (spending that only keeps the plant does not earn growth). The plant fact
// itself omits nothing: it is a mark.
func gateOf(used []retYear, revenue []decimal.Decimal) (omission string, plantSkipped bool) {
	p := retParams
	latest := used[len(used)-p.rYears:]
	var spend, dep []*decimal.Decimal
	plantSkipped = true
	for _, y := range latest {
		spend, dep = append(spend, y.capex), append(dep, y.dep)
		plantSkipped = plantSkipped && y.plant == nil
	}
	s, okS := known(spend...)
	d, okD := known(dep...)
	_, plantOK := plantOf(used, revenue)
	if !okS || !okD || (!plantSkipped && !plantOK) {
		return omitGateUnknown, plantSkipped
	}
	if !sumOf(s).GreaterThan(mul(p.gateRatio, sumOf(d))) {
		return omitGateSpending, plantSkipped
	}
	return "", plantSkipped
}
