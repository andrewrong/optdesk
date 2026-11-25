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
	{"fiscal_year_end", labelFiscalYearEnd, glossFiscalYearEnd},
	{"price_to", labelPriceTo, glossPriceTo},
	{"bar_count", labelBarCount, glossBarCount},
	{"statements", labelStatements, glossStatements},
	{"latest_quarter", labelLatestQuarter, glossLatestQuarter},
	{"eps_quarters", labelEPSQuarters, glossEPSQuarters},
	{"next_report", labelNextReport, glossNextReport},
	{"missing", labelMissing, glossMissing},
}

type glossaryRow struct {
	Term string `json:"term"`
	Text string `json:"text"`
}

var glossary = []glossaryRow{
	{labelFiscalYearEnd, textFiscalYearEnd},
	{labelPriceTo, textPriceTo},
	{labelBarCount, textDailyBars},
	{labelStatements, textStatements},
	{labelLatestQuarter, textLatestQuarter},
	{labelEPSQuarters, textEPSQuarters},
	{labelNextReport, textNextReport},
	{string(companydataapi.ReasonJP), textReasonJP},
	{string(companydataapi.ReasonNoCIK), textReasonNoCIK},
	{string(companydataapi.ReasonFundamentalsOff), textReasonFundOff},
	{string(companydataapi.ReasonNoStatements), textReasonNoStmts},
	{string(companydataapi.ReasonAnnualOnly), textReasonAnnual},
	{string(companydataapi.ReasonNoInstrument), textReasonNoInstr},
	{string(companydataapi.ReasonNoBars), textReasonNoBars},
	{string(companydataapi.ReasonNoNextReport), textReasonNoNextRep},
}
