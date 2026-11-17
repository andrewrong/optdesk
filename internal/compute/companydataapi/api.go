// Package companydataapi declares the company data calculator: the types, the constants and the
// Calculator that is the only place single quarters and their summary are derived from stored
// SEC facts. Declarations only. Numbers are decimal text, never a float; no clock, no I/O.
package companydataapi

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Why is the reason a cell has no value (five codes).
type Why string

// The codes of Why.
const (
	WhyNone              Why = ""                    // the cell has a value
	WhyNoFact            Why = "no_fact"             // no filing reported it
	WhyCannotDerive      Why = "cannot_derive"       // a year-to-date operand or a quarter-length period is missing
	WhyPeriodMismatch    Why = "period_mismatch"     // fp of the filing disagrees with the periods; the fiscal year is not guessed
	WhyShareBasisChanged Why = "share_basis_changed" // diluted share counts of Q1 to Q3 and FY differ by more than one band (split)
)

// Cell is one figure of one quarter. Value is exact decimal text, empty when Missing.
type Cell struct {
	Value    string
	Missing  bool
	Derived  bool   // year-to-date minus year-to-date, FY EPS minus three quarters, or operating cash flow minus capex
	Restated bool   // two filings gave different values for the period
	Concept  string // the concept the value was read from; empty when Missing
	Why      Why
}

// Quarter is one fiscal quarter of one issuer, newest first in the answer.
type Quarter struct {
	Label              string        // "FY2025 Q3" from fy and fp of the original filing; "" when fy is unknown
	FiscalYear         int           // 0 when unknown
	FiscalYearEndMonth int           // 1 to 12, from the 12-month fact; 0 when unknown
	Number             int           // 1 to 4
	PeriodEnd          contracts.Day // the quarter end
	FilingDate         contracts.Day // the earliest filed of the facts of the quarter end

	Revenue, OperatingIncome, NetIncome, EPSDiluted, SharesDiluted Cell
	OperatingCashFlow, Capex, FreeCash, Buybacks, Dividends        Cell
	Cash, LongTermDebt, ShortTermDebt                              Cell
}

// Reason is a code of "what is missing and why".
type Reason string

// The codes of Reason.
const (
	ReasonJP              Reason = "jp"               // market JP: no statements source; left out on purpose
	ReasonNoCIK           Reason = "no_cik"           // US, no cik issuer
	ReasonFundamentalsOff Reason = "fundamentals_off" // the flag is off
	ReasonNoStatements    Reason = "no_statements"    // US, CIK, flag on, no us-gaap fact stored
	ReasonAnnualOnly      Reason = "annual_only"      // 20-F or 40-F facts and no 10-Q: no quarters
	ReasonNoInstrument    Reason = "no_instrument"    // no con_id yet: no bars can be collected
	ReasonNoBars          Reason = "no_bars"          // instrument but no bar
	ReasonNoNextReport    Reason = "no_next_report"   // no listed report date; informational
)

// SummaryInput is what Summarize needs for one company.
type SummaryInput struct {
	Market        string          // "US" or "JP"
	Issuer        string          // "cik:0000..." or "edinet:..." or ""
	Fundamentals  bool            // the fundamentals flag of the underlying
	HasInstrument bool            // con_id resolved
	BarDays       []contracts.Day // days of bars with status ok
	Facts         []contracts.FundamentalFact
	NextReport    *contracts.Day
}

// Summary is the per company answer of the data page.
type Summary struct {
	LastBarDay         *contracts.Day
	BarCount           int
	FiscalYearEndMonth int  // 0 when unknown
	Statements         bool // at least one quarter with a revenue, net income or EPS value
	LatestLabel        string
	LatestEnd          *contracts.Day
	EPSQuarters        int // quarters with an EPS value
	Missing            []Missing
}

// Missing is one item of "what is missing and why"; Text is the one-sentence explanation the page prints.
type Missing struct {
	Reason Reason
	Text   string
}

// PeriodFactReader reads the facts with fy, fp and frame (the view rd.fundamental_fact_r2);
// FactsFiledBy stays on r1.
type PeriodFactReader interface {
	// FactsWithPeriodFiledBy returns every row of the subject with filed on or before cutoff;
	// no winner is picked.
	FactsWithPeriodFiledBy(ctx context.Context, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error)
}

// Calculator is the only place that derives these numbers.
type Calculator interface {
	// Quarters derives the single quarters of one issuer from its facts, newest first. The slice
	// is never nil. A quarter that cannot be derived is a Missing cell with a Why, never a wrong number.
	Quarters(facts []contracts.FundamentalFact) ([]Quarter, error)
	// Summarize counts what is stored and lists what is missing and why. Missing is never nil.
	Summarize(in SummaryInput) (Summary, error)
}
