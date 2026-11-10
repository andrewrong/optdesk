package valuation

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Return at a price (Decisions 1, 4, 10 and 11), ported in meaning from price_return v1
// (option-trader blob 8d7be25e35c3, ADR 0023): the yearly rate that a bare strike earns
// under four named lines, from discounted cash flows per share with a growing perpetuity
// after ten years, solved for the rate. The result holds rates only: no value per share and
// no verdict word. A growth is paid for: with a reinvestment return R a year that grows by g
// pays out 1 - g/R of its earnings.

// Role names of the two figures of a year that only the return at a price reads.
const (
	roleDepreciation = "depreciation"
	roleGrossPlant   = "gross_plant"
)

// returnParams are the thresholds of price_return v1.
type returnParams struct {
	horizon, recordYears, endPoint, shortRecord, shortEndPoint int
	marginYears, rYears, staleDays                             int
	growthCap, recordShare, terminal, growthFloor              decimal.Decimal
	rMin, rMax, gateRatio, plantDrop                           decimal.Decimal
	anchorMin, anchorMax, shareJump, rateOffset, rateMax       decimal.Decimal
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

var retParams = returnParams{
	horizon: 10, recordYears: 10, endPoint: 3, shortRecord: 5, shortEndPoint: 2,
	marginYears: 5, rYears: 5, staleDays: 457,
	growthCap: dec("0.10"), recordShare: dec("0.5"), terminal: dec("0.02"), growthFloor: dec("-0.05"),
	rMin: dec("0.08"), rMax: dec("0.25"), gateRatio: dec("1.5"), plantDrop: dec("0.20"),
	anchorMin: dec("0.5"), anchorMax: dec("2"), shareJump: dec("0.30"),
	rateOffset: dec("0.0001"), rateMax: decOne,
}

// analystYears is the number of forecast years an Analyst input covers.
const analystYears = 5

// Omissions of a line.
const (
	omitEarningsBaseUnknown   = "EARNINGS_BASE_UNKNOWN"
	omitEarningsBaseNotPos    = "EARNINGS_BASE_NOT_POSITIVE"
	omitEarningsRecordUndef   = "EARNINGS_RECORD_UNDEFINED"
	omitRUnknown              = "R_UNKNOWN"
	omitRBelowMin             = "R_BELOW_MIN"
	omitNoAnalyst             = "NO_ANALYST"
	omitAnalystScale          = "ANALYST_SCALE"
	reasonNoAnalystText       = "no analyst input"
	reasonNoSolution          = "NO_SOLUTION"
	refusalBadInput           = "BAD_INPUT"
	flagEarningsRecordUndef   = "EARNINGS_RECORD_UNDEFINED"
	flagEarningsRecordShort   = "EARNINGS_RECORD_SHORT"
	flagPlantNotReported      = "PLANT_NOT_REPORTED"
	flagGrowthLimitedByR      = "GROWTH_LIMITED_BY_R"
	flagPlantRevenueFell      = "PLANT_REVENUE_FELL"
	flagLatestLevelNotPos     = "LATEST_LEVEL_NOT_POSITIVE"
	statusSolved, statusNoSol = "SOLVED", "NO_SOLUTION"
)

// The shape of a result as the golden files of the old system give it; strings are fixed
// decimals and a pointer is null when the figure is unknown.
type solvedOut struct {
	Cost   string  `json:"cost"`
	Rate   *string `json:"rate"`
	Beyond *string `json:"beyond"`
	Status string  `json:"status"`
}

type scenarioOut struct {
	Key         string      `json:"key"`
	Base        *string     `json:"base"`
	Growth      []string    `json:"growth"`
	R           *string     `json:"r"`
	Omitted     *string     `json:"omitted"`
	Flags       []string    `json:"flags"`
	Solved      []solvedOut `json:"solved"`
	LatestBase  *string     `json:"latest_base"`
	LatestLevel []solvedOut `json:"latest_level"`
}

type returnOut struct {
	State               string   `json:"state"`
	Reason              string   `json:"reason,omitempty"`
	PeriodEnd           string   `json:"period_end"`
	SharesNow           string   `json:"shares_now"`
	RevenueRecord       string   `json:"revenue_record"`
	EarningsRecord      *string  `json:"earnings_record"`
	EarningsRecordYears *int     `json:"earnings_record_years"`
	LowPath             []string `json:"low_path"`
	R                   *string  `json:"r"`
	Flags               []string `json:"flags"`
	Analyst             *struct {
		Growth string `json:"growth"`
		Years  int    `json:"years"`
	} `json:"analyst"`
	Plant *struct {
		Usual  string `json:"usual"`
		Latest string `json:"latest"`
	} `json:"plant"`
	Scenarios []scenarioOut `json:"scenarios"`
}

func fix(d decimal.Decimal, places int32) string { return d.RoundBank(places).StringFixed(places) }

func rateText(d decimal.Decimal) string { return fix(d, 6) }

func opt(d decimal.Decimal, places int32) *string {
	s := fix(d, places)
	return &s
}

func rates(ds []decimal.Decimal) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = rateText(d)
	}
	return out
}

