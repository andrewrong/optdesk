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
	missingGeneric   = "No value is stored for this quarter."

	noteNoLabel = "Fiscal-year labels (for example FY2025 Q3) appear after the next daily run of fundamentals " +
		"re-fetches the SEC filings; until then a quarter is named by its end date."
)

const (
	gapUnknownCompany = "unknown_company"
	gapNoCIK          = "no_cik"
	gapJP             = "jp"
	gapNoStatements   = "no_quarters"
	gapReadFailed     = "read_failed"

	textUnknownCompany = "No company is chosen or the name is not on the list; pick one below."
	textNoCIK          = "This company has no SEC number (cik); type it on the underlyings page, then its quarters can be shown."
	textJP             = "Statements of Japanese companies are left out on purpose; no source is wired for them."
	textNoStatements   = "No quarter could be derived from the stored SEC filings yet."
	textReadFailed     = "The stored values could not be read, so no quarter is shown."
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
	companydataapi.WhyNoFact:            "No filing reported this figure for the quarter.",
	companydataapi.WhyCannotDerive:      "The year-to-date figures needed to work out this quarter are not all stored.",
	companydataapi.WhyPeriodMismatch:    "The fiscal period named in the filing does not match its dates, so the quarter is not guessed.",
	companydataapi.WhyShareBasisChanged: "The share count changed (for example a split), so the quarter is not worked out.",
}
