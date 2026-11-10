package valuation

import (
	"sort"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// The diagnosis (Decision 1): ten fiscal years of context for a cyclical or a financial
// company, never a value or a gate. Cyclical: operating margin (operating income over
// revenue) and inventory over revenue. Financial: return on equity and return on assets,
// each on the mean of the opening and the closing balance, the opening one being the
// balance dated a fiscal year before. A figure that cannot be formed is missing_input.
const (
	diagnosisYears    = 10
	diagnosisMinYears = 5 // years with a primary figure needed to read the statistics
	openingDaysMin    = 350
	openingDaysMax    = 381 // a 380-day year starts the day after its opening balance
)

// The diagnosis's own SIC lists (REITs, 6798, are left out of the financial one on purpose);
// they are not the lists that make a company unvalued.
var (
	diagnosisFinancialSIC = [][2]int{{6000, 6797}, {6799, 6799}}
	diagnosisCyclicalSIC  = [][2]int{
		{1000, 1499}, {1500, 1799}, {2800, 2832}, {2837, 2899}, {2900, 2999}, {3300, 3399},
		{3520, 3569}, {3674, 3674}, {3700, 3799}, {4400, 4499}, {4512, 4512},
	}
)

const (
	groupCyclical  = "cyclical"
	groupFinancial = "financial"
)

func dOK(key string, d decimal.Decimal) valuationapi.DiagnosisFigure {
	return valuationapi.DiagnosisFigure{Key: key, State: valuationapi.StateOK, Value: d.String()}
}

func dMissing(key, reason string) valuationapi.DiagnosisFigure {
	return valuationapi.DiagnosisFigure{Key: key, State: valuationapi.StateMissingInput, Reason: reason}
}

// dOpt is a figure that may not have been formed.
func dOpt(key string, d *decimal.Decimal) valuationapi.DiagnosisFigure {
	if d == nil {
		return dMissing(key, reasonNotFormed)
	}
	return dOK(key, *d)
}

// dCount is a whole number figure.
func dCount(key string, n int) valuationapi.DiagnosisFigure {
	return dOK(key, decimal.NewFromInt(int64(n)))
}

// ratioOf is numerator over denominator, nil when either is missing or the denominator is
// not positive.
func ratioOf(num, den *decimal.Decimal) *decimal.Decimal {
	if num == nil || den == nil || den.Sign() <= 0 {
		return nil
	}
	q, _ := div(*num, *den)
	return &q
}

// meanRatio is the flow over the mean of the opening and the closing balance, both of
// which must be positive.
func meanRatio(flow, closing, opening *decimal.Decimal) *decimal.Decimal {
	if closing == nil || opening == nil || closing.Sign() <= 0 || opening.Sign() <= 0 {
		return nil
	}
	mean, _ := div(add(*opening, *closing), decTwo)
	return ratioOf(flow, &mean)
}

// openingOf is the latest balance dated 350 to 381 days before end.
func openingOf(balance series, end contracts.Day) *decimal.Decimal {
	e, ok := parseDay(end)
	if !ok {
		return nil
	}
	var best contracts.Day
	for d := range balance {
		t, ok := parseDay(d)
		if !ok {
			continue
		}
		if days := int(e.Sub(t).Hours() / 24); days >= openingDaysMin && days <= openingDaysMax && d > best {
			best = d
		}
	}
	if best == "" {
		return nil
	}
	return balance.at(best)
}

// medianOf is the median of the values, the mean of the two middle ones for an even count.
func medianOf(vs []decimal.Decimal) decimal.Decimal {
	s := append([]decimal.Decimal(nil), vs...)
	sort.Slice(s, func(i, j int) bool { return s[i].LessThan(s[j]) })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	q, _ := div(add(s[n/2-1], s[n/2]), decTwo)
	return q
}

// groupOf names the group a SIC belongs to, or "" when it is in neither list.
func groupOf(sic string) string {
	d, ok := parseValue(sic)
	if !ok || !d.IsInteger() || d.Sign() < 0 {
		return ""
	}
	code := int(d.IntPart())
	switch {
	case inRanges(code, diagnosisFinancialSIC):
		return groupFinancial
	case inRanges(code, diagnosisCyclicalSIC):
		return groupCyclical
	}
	return ""
}

// diagRow is one fiscal year: the primary figure (operating margin, or ROE), the secondary
// (inventory over revenue, or ROA) and the scale (revenue, or the closing equity).
type diagRow struct {
	end                       contracts.Day
	primary, secondary, scale *decimal.Decimal
}

// Diagnosis forms the diagnosis of a company from its filings; the group comes from the SIC
// of the filings, and a company in neither list has none and no figures. The figures are, per
// fiscal year of the window, "operating_margin@<end>", "inventory_to_revenue@<end>" and
// "revenue@<end>" (cyclical) or "roe@<end>", "roa@<end>" and "equity@<end>" (financial),
// then "ranked", "loss_years", "record_years", and the statistics "latest_rank",
// "<primary>_median", "<secondary>_median", "<primary>_usual_low", "<primary>_usual_high"
// (the lowest and the highest primary figure), "<secondary>_latest" and, cyclical only,
// "revenue_vs_peak". With fewer than five years that have a primary figure the statistics
// are missing_input with the reason too_short. It implements valuationapi.DiagnosisFunc.
func Diagnosis(f valuationapi.Filings, _ contracts.Day) valuationapi.DiagnosisResult {
	group := groupOf(f.SIC)
	if group == "" {
		return valuationapi.DiagnosisResult{}
	}
	cyclical := group == groupCyclical
	netIncome, revenue := seriesOf(f, roleNetIncome), seriesOf(f, roleRevenue)
	op, inv := seriesOf(f, roleOperatingIncome), seriesOf(f, roleInventory)
	equity, assets := seriesOf(f, roleEquity), seriesOf(f, roleAssets)

	anchor := netIncome
	if cyclical {
		anchor = revenue
	}
	ends := newest(anchor.ends(), diagnosisYears)
	rows := make([]diagRow, 0, len(ends))
	for _, e := range ends {
		if cyclical {
			r := revenue.at(e)
			rows = append(rows, diagRow{e, ratioOf(op.at(e), r), ratioOf(inv.at(e), r), r})
			continue
		}
		income, closing := netIncome.at(e), equity.at(e)
		rows = append(rows, diagRow{
			e,
			meanRatio(income, closing, openingOf(equity, e)),
			meanRatio(income, assets.at(e), openingOf(assets, e)),
			closing,
		})
	}

	names := [3]string{"roe", "roa", "equity"}
	if cyclical {
		names = [3]string{"operating_margin", "inventory_to_revenue", "revenue"}
	}
	res := valuationapi.DiagnosisResult{Group: group}
	var primaries, secondaries []decimal.Decimal
	for _, r := range rows {
		res.Figures = append(res.Figures,
			dOpt(names[0]+"@"+string(r.end), r.primary),
			dOpt(names[1]+"@"+string(r.end), r.secondary),
			dOpt(names[2]+"@"+string(r.end), r.scale))
		if r.primary != nil {
			primaries = append(primaries, *r.primary)
		}
		if r.secondary != nil {
			secondaries = append(secondaries, *r.secondary)
		}
	}
	losses, records := 0, 0
	for _, e := range ends {
		if ni := netIncome.at(e); ni != nil {
			records++
			if ni.Sign() < 0 {
				losses++
			}
		}
	}
	res.Figures = append(res.Figures, dCount("ranked", len(primaries)), dCount("loss_years", losses), dCount("record_years", records))

	stats := []valuationapi.DiagnosisFigure{
		dRank(rows), dMedian(names[0]+"_median", primaries), dMedian(names[1]+"_median", secondaries),
		dUsual(names[0]+"_usual_low", primaries, true), dUsual(names[0]+"_usual_high", primaries, false),
	}
	if len(rows) > 0 {
		stats = append(stats, dOpt(names[1]+"_latest", rows[len(rows)-1].secondary))
	} else {
		stats = append(stats, dMissing(names[1]+"_latest", reasonNotFormed))
	}
	if cyclical {
		stats = append(stats, dOpt("revenue_vs_peak", peakRatio(rows)))
	}
	for _, s := range stats {
		if len(primaries) < diagnosisMinYears {
			s = dMissing(s.Key, reasonTooShort)
		}
		res.Figures = append(res.Figures, s)
	}
	return res
}

var _ valuationapi.DiagnosisFunc = Diagnosis

// dRank is the rank of the latest year's primary figure in the window, 1 being the highest.
func dRank(rows []diagRow) valuationapi.DiagnosisFigure {
	if len(rows) == 0 || rows[len(rows)-1].primary == nil {
		return dMissing("latest_rank", reasonNotFormed)
	}
	latest, rank := *rows[len(rows)-1].primary, 1
	for _, r := range rows {
		if r.primary != nil && r.primary.GreaterThan(latest) {
			rank++
		}
	}
	return dCount("latest_rank", rank)
}

func dMedian(key string, vs []decimal.Decimal) valuationapi.DiagnosisFigure {
	if len(vs) == 0 {
		return dMissing(key, reasonNotFormed)
	}
	return dOK(key, medianOf(vs))
}

// dUsual is the lowest (or the highest) of the values, not_formed when there are none.
func dUsual(key string, vs []decimal.Decimal, low bool) valuationapi.DiagnosisFigure {
	if len(vs) == 0 {
		return dMissing(key, reasonNotFormed)
	}
	best := vs[0]
	for _, v := range vs[1:] {
		if (low && v.LessThan(best)) || (!low && v.GreaterThan(best)) {
			best = v
		}
	}
	return dOK(key, best)
}

// peakRatio is the latest revenue over the highest revenue of the window, nil when the
// latest has none or the highest is not positive.
func peakRatio(rows []diagRow) *decimal.Decimal {
	if len(rows) == 0 || rows[len(rows)-1].scale == nil {
		return nil
	}
	var peak *decimal.Decimal
	for _, r := range rows {
		if r.scale != nil && (peak == nil || r.scale.GreaterThan(*peak)) {
			peak = r.scale
		}
	}
	return ratioOf(rows[len(rows)-1].scale, peak)
}
