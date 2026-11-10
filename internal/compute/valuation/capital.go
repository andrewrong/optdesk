package valuation

import (
	"sort"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Roles the five questions and the diagnosis read from FilingYear.Values, beside the roles
// of params.go. A fiscal year carries one only when the parameters of its standard list the
// concepts of that role; without it the figure that needs it is missing_input.
const (
	roleOperatingIncome = "operating_income"
	rolePretaxIncome    = "pretax_income"
	roleIncomeTax       = "income_tax"
	rolePPE             = "ppe"
	roleReceivables     = "receivables"
	roleInventory       = "inventory"
	rolePayables        = "payables"
	roleGrossProfit     = "gross_profit"
	roleAssets          = "assets"
)

// Reasons a question or a diagnosis figure cannot be formed.
const (
	reasonNotEnoughYears = "not_enough_years"
	reasonTooShort       = "too_short"
	reasonNotFormed      = "not_formed"
)

// Tax rate of a year: income tax over pretax income, held between these limits, which is
// also the rate when either figure is missing or the pretax income is not positive.
var (
	taxRateMin = decimal.RequireFromString("0.15")
	taxRateMax = decimal.RequireFromString("0.28")
)

// series is one role by fiscal-year end.
type series map[contracts.Day]decimal.Decimal

// seriesOf reads a role of every year of f that has a figure for it.
func seriesOf(f valuationapi.Filings, role string) series {
	s := series{}
	for _, y := range f.Years {
		if d, ok := parseValue(y.Values[role]); ok {
			s[y.FYEnd] = d
		}
	}
	return s
}

// ends lists the fiscal-year ends of s, oldest first.
func (s series) ends() []contracts.Day {
	out := make([]contracts.Day, 0, len(s))
	for d := range s {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// at gives the figure of an end, nil when there is none.
func (s series) at(end contracts.Day) *decimal.Decimal {
	if d, ok := s[end]; ok {
		return &d
	}
	return nil
}

// newest keeps the last n ends of an ascending list.
func newest(ends []contracts.Day, n int) []contracts.Day {
	if len(ends) > n {
		return ends[len(ends)-n:]
	}
	return ends
}

// taxRate is the rate NOPAT takes off the operating income of a year.
func taxRate(tax, pretax *decimal.Decimal) decimal.Decimal {
	if tax == nil || pretax == nil || pretax.Sign() <= 0 {
		return taxRateMax
	}
	q, _ := div(*tax, *pretax)
	return decimal.Min(decimal.Max(q, taxRateMin), taxRateMax)
}

// capYear is one fiscal year of the five questions: its NOPAT (operating income less tax at
// the rate of the year), its tangible capital and its strict free cash flow, nil when
// the figures to form them are missing.
type capYear struct {
	end     contracts.Day
	nopat   decimal.Decimal
	capital *decimal.Decimal
	fcf     *decimal.Decimal
}

// capitalOf is property and equipment plus receivables plus inventory less payables; a
// missing receivable or inventory counts as nothing, a missing plant or payable gives none.
func capitalOf(ppe, receivables, inventory, payables series, end contracts.Day) *decimal.Decimal {
	p, pay := ppe.at(end), payables.at(end)
	if p == nil || pay == nil {
		return nil
	}
	c := *p
	if r := receivables.at(end); r != nil {
		c = add(c, *r)
	}
	if i := inventory.at(end); i != nil {
		c = add(c, *i)
	}
	c = sub(c, *pay)
	return &c
}

// capYears forms the years of the window: operating income, then each year's NOPAT,
// capital and strict free cash flow (operating cash flow less capital spending less stock
// compensation, the last one counting as nothing when missing).
func capYears(f valuationapi.Filings, ends []contracts.Day) []capYear {
	op := seriesOf(f, roleOperatingIncome)
	tax, pretax := seriesOf(f, roleIncomeTax), seriesOf(f, rolePretaxIncome)
	ppe, rec := seriesOf(f, rolePPE), seriesOf(f, roleReceivables)
	inv, pay := seriesOf(f, roleInventory), seriesOf(f, rolePayables)
	ocf, capex, sbc := seriesOf(f, roleOperatingCashFlow), seriesOf(f, roleCapitalSpending), seriesOf(f, roleStockCompensation)
	years := make([]capYear, 0, len(ends))
	for _, e := range ends {
		y := capYear{end: e, capital: capitalOf(ppe, rec, inv, pay, e)}
		y.nopat = mul(op[e], sub(decOne, taxRate(tax.at(e), pretax.at(e))))
		if o, c := ocf.at(e), capex.at(e); o != nil && c != nil {
			v := sub(*o, *c)
			if s := sbc.at(e); s != nil {
				v = sub(v, *s)
			}
			y.fcf = &v
		}
		years = append(years, y)
	}
	return years
}