func solvedOf(costs []decimal.Decimal, flows []decimal.Decimal, lastGrowth decimal.Decimal) []solvedOut {
	out := make([]solvedOut, len(costs))
	for i, c := range costs {
		s := solve(flows, lastGrowth, c)
		out[i] = solvedOut{Cost: c.String(), Status: statusNoSol}
		if s.rate != nil {
			out[i].Rate, out[i].Beyond, out[i].Status = opt(*s.rate, 6), opt(*s.beyond, 6), statusSolved
		}
	}
	return out
}

// leftOut is a line that is not formed, with its reason.
func leftOut(key string, base *decimal.Decimal, why string) scenarioOut {
	s := scenarioOut{Key: key, Growth: []string{}, Omitted: &why, Flags: []string{}, Solved: []solvedOut{}}
	if base != nil {
		s.Base = opt(*base, 4)
	}
	return s
}

// scenario forms one line: the rate at each cost on the path, capped at R. With latestBase
// (the strict figure of the latest year per share): above zero and below base it is solved
// beside the base on the same path; at or below zero the line is flagged.
func scenario(key string, base decimal.Decimal, growth []decimal.Decimal, costs []decimal.Decimal, r, latestBase *decimal.Decimal) scenarioOut {
	used := limited(growth, r)
	flags := []string{}
	for i := range used {
		if !used[i].Equal(growth[i]) {
			flags = append(flags, flagGrowthLimitedByR)
			break
		}
	}
	last := used[len(used)-1]
	s := scenarioOut{
		Key: key, Base: opt(base, 4), Growth: rates(used), Omitted: nil,
		Solved: solvedOf(costs, cashFlows(base, used, r), last),
	}
	if r != nil {
		s.R = opt(*r, 6)
	}
	switch {
	case latestBase == nil:
	case latestBase.Sign() <= 0:
		flags = append(flags, flagLatestLevelNotPos)
	case latestBase.LessThan(base):
		s.LatestBase = opt(*latestBase, 4)
		s.LatestLevel = solvedOf(costs, cashFlows(*latestBase, used, r), last)
	}
	s.Flags = flags
	return s
}

// baseOf is the latest revenue times the median ratio of the figure to revenue over the
// latest marginYears, per share now; nil when a figure is unknown.
func baseOf(figures []*decimal.Decimal, revenue []decimal.Decimal, sharesNow decimal.Decimal) *decimal.Decimal {
	p := retParams
	k, ok := known(figures[len(figures)-p.marginYears:]...)
	if !ok {
		return nil
	}
	rev := revenue[len(revenue)-p.marginYears:]
	ratios := make([]decimal.Decimal, len(k))
	for i := range k {
		ratios[i] = quo(k[i], rev[i])
	}
	b := quo(mul(revenue[len(revenue)-1], median(ratios)), sharesNow)
	return &b
}

func decimalPtrs(ds []decimal.Decimal) []*decimal.Decimal {
	out := make([]*decimal.Decimal, len(ds))
	for i := range ds {
		out[i] = &ds[i]
	}
	return out
}

