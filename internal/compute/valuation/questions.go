package valuation

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// The four numeric questions of the five (Decision 1; the fifth is the owner's and has no
// figure): Q1 the return on tangible capital, Q2 the capital that growth costs, Q3 pricing
// power and Q4 the cash behind the profit. Ratios of reported annual figures, no forecast,
// no price, no verdict. Every operation keeps 34 significant digits, half-even; a figure
// that cannot be formed is missing_input with its reason, never zero.
const (
	questionYears    = 10 // fiscal years of the window
	questionMinYears = 6  // years with operating income needed
)

// Reasons a question cannot be formed, besides reasonNotEnoughYears.
const (
	reasonNoCapitalFigure = "no_capital_figure"
	reasonFewCapitalYears = "fewer_than_2_capital_years"
	reasonNoNewNopat      = "new_nopat_not_positive"
	reasonNoNewCapital    = "no_new_capital"
	reasonMarginInput     = "margin_input_missing"
	reasonNoCashYear      = "no_cash_year"
	reasonNoProfit        = "net_income_not_positive"
)

var decThree = decimal.NewFromInt(3)

func qOK(key string, d decimal.Decimal) valuationapi.Question {
	return valuationapi.Question{Key: key, State: valuationapi.StateOK, Value: d.String()}
}

func qMissing(key, reason string) valuationapi.Question {
	return valuationapi.Question{Key: key, State: valuationapi.StateMissingInput, Reason: reason}
}

// Questions forms Q1 to Q4 from the filings: the window is the newest ten fiscal years that
// have an operating income, and fewer than six of them leave every question missing. The
// answer of a question is its Key ("Q1" to "Q4"); the keys that follow it ("Q1.worst",
// "Q1.capped_years", "Q2.new_capital", "Q2.new_nopat", "Q3.first", "Q3.basis.gross" or
// "Q3.basis.operating") are the figures it was formed from. It implements
// valuationapi.QuestionsFunc.
func Questions(f valuationapi.Filings, _ contracts.Day) valuationapi.QuestionsResult {
	ends := newest(seriesOf(f, roleOperatingIncome).ends(), questionYears)
	if len(ends) < questionMinYears {
		var qs []valuationapi.Question
		for _, k := range []string{"Q1", "Q2", "Q3", "Q4"} {
			qs = append(qs, qMissing(k, reasonNotEnoughYears))
		}
		return valuationapi.QuestionsResult{Questions: qs}
	}
	years := capYears(f, ends)
	var qs []valuationapi.Question
	qs = append(qs, q1Of(years)...)
	qs = append(qs, q2Of(years)...)
	qs = append(qs, q3Of(f, ends)...)
	qs = append(qs, q4Of(years, seriesOf(f, roleNetIncome))...)
	return valuationapi.QuestionsResult{Questions: qs}
}

var _ valuationapi.QuestionsFunc = Questions

// q1Of is the mean of the yearly returns on tangible capital (NOPAT over capital, at most 1;
// a year of capital at or below zero counts 1 when its NOPAT is positive and is left out
// otherwise), with the worst year and the number of years at the cap.
func q1Of(years []capYear) []valuationapi.Question {
	var returns []decimal.Decimal
	for _, y := range years {
		switch {
		case y.capital == nil:
		case y.capital.Sign() <= 0:
			if y.nopat.Sign() > 0 {
				returns = append(returns, decOne)
			}
		default:
			q, _ := div(y.nopat, *y.capital)
			returns = append(returns, decimal.Min(decOne, q))
		}
	}
	if len(returns) == 0 {
		return []valuationapi.Question{qMissing("Q1", reasonNoCapitalFigure)}
	}
	sum, worst, capped := decimal.Zero, returns[0], 0
	for _, r := range returns {
		sum = add(sum, r)
		worst = decimal.Min(worst, r)
		if r.Equal(decOne) {
			capped++
		}
	}
	mean, _ := div(sum, decimal.NewFromInt(int64(len(returns))))
	return []valuationapi.Question{
		qOK("Q1", mean), qOK("Q1.worst", worst), qOK("Q1.capped_years", decimal.NewFromInt(int64(capped))),
	}
}

