package companydata

import (
	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Calc is the company data calculator: Quarters and Summarize over stored facts.
type Calc struct{}

var _ companydataapi.Calculator = Calc{}

// Quarters is the package function Quarters.
func (Calc) Quarters(facts []contracts.FundamentalFact) ([]companydataapi.Quarter, error) {
	return Quarters(facts)
}

// Summarize counts what is stored and lists what is missing and why. Missing is never nil.
func (Calc) Summarize(in companydataapi.SummaryInput) (companydataapi.Summary, error) {
	s := companydataapi.Summary{BarCount: len(in.BarDays), Missing: reasons(in)}
	for _, d := range in.BarDays {
		if s.LastBarDay == nil || d > *s.LastBarDay {
			d := d
			s.LastBarDay = &d
		}
	}
	qs, err := Quarters(in.Facts)
	if err != nil {
		return companydataapi.Summary{}, err
	}
	for _, q := range qs { // newest first
		if s.FiscalYearEndMonth == 0 {
			s.FiscalYearEndMonth = q.FiscalYearEndMonth
		}
		if !q.EPSDiluted.Missing {
			s.EPSQuarters++
		}
		if q.Revenue.Missing && q.NetIncome.Missing && q.EPSDiluted.Missing {
			continue
		}
		if !s.Statements {
			s.Statements, s.LatestLabel = true, q.Label
			end := q.PeriodEnd
			s.LatestEnd = &end
		}
	}
	return s, nil
}