// figuresOf reads the revenue and share count (known after refusalOf) and the strict and
// earnings cash flow of each year.
func figuresOf(used []retYear) (revenue, shares []decimal.Decimal, strict, earn []*decimal.Decimal) {
	n := len(used)
	revenue, shares = make([]decimal.Decimal, n), make([]decimal.Decimal, n)
	strict, earn = make([]*decimal.Decimal, n), make([]*decimal.Decimal, n)
	for i, y := range used {
		revenue[i], shares[i] = *y.revenue, *y.shares
		if s, ok := y.strict(); ok {
			strict[i] = ptr(s)
		}
		if e, ok := y.earnings(); ok {
			earn[i] = ptr(e)
		}
	}
	return revenue, shares, strict, earn
}

// perShareOf divides each known figure by its year's share count.
func perShareOf(vals []*decimal.Decimal, shares []decimal.Decimal) []*decimal.Decimal {
	out := make([]*decimal.Decimal, len(vals))
	for i, v := range vals {
		if v != nil {
			out[i] = ptr(quo(*v, shares[i]))
		}
	}
	return out
}

// usedYears checks the numbers given and returns the latest recordYears in date order.
func usedYears(years []retYear, sharesNow decimal.Decimal, costs []decimal.Decimal) ([]retYear, string) {
	if sharesNow.Sign() <= 0 {
		return nil, refusalBadInput
	}
	for _, c := range costs {
		if c.Sign() <= 0 {
			return nil, refusalBadInput
		}
	}
	ordered := append([]retYear(nil), years...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].end.Before(ordered[j].end) })
	if len(ordered) > retParams.recordYears {
		ordered = ordered[len(ordered)-retParams.recordYears:]
	}
	return ordered, ""
}

// forecastOf is the analyst path, or why there is none.
func forecastOf(a *analystIn) (path []decimal.Decimal, omission string) {
	switch {
	case a == nil:
		return nil, omitNoAnalyst
	case a.offScale:
		return nil, omitAnalystScale
	}
	return analystPath(*a), ""
}

// earningsOmission is why the earnings lines are left out, or "": the first of an unknown or
// non-positive earnings base, an undefined record, an unknown or low R, and the gate.
func earningsOmission(base *decimal.Decimal, haveRecord, haveR bool, r decimal.Decimal, gate string) string {
	switch {
	case base == nil:
		return omitEarningsBaseUnknown
	case base.Sign() <= 0:
		return omitEarningsBaseNotPos
	case !haveRecord:
		return omitEarningsRecordUndef
	case !haveR:
		return omitRUnknown
	case r.LessThan(retParams.rMin):
		return omitRBelowMin
	}
	return gate
}

func ptr(d decimal.Decimal) *decimal.Decimal { return &d }

