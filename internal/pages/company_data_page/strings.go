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

	textUnderlyings = "The list of companies could not be read, so no row is shown."
)

type column struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Gloss string `json:"gloss"`
}

var columns = []column{
	{"fiscal_year_end", "Fiscal year end", "the month in which the company's financial year ends"},
	{"price_to", "Price up to", "the day of the newest daily price bar stored for the company"},
	{"bar_count", "Daily bars", "how many daily price bars are stored for the last 400 days"},
	{"statements", "Statements", "whether at least one quarter of reported figures is available"},
	{"latest_quarter", "Latest quarter", "the newest fiscal quarter with figures, named by its fiscal year and quarter"},
	{"eps_quarters", "EPS quarters", "how many quarters have a diluted earnings-per-share figure"},
	{"next_report", "Next report", "the date of the next earnings report if a source has listed one"},
	{"missing", "Missing", "what is not stored for the company, and why"},
}

type glossaryRow struct {
	Term string `json:"term"`
	Text string `json:"text"`
}

var glossary = []glossaryRow{
	{"Fiscal year end", "The month in which the company's financial year ends; it comes from its 12-month figures."},
	{"Price up to", "The day of the newest daily price bar stored for the company."},
	{"Daily bars", "How many daily price bars (open, high, low, close of one day) are stored for the last 400 days."},
	{"Statements", "Yes when at least one quarter of revenue, net income or earnings per share is available."},
	{"Latest quarter", "The newest fiscal quarter with figures, named by its fiscal year and quarter."},
	{"EPS quarters", "How many quarters have a diluted earnings-per-share figure (net income per share)."},
	{"Next report", "The date of the next earnings report when a source has listed one; empty when none is listed."},
	{string(companydataapi.ReasonJP), "A Japanese company: no source of statements is wired, which is left out on purpose."},
	{string(companydataapi.ReasonNoCIK), "A US company with no SEC number (cik); type it on the underlyings page."},
	{string(companydataapi.ReasonFundamentalsOff), "The statements flag of the company is off, so no filing is fetched."},
	{string(companydataapi.ReasonNoStatements), "No US-GAAP fact is stored: either an IFRS filer, whose quarters are left out on purpose, " +
		"or the daily run has not stored it yet."},
	{string(companydataapi.ReasonAnnualOnly), "The filer reports once a year (forms 20-F or 40-F), so there are no quarters."},
	{string(companydataapi.ReasonNoInstrument), "The broker has not resolved the company yet (no con_id), so no price bars can be collected."},
	{string(companydataapi.ReasonNoBars), "The company is resolved but no daily price bar is stored yet."},
	{string(companydataapi.ReasonNoNextReport), "No report date is listed for the company; this is for information only."},
}
