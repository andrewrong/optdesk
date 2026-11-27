package valuation

import (
	"github.com/shopspring/decimal"
)

// The paths of the return at a price: the growth of each of the ten years, the cash each
// year pays, and the rate at which the cash is worth the cost.

var decZero = decimal.Zero

// powFrac is x^e for x > 0, to the 34 digits of the system this ports.
func powFrac(x, e decimal.Decimal) decimal.Decimal {
	ln, err := x.Ln(60)
	if err != nil {
		return decOne
	}
	v, err := e.Mul(ln).ExpTaylor(50)
	if err != nil {
		return decOne
	}
	return r34(v)
}

// record is the compound yearly rate between the averages of the first and the last n of
// the per-share figures; ok is false when an end-point figure is unknown or an end-point
// sum is not above zero.
func record(perShare []*decimal.Decimal, n int) (decimal.Decimal, bool) {
	first, ok1 := known(perShare[:n]...)
	last, ok2 := known(perShare[len(perShare)-n:]...)
	if !ok1 || !ok2 || sumOf(first).Sign() <= 0 || sumOf(last).Sign() <= 0 {
		return decZero, false
	}
	ratio := quo(sumOf(last), sumOf(first))
	e := quo(decOne, decimal.NewFromInt(int64(len(perShare)-n)))
	return sub(powFrac(ratio, e), decOne), true
}

// earningsRecord is the record over all the years when it is defined. When it is not
// because an end-point figure is unknown and each of the latest shortRecord figures is
// known, it is the record over those years (short). A record that is undefined with every
// end-point figure known stays undefined.
func earningsRecord(perShare []*decimal.Decimal) (rec decimal.Decimal, years int, short, ok bool) {
	p := retParams
	n := p.endPoint
	if r, ok := record(perShare, n); ok {
		return r, p.recordYears, false, true
	}
	ends := append(append([]*decimal.Decimal{}, perShare[:n]...), perShare[len(perShare)-n:]...)
	latest := perShare[len(perShare)-p.shortRecord:]
	_, endsKnown := known(ends...)
	_, latestKnown := known(latest...)
	if !endsKnown && latestKnown {
		if r, ok := record(latest, p.shortEndPoint); ok {
			return r, p.shortRecord, true, true
		}
	}
	return decZero, 0, false, false
}

// fade is start for held years, then a straight line to the terminal growth at the last year.
func fade(start decimal.Decimal, held int) []decimal.Decimal {
	p := retParams
	step := quo(sub(p.terminal, start), decimal.NewFromInt(int64(p.horizon-held)))
	out := make([]decimal.Decimal, p.horizon)
	for t := 1; t <= p.horizon; t++ {
		k := t - held
		if k < 0 {
			k = 0
		}
		out[t-1] = add(start, mul(step, decimal.NewFromInt(int64(k))))
	}
	return out
}

func flat(g decimal.Decimal, n int) []decimal.Decimal {
	out := make([]decimal.Decimal, n)
	for i := range out {
		out[i] = g
	}
	return out
}

// lowPath is the yearly growth of the low path, unrounded: it starts at the lowest of half
// the earnings record, the revenue record and the cap, and falls to the terminal growth.
func lowPath(revenueRec decimal.Decimal, earningsRec *decimal.Decimal) []decimal.Decimal {
	p := retParams
	lowest := revenueRec
	if earningsRec != nil {
		lowest = decimal.Min(revenueRec, *earningsRec)
	}
	var start decimal.Decimal
	switch {
	case lowest.Sign() <= 0:
		start = decimal.Max(lowest, p.growthFloor)
	case earningsRec == nil:
		start = decimal.Min(revenueRec, p.growthCap)
	default:
		start = decimal.Min(mul(p.recordShare, *earningsRec), revenueRec, p.growthCap)
	}
	if start.LessThanOrEqual(p.terminal) {
		return flat(start, p.horizon)
	}
	return fade(start, 1)
}

// analystIn is the analyst input of the core: the compound yearly growth over years
// forecast years, or offScale when the forecast is not of the scale of the filings.
type analystIn struct {
	growth   decimal.Decimal
	years    int
	offScale bool
}