// priceReturn is the core: the four lines at each cost, or the reason the inputs are refused
// (the second result). The years may come in any order; the latest recordYears are used.
func priceReturn(years []retYear, sharesNow decimal.Decimal, costs []decimal.Decimal, priceDate time.Time, analyst *analystIn) (returnOut, string) {
	p := retParams
	used, why := usedYears(years, sharesNow, costs)
	if why != "" {
		return returnOut{}, why
	}
	if why = refusalOf(used, sharesNow, priceDate); why != "" {
		return returnOut{}, why
	}
	n := len(used)
	revenue, shares, strict, earn := figuresOf(used)
	strictBase := baseOf(strict, revenue, sharesNow)
	if strictBase == nil || strictBase.Sign() <= 0 {
		return returnOut{}, refusalBaseNotPos
	}
	earningsBase := baseOf(earn, revenue, sharesNow)

	revPS := perShareOf(decimalPtrs(revenue), shares)
	revenueRec, _ := record(revPS, p.endPoint) // every revenue and share count is above zero
	earnRec, earnYears, short, haveEarnRec := earningsRecord(perShareOf(earn, shares))
	var earnRecP *decimal.Decimal
	if haveEarnRec {
		earnRecP = &earnRec
	}
	low := lowPath(revenueRec, earnRecP)
	r, haveR := rOf(used)
	var rP *decimal.Decimal
	if haveR {
		rP = &r
	}
	gate, plantSkipped := gateOf(used, revenue)
	plant, havePlant := plantOf(used, revenue)
	plantFell := havePlant && plant.latest.LessThan(mul(plant.usual, sub(decOne, p.plantDrop)))
	latestStrict := *strict[n-1] // a missing figure of the latest years is refused
	latestBase := quo(latestStrict, sharesNow)

	forecast, forecastOmit := forecastOf(analyst)
	omitted := earningsOmission(earningsBase, haveEarnRec, haveR, r, gate)
	withR := func(key string, growth []decimal.Decimal, why string) scenarioOut {
		if omitted != "" {
			why = omitted
		}
		if why != "" {
			return leftOut(key, earningsBase, why)
		}
		s := scenario(key, *earningsBase, growth, costs, rP, nil)
		if plantFell {
			s.Flags = append(s.Flags, flagPlantRevenueFell)
		}
		return s
	}
	allPaid := func(key string, growth []decimal.Decimal, why string) scenarioOut {
		if why != "" {
			return leftOut(key, strictBase, why)
		}
		return scenario(key, *strictBase, growth, costs, nil, &latestBase)
	}

	out := returnOut{
		State: "ok", PeriodEnd: string(dayText(used[n-1].end)), SharesNow: sharesNow.String(),
		RevenueRecord: rateText(revenueRec), LowPath: rates(low), Flags: []string{},
	}
	out.fill(header{haveEarnRec, short, plantSkipped, earnRec, earnYears, haveR, r, analyst, havePlant, plant})
	out.Scenarios = []scenarioOut{
		allPaid(valuationapi.LineSpendingStays, low, ""),
		withR(valuationapi.LineSpendingEarns, low, ""),
		allPaid(valuationapi.LineAnalystPath, forecast, forecastOmit),
		withR(valuationapi.LineSpendingEarnsAnalyst, forecast, forecastOmit),
	}
	return out, ""
}

// header holds what priceReturn found for the figures above the lines.
type header struct {
	haveEarnRec, short, plantSkipped bool
	earnRec                          decimal.Decimal
	earnYears                        int
	haveR                            bool
	r                                decimal.Decimal
	analyst                          *analystIn
	havePlant                        bool
	plant                            plantFact
}

// fill sets the figures of the header: flags, the earnings record, R, the analyst growth
// and the plant fact.
func (out *returnOut) fill(h header) {
	haveEarnRec, short, plantSkipped, earnRec, earnYears := h.haveEarnRec, h.short, h.plantSkipped, h.earnRec, h.earnYears
	haveR, r, analyst, havePlant, plant := h.haveR, h.r, h.analyst, h.havePlant, h.plant
	switch {
	case !haveEarnRec:
		out.Flags = append(out.Flags, flagEarningsRecordUndef)
	case short:
		out.Flags = append(out.Flags, flagEarningsRecordShort)
	}
	if plantSkipped {
		out.Flags = append(out.Flags, flagPlantNotReported)
	}
	if haveEarnRec {
		out.EarningsRecord, out.EarningsRecordYears = opt(earnRec, 6), &earnYears
	}
	if haveR {
		out.R = opt(r, 6)
	}
	if analyst != nil && !analyst.offScale {
		out.Analyst = &struct {
			Growth string `json:"growth"`
			Years  int    `json:"years"`
		}{rateText(analyst.growth), analyst.years}
	}
	if havePlant {
		out.Plant = &struct {
			Usual  string `json:"usual"`
			Latest string `json:"latest"`
		}{fix(plant.usual, 2), fix(plant.latest, 2)}
	}
}

