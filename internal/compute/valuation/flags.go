package valuation

import "github.com/shopspring/decimal"

// Flags of the band, named as in the system this ports.
const (
	flagUnstableFCF      = "unstable_fcf"
	flagLeverageUnknown  = "leverage_unknown"
	flagHighLeverage     = "high_leverage"
	flagNegativeCapex    = "negative_capex_reported"
	flagSharesUnverified = "shares_unverified"
)

// bandFlags lists the flags of a formed band: the free cash flow is unstable (a year at or
// below zero, or a range wider than its median), the leverage is unknown or high, a capital
// spending was reported negative, the cover-page share count is absent.
func bandFlags(p BandParams, rows []fcfRow, medFCF, medOCF decimal.Decimal, debt *decimal.Decimal, covered bool) []string {
	var flags []string
	lo, hi := rows[0].fcf, rows[0].fcf
	for _, r := range rows {
		lo, hi = decimal.Min(lo, r.fcf), decimal.Max(hi, r.fcf)
	}
	// (hi-lo)/medFCF > limit, the median being positive.
	if lo.Sign() <= 0 || hi.Sub(lo).GreaterThan(medFCF.Mul(p.MaxFCFRangeToMedian)) {
		flags = append(flags, flagUnstableFCF)
	}
	switch {
	case debt == nil:
		flags = append(flags, flagLeverageUnknown)
	case medOCF.Sign() <= 0 || debt.GreaterThan(medOCF.Mul(p.MaxDebtToMedianOCF)):
		flags = append(flags, flagHighLeverage)
	}
	for _, r := range rows {
		if r.capex.Sign() < 0 {
			flags = append(flags, flagNegativeCapex)
			break
		}
	}
	if !covered {
		flags = append(flags, flagSharesUnverified)
	}
	return flags
}

// unvaluedSIC gives the reason a SIC code is not valued, or "": a financial range or a
// cyclical range of the parameters. A code that is not a number is not judged here;
// BuildFilings refuses a company with no SIC.
func unvaluedSIC(sic string, p Params) string {
	d, ok := parseValue(sic)
	if !ok || !d.IsInteger() || d.Sign() < 0 {
		return ""
	}
	code := int(d.IntPart())
	switch {
	case inRanges(code, p.FinancialSIC):
		return reasonFinancialSIC
	case inRanges(code, p.CyclicalSIC):
		return reasonCyclicalSIC
	}
	return ""
}
