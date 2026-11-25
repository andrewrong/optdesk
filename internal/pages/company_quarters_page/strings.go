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
	{"filing_date", "Filing date", filingGloss},
	{"revenue", "Revenue", "sales of the quarter as the company reports them"},
	{"operating_income", "Operating income", "profit of the quarter before interest and tax"},
	{"net_income", "Net income", "profit of the quarter after every cost and tax"},
	{"eps_diluted", "Diluted EPS", "net income per share if every share that could be issued were counted"},
	{"shares_diluted", "Diluted shares", "the average number of shares of the quarter, counting those that could be issued"},
	{"operating_cash_flow", "Operating cash flow", "cash the business itself brought in during the quarter"},
	{"capex", "Capex", "cash spent on buildings, machines and other long-lived assets"},
	{"free_cash", "Free cash flow", "operating cash flow minus capex, worked out when the page is built and stored nowhere"},
	{"buybacks", "Buybacks", "cash the company spent buying back its own shares"},
	{"dividends", "Dividends", "cash paid out to shareholders as dividends"},
	{"cash", "Cash", "cash and cash equivalents at the end of the quarter"},
	{"long_term_debt", "Long-term debt", "debt due in more than a year at the end of the quarter"},
	{"short_term_debt", "Short-term debt", "debt due within a year at the end of the quarter"},
}

var whyTexts = map[companydataapi.Why]string{
	companydataapi.WhyNoFact:            "没有任何财报披露这个季度的这项数据。",
	companydataapi.WhyCannotDerive:      "推算这个季度所需的年初至今数据没有全部存下。",
	companydataapi.WhyPeriodMismatch:    "财报标注的财政期间与它的日期对不上，所以不去猜这个季度。",
	companydataapi.WhyShareBasisChanged: "股数发生了变化（例如拆股），所以不推算这个季度。",
}
