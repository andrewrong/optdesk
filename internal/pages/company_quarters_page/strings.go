// strings.go holds every text of the page (docs/design/data-collection.md, Decisions 5 to 8).
package company_quarters_page

import "github.com/andrewrong/optdesk/internal/compute/companydataapi"

const (
	pageName   = "company_quarters_page"
	runID      = "company_quarters"
	timeLayout = "2006-01-02T15:04:05-07:00"
	sourceSEC  = "sec"
	pathBase   = "/company-quarters"

	labelPrefix = "Quarter ending "

	epsDerivedGloss  = "annual minus the first three quarters; may differ from the reported Q4 EPS by rounding or a change in the share count"
	flowDerivedGloss = "year-to-date value minus the year-to-date value before it"
	freeCashGloss    = "operating cash flow minus capex"
)

const (
	gapUnknownCompany = "unknown_company"
	gapNoCIK          = "no_cik"
	gapJP             = "jp"
	gapNoStatements   = "no_quarters"
	gapReadFailed     = "read_failed"
)

type column struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Gloss string `json:"gloss"`
}

// columns lists the table columns; the first is the filing date of the row, the rest are the cells.
const filingGloss = "the day the report was filed with the SEC; usually a few days after the earnings release; " +
	"your own report-day record is not changed"

var columns = []column{
	{"filing_date", labelFilingDate, filingGloss},
	{"revenue", labelRevenue, "sales of the quarter as the company reports them"},
	{"operating_income", labelOperatingIncome, "profit of the quarter before interest and tax"},
	{"net_income", labelNetIncome, "profit of the quarter after every cost and tax"},
	{"eps_diluted", labelDilutedEPS, "net income per share if every share that could be issued were counted"},
	{"shares_diluted", labelDilutedShares, "the average number of shares of the quarter, counting those that could be issued"},
	{"operating_cash_flow", labelOperatingCashFlow, "cash the business itself brought in during the quarter"},
	{"capex", labelCapex, "cash spent on buildings, machines and other long-lived assets"},
	{"free_cash", labelFreeCashFlow, "operating cash flow minus capex, worked out when the page is built and stored nowhere"},
	{"buybacks", labelBuybacks, "cash the company spent buying back its own shares"},
	{"dividends", labelDividends, "cash paid out to shareholders as dividends"},
	{"cash", labelCash, "cash and cash equivalents at the end of the quarter"},
	{"long_term_debt", labelLongTermDebt, "debt due in more than a year at the end of the quarter"},
	{"short_term_debt", labelShortTermDebt, "debt due within a year at the end of the quarter"},
}

var whyTexts = map[companydataapi.Why]string{
	companydataapi.WhyNoFact:            whyNoFact,
	companydataapi.WhyCannotDerive:      whyCannotDerive,
	companydataapi.WhyPeriodMismatch:    whyPeriodMismatch,
	companydataapi.WhyShareBasisChanged: whyShareBasisChanged,
}