// q2Of is the new capital over the new NOPAT, from the first to the last year that has a
// capital. With no growth in NOPAT, or growth that came with no new capital, the ratio does
// not exist and the question is missing, the two figures still given.
func q2Of(years []capYear) []valuationapi.Question {
	var usable []capYear
	for _, y := range years {
		if y.capital != nil {
			usable = append(usable, y)
		}
	}
	if len(usable) < 2 {
		return []valuationapi.Question{qMissing("Q2", reasonFewCapitalYears)}
	}
	first, last := usable[0], usable[len(usable)-1]
	newCapital, newNopat := sub(*last.capital, *first.capital), sub(last.nopat, first.nopat)
	extra := []valuationapi.Question{qOK("Q2.new_capital", newCapital), qOK("Q2.new_nopat", newNopat)}
	switch {
	case newNopat.Sign() <= 0:
		return append([]valuationapi.Question{qMissing("Q2", reasonNoNewNopat)}, extra...)
	case newCapital.Sign() <= 0:
		return append([]valuationapi.Question{qMissing("Q2", reasonNoNewCapital)}, extra...)
	}
	v, _ := div(newCapital, newNopat)
	return append([]valuationapi.Question{qOK("Q2", v)}, extra...)
}

// q3Of is the margin of pricing power: the mean margin of the first three years of the
// window and of the last three, in whole percentage points as a fraction (0.45 is 45%).
// The margin is gross profit over revenue when the six years all have gross profit, else
// operating income over revenue. "Q3" is the last three years, "Q3.first" the first three.
func q3Of(f valuationapi.Filings, ends []contracts.Day) []valuationapi.Question {
	gross, revenue := seriesOf(f, roleGrossProfit), seriesOf(f, roleRevenue)
	picked := append(append([]contracts.Day{}, ends[:3]...), ends[len(ends)-3:]...)
	basis, numerator := "gross", gross
	for _, e := range picked {
		if gross.at(e) == nil {
			basis, numerator = "operating", seriesOf(f, roleOperatingIncome)
			break
		}
	}
	basisQ := valuationapi.Question{Key: "Q3.basis." + basis, State: valuationapi.StateOK}
	var margins []decimal.Decimal
	for _, e := range picked {
		top, bottom := numerator.at(e), revenue.at(e)
		if top == nil || bottom == nil || bottom.Sign() <= 0 {
			return []valuationapi.Question{qMissing("Q3", reasonMarginInput), qMissing("Q3.first", reasonMarginInput), basisQ}
		}
		m, _ := div(*top, *bottom)
		margins = append(margins, m)
	}
	mean := func(ms []decimal.Decimal) decimal.Decimal {
		q, _ := div(add(add(ms[0], ms[1]), ms[2]), decThree)
		return q.RoundBank(2)
	}
	first, last := mean(margins[:3]), mean(margins[3:])
	text := func(key string, d decimal.Decimal) valuationapi.Question {
		return valuationapi.Question{Key: key, State: valuationapi.StateOK, Value: d.StringFixed(2)}
	}
	return []valuationapi.Question{text("Q3", last), text("Q3.first", first), basisQ}
}

// q4Of is the strict free cash flow over the net income, summed over the years that have
// both. The profit must be positive for the ratio to mean anything.
func q4Of(years []capYear, netIncome series) []valuationapi.Question {
	cash, income, n := decimal.Zero, decimal.Zero, 0
	for _, y := range years {
		ni := netIncome.at(y.end)
		if y.fcf == nil || ni == nil {
			continue
		}
		cash, income, n = add(cash, *y.fcf), add(income, *ni), n+1
	}
	switch {
	case n == 0:
		return []valuationapi.Question{qMissing("Q4", reasonNoCashYear)}
	case income.Sign() <= 0:
		return []valuationapi.Question{qMissing("Q4", reasonNoProfit)}
	}
	v, _ := div(cash, income)
	return []valuationapi.Question{qOK("Q4", v)}
}
