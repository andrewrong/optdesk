// Package valuationapi declares the valuation rule of compute: the function types and
// the types they use. It holds declarations only; the implementation is
// internal/compute/valuation. Design: docs/design/valuation.md#valuation-compute-interface.
//
// Every number is decimal text; no float, no library type, no I/O and no clock. An empty
// string is "no value". A figure that cannot be formed has a state and a reason, never
// zero. No verdict word is used anywhere: a result gives figures, states and reasons.
package valuationapi

import "github.com/andrewrong/optdesk/internal/contracts"

// Standard is the accounting standard of a company.
type Standard string

// The three standards. The empty Standard means none was found.
const (
	USGAAP Standard = "US_GAAP"
	JGAAP  Standard = "J_GAAP"
	IFRS   Standard = "IFRS"
)

// Result states shared by the result types.
const (
	// StateOK means the figure was formed.
	StateOK = "ok"
	// StateMissingInput means the figure cannot be formed; the Reason says why.
	StateMissingInput = "missing_input"
	// StateUnvalued means the company is not valued (financial or cyclical SIC range);
	// the Reason says which, and the diagnosis is given instead of a band.
	StateUnvalued = "unvalued"
)

// StandardOfFunc reads the accounting standard of a company from its own facts:
// US_GAAP when the facts have source "sec", else the unit of the fact with taxonomy
// "dei" and concept "AccountingStandard" (J_GAAP or IFRS). No standard gives the empty
// Standard, and BuildFilings then refuses with the reason no_standard.
type StandardOfFunc func(facts []contracts.FundamentalFact) Standard

// BuildFilingsFunc turns the facts of one company into ten fiscal years of figures.
// It takes the facts as the store returns them, applies the as-of rule (latest filed on
// or before asOf) and the eligible-year rule, and returns the years or a refusal.
type BuildFilingsFunc func(facts []contracts.FundamentalFact, asOf contracts.Day, std Standard) FilingsResult

// BandFunc forms the buy band of a company. It takes no price: the band is price-free.
// It holds no expiry and no clock; the approval day sets validity in the page builder.
type BandFunc func(id string, f Filings, asOf contracts.Day) BandFigure

// ImpliedGrowthFunc solves the reverse DCF growth that the spot price implies. spot is
// decimal text.
type ImpliedGrowthFunc func(f Filings, spot string) ImpliedResult

// ReturnAtPriceFunc forms the return of selling a put at each cost (the bare strike,
// decimal text). a is the optional analyst input; nil means absent.
type ReturnAtPriceFunc func(f Filings, costs []string, a *Analyst, asOf contracts.Day) ReturnResult

// QuestionsFunc forms the figures of questions Q1 to Q4 (Q5 has no figure).
type QuestionsFunc func(f Filings, asOf contracts.Day) QuestionsResult

// DiagnosisFunc forms the diagnosis figures of a cyclical or financial company.
type DiagnosisFunc func(f Filings, asOf contracts.Day) DiagnosisResult

// FilingYear is one eligible fiscal year. Values maps the concept name the parameters
// of the standard use to its decimal text.
type FilingYear struct {
	FYEnd  contracts.Day
	Filed  contracts.Day
	Values map[string]string
}

// Filings is the ten fiscal years of one company, newest first, as decimal text.
type Filings struct {
	Standard Standard
	Currency string
	SIC      string
	Years    []FilingYear
	// SharesOutstanding is the latest diluted share count; CoverShares is the cover
	// page count used for the share-count check. Both are empty when absent.
	SharesOutstanding string
	CoverShares       string
}

// FilingsResult is BuildFilings' answer: Filings with State "ok", or a refusal.
// SIC is the SIC text and SICState is "ok" or "missing" (a company with no SIC fact is
// refused). State is "ok", "missing_input" or "unvalued". Reasons are those of
// Decision 5: no_standard, no_sic, financial_sic, cyclical_sic, fewer_than_5_years,
// non_consecutive_years, share_count_jump, stale_latest_year, non_positive_base.
type FilingsResult struct {
	Filings  Filings
	SIC      string
	SICState string
	State    string
	Reasons  []string
}

// BandFigure is the band of one company. Value is the value per share at the primary
// yield (the maximum assignment price), Low at the high required yield (lower edge) and
// RefHigh at the low required yield. State is "ok", "missing_input" or "unvalued";
// Reasons says why when it is not "ok". Flags name leverage and instability.
type BandFigure struct {
	ID        string
	State     string
	Value     string
	Low       string
	RefHigh   string
	Currency  string
	BaseFYEnd contracts.Day
	Flags     []string
	Reasons   []string
}

// ImpliedRate is the growth implied at one discount rate; both are decimal text.
type ImpliedRate struct {
	Rate   string
	Growth string
}

// ImpliedResult is the reverse DCF at the spot price. Primary is the growth at the
// primary rate; Rates holds every rate. State is "ok" or "missing_input" with Reasons.
type ImpliedResult struct {
	State   string
	Primary string
	Rates   []ImpliedRate
	Reasons []string
}

// Analyst is the optional analyst input of ReturnAtPrice. The page passes nil: lines 3
// and 4 then show the state "left out: no analyst input".
type Analyst struct {
	Growth string
	AsOf   contracts.Day
}

// The four line keys of ReturnLine.Key.
const (
	LineSpendingStays        = "SPENDING_STAYS"
	LineSpendingEarns        = "SPENDING_EARNS"
	LineAnalystPath          = "ANALYST_PATH"
	LineSpendingEarnsAnalyst = "SPENDING_EARNS_ANALYST"
)

// ReturnLine is one line of the return at one cost. Value is the annual return as
// decimal text; State is "ok", "missing_input" or "left_out" (no analyst input), and
// Reason says why when it is not "ok".
type ReturnLine struct {
	Cost   string
	Key    string
	State  string
	Value  string
	Reason string
}

// ReturnResult is the return at a price. Each cost has four lines with the keys
// SPENDING_STAYS, SPENDING_EARNS, ANALYST_PATH and SPENDING_EARNS_ANALYST. It holds no
// value figure and no verdict word. State is "ok" or "missing_input" (a refusal, with
// Reasons); Flags name the limits the figures met.
type ReturnResult struct {
	State   string
	Lines   []ReturnLine
	Flags   []string
	Reasons []string
}

// Question is one of Q1 (return on tangible capital), Q2 (growth cost), Q3 (pricing
// power) or Q4 (cash behind profit). Key is "Q1" to "Q4".
type Question struct {
	Key    string
	State  string
	Value  string
	Reason string
}

// QuestionsResult holds the figures of Q1 to Q4; a figure that cannot be formed is
// "missing_input" with its reason.
type QuestionsResult struct {
	Questions []Question
}

// DiagnosisFigure is one figure of the diagnosis.
type DiagnosisFigure struct {
	Key    string
	State  string
	Value  string
	Reason string
}

// DiagnosisResult is the diagnosis of a company. Group is "cyclical" (margins and
// inventory), "financial" (ROE and ROA) or empty when none applies.
type DiagnosisResult struct {
	Group   string
	Figures []DiagnosisFigure
}