// analystFromRevenue anchors an analyst revenue forecast (the first forecast year first) to
// the latest reported revenue: the first year over the reported must lie between anchorMin
// and anchorMax, else the forecast is off scale. Growth is the compound yearly rate from
// the reported revenue to the last forecast year.
func analystFromRevenue(revenue []decimal.Decimal, reported decimal.Decimal) analystIn {
	p := retParams
	first := quo(revenue[0], reported)
	if first.LessThan(p.anchorMin) || first.GreaterThan(p.anchorMax) {
		return analystIn{offScale: true}
	}
	n := len(revenue)
	g := sub(powFrac(quo(revenue[n-1], reported), quo(decOne, decimal.NewFromInt(int64(n)))), decOne)
	return analystIn{growth: g, years: n}
}

// analystPath is the analyst growth for its years and the path after them: at or above
// the terminal growth it falls to the terminal growth at the last year; below it, later
// years have the higher of it and zero.
func analystPath(a analystIn) []decimal.Decimal {
	p := retParams
	if a.growth.GreaterThanOrEqual(p.terminal) {
		return fade(a.growth, a.years)
	}
	return append(flat(a.growth, a.years), flat(decimal.Max(a.growth, decZero), p.horizon-a.years)...)
}

// limited caps each growth at R; no R leaves the path as it is.
func limited(growth []decimal.Decimal, r *decimal.Decimal) []decimal.Decimal {
	if r == nil {
		return growth
	}
	out := make([]decimal.Decimal, len(growth))
	for i, g := range growth {
		out[i] = decimal.Min(g, *r)
	}
	return out
}

// payout is the share of a year's earnings that is paid out: 1 - g/R between 0 and 1, all
// of it without R. Growth is paid for.
func payout(g decimal.Decimal, r *decimal.Decimal) decimal.Decimal {
	if r == nil {
		return decOne
	}
	return decimal.Min(decOne, decimal.Max(decZero, sub(decOne, quo(g, *r))))
}

// cashFlows is the cash per share of each year of the path and of the year after it,
// unrounded. The year after grows at the last year's rate.
func cashFlows(base decimal.Decimal, used []decimal.Decimal, r *decimal.Decimal) []decimal.Decimal {
	earnings := base
	flows := make([]decimal.Decimal, 0, len(used)+1)
	for _, g := range append(append([]decimal.Decimal{}, used...), used[len(used)-1]) {
		earnings = mul(earnings, add(decOne, g))
		flows = append(flows, mul(earnings, payout(g, r)))
	}
	return flows
}

// present is the present value of the yearly amounts and the perpetuity after them, and
// the perpetuity alone.
func present(flows []decimal.Decimal, lastGrowth, rate decimal.Decimal) (total, after decimal.Decimal) {
	disc := decOne
	total = decZero
	for _, f := range flows[:len(flows)-1] {
		disc = mul(disc, add(decOne, rate))
		total = add(total, quo(f, disc))
	}
	after = quo(quo(flows[len(flows)-1], sub(rate, lastGrowth)), disc)
	return add(total, after), after
}

// solved is the rate at one cost; rate and beyond are empty with NO_SOLUTION.
type solved struct {
	rate, beyond *decimal.Decimal
}

var bisectWidth = decimal.New(1, -9)

// solve bisects the rate at which the present value equals the cost; the value falls as
// the rate rises.
func solve(flows []decimal.Decimal, lastGrowth, cost decimal.Decimal) solved {
	p := retParams
	lo, hi := add(lastGrowth, p.rateOffset), p.rateMax
	if lo.GreaterThanOrEqual(hi) {
		return solved{}
	}
	if v, _ := present(flows, lastGrowth, lo); v.LessThan(cost) {
		return solved{}
	}
	if v, _ := present(flows, lastGrowth, hi); v.GreaterThan(cost) {
		return solved{}
	}
	for sub(hi, lo).GreaterThanOrEqual(bisectWidth) {
		mid := quo(add(lo, hi), decTwo)
		if v, _ := present(flows, lastGrowth, mid); v.GreaterThan(cost) {
			lo = mid
		} else {
			hi = mid
		}
	}
	rate := quo(add(lo, hi), decTwo)
	total, after := present(flows, lastGrowth, rate)
	beyond := quo(after, total)
	return solved{rate: &rate, beyond: &beyond}
}
