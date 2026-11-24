// strings.go holds every text of the page (docs/design/data-collection.md, Decisions 9 to 12).
package company_data_page

import "github.com/andrewrong/optdesk/internal/compute/companydataapi"

const (
	pageName       = "company_data_page"
	runID          = "company_data"
	timeLayout     = "2006-01-02T15:04:05-07:00"
	dataHealthPath = "/data-health"
	quartersBase   = "/company-quarters"
	labelPrefix    = "Quarter ending "

	gapUnderlyings = "underlyings_unavailable"
	gapRead        = "read_failed"
)

type column struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Gloss string `json:"gloss"`
}

var columns = []column{
	{"fiscal_year_end", "Fiscal year end", glossFiscalYearEnd},
	{"price_to", "Price up to", glossPriceTo},
	{"bar_count", "Daily bars", glossBarCount},
	{"statements", "Statements", glossStatements},
	{"latest_quarter", "Latest quarter", glossLatestQuarter},
	{"eps_quarters", "EPS quarters", glossEPSQuarters},
	{"next_report", "Next report", glossNextReport},
	{"missing", "Missing", glossMissing},
}

type glossaryRow struct {
	Term string `json:"term"`
	Text string `json:"text"`
}

var glossary = []glossaryRow{
	{"Fiscal year end", textFiscalYearEnd},
	{"Price up to", textPriceTo},
	{"Daily bars", textDailyBars},
	{"Statements", textStatements},
	{"Latest quarter", textLatestQuarter},
	{"EPS quarters", textEPSQuarters},
	{"Next report", textNextReport},
	{string(companydataapi.ReasonJP), textReasonJP},
	{string(companydataapi.ReasonNoCIK), textReasonNoCIK},
	{string(companydataapi.ReasonFundamentalsOff), textReasonFundOff},
	{string(companydataapi.ReasonNoStatements), textReasonNoStmts},
	{string(companydataapi.ReasonAnnualOnly), textReasonAnnual},
	{string(companydataapi.ReasonNoInstrument), textReasonNoInstr},
	{string(companydataapi.ReasonNoBars), textReasonNoBars},
	{string(companydataapi.ReasonNoNextReport), textReasonNoNextRep},
}