// yearOf reads one fiscal year of the filings; absent or unparsable figures are unknown.
func yearOf(y valuationapi.FilingYear) (retYear, bool) {
	end, ok := parseDay(y.FYEnd)
	if !ok {
		return retYear{}, false
	}
	get := func(role string) *decimal.Decimal {
		if d, ok := parseValue(y.Values[role]); ok {
			return &d
		}
		return nil
	}
	return retYear{
		end: end, revenue: get(roleRevenue), ocf: get(roleOperatingCashFlow),
		capex: get(roleCapitalSpending), dep: get(roleDepreciation), sbc: get(roleStockCompensation),
		shares: get(roleDilutedShares), plant: get(roleGrossPlant), equity: get(roleEquity),
	}, true
}

// ReturnAtPrice forms the return of selling a put at each cost (the bare strike: the
// premium is already in the put's own return). It implements valuationapi.ReturnAtPriceFunc.
// Each cost has four lines: strict base on the low path with all paid out (SPENDING_STAYS),
// earnings base on the low path with growth paid for (SPENDING_EARNS), and the same two on
// the analyst path (ANALYST_PATH, SPENDING_EARNS_ANALYST). A nil analyst input leaves the
// last two out with state left_out and reason "no analyst input"; a non-nil one gives a
// compound growth over analystYears (5) forecast years, so it carries no first-year figure
// to anchor to the filings' scale; an unparsable growth is left out as ANALYST_SCALE. The
// shares are f.SharesOutstanding and the price day is asOf. Inputs that fail a check are
// refused with their reason (TOO_FEW_YEARS, NOT_CONSECUTIVE, STALE_FILING, MISSING_FIGURE,
// SHARE_JUMP, BASE_NOT_POSITIVE, BAD_INPUT). A line is a rate, never a value per share and
// never a verdict.
func ReturnAtPrice(f valuationapi.Filings, costs []string, a *valuationapi.Analyst, asOf contracts.Day) valuationapi.ReturnResult {
	refused := func(why string) valuationapi.ReturnResult {
		return valuationapi.ReturnResult{State: valuationapi.StateMissingInput, Reasons: []string{why}}
	}
	day, okDay := parseDay(asOf)
	sharesNow, okShares := parseValue(f.SharesOutstanding)
	if !okDay || !okShares || len(costs) == 0 {
		return refused(refusalBadInput)
	}
	cs := make([]decimal.Decimal, len(costs))
	for i, c := range costs {
		d, ok := parseValue(c)
		if !ok {
			return refused(refusalBadInput)
		}
		cs[i] = d
	}
	years := make([]retYear, 0, len(f.Years))
	for _, y := range f.Years {
		ry, ok := yearOf(y)
		if !ok {
			return refused(refusalBadInput)
		}
		years = append(years, ry)
	}
	var in *analystIn
	if a != nil {
		g, ok := parseValue(a.Growth)
		in = &analystIn{growth: g, years: analystYears, offScale: !ok || g.LessThanOrEqual(decOne.Neg())}
	}
	out, why := priceReturn(years, sharesNow, cs, day, in)
	if why != "" {
		return refused(why)
	}
	res := valuationapi.ReturnResult{State: valuationapi.StateOK}
	seen := map[string]bool{}
	flag := func(fs []string) {
		for _, x := range fs {
			if !seen[x] {
				seen[x] = true
				res.Flags = append(res.Flags, x)
			}
		}
	}
	flag(out.Flags)
	for _, s := range out.Scenarios {
		flag(s.Flags)
	}
	for i, c := range costs {
		for _, s := range out.Scenarios {
			line := valuationapi.ReturnLine{Cost: c, Key: s.Key}
			switch {
			case s.Omitted != nil && *s.Omitted == omitNoAnalyst:
				line.State, line.Reason = "left_out", reasonNoAnalystText
			case s.Omitted != nil:
				line.State, line.Reason = valuationapi.StateMissingInput, *s.Omitted
			case s.Solved[i].Rate == nil:
				line.State, line.Reason = valuationapi.StateMissingInput, reasonNoSolution
			default:
				line.State, line.Value = valuationapi.StateOK, *s.Solved[i].Rate
			}
			res.Lines = append(res.Lines, line)
		}
	}
	return res
}

var _ valuationapi.ReturnAtPriceFunc = ReturnAtPrice
