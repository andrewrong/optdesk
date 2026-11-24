package household_page

// The words of the page, written once.
const (
	pageTitle = "Household"

	// The words of the two header links, to /risk and /positions; the page draws them as given.
	linkPositions = "Positions"

	titleOverview      = "Overview"
	titleComposition   = "Holdings composition"
	titleMovers        = "What moved today"
	titleTrend         = "Net value trend"
	titleStress        = "Stress test"
	titleIncome        = "Monthly income"
	titleMonthChange   = "Monthly change"
	titleCashAfter     = "Cash after assignment"
	titleObligations   = "Option obligations"
	titleConcentration = "Concentration"
	titleAllocation    = "Asset allocation"
	titleAccounts      = "Account"

	nameMoversTotal = "Change in net liquidation"
	nameMoversOther = "Other"
	headName        = "Name"
	headPrevious    = "Previous (%s)"
	headLast        = "Last (%s)"
	headMoved       = "Moved (USD)"
	nameStock       = "Stock"

	nameMonthStart = "Month start"
	nameMonthEnd   = "Month end"
	whoseMain      = "Me"
	whoseOther     = "Other account"
)

var (
	headsComposition   = []string{"Position", "Amount", "Share"}
	headsConcentration = []string{"Company", "Share of household net value", "Market value"}
)

var accountColumns = []string{"Account", "Net liquidation", "Cash", "Stocks", "Options", "Data day"}

// gapTexts are the words of each gap reason a line of pagefig can carry.
var gapTexts = map[string]string{
	"missing_quote":        textMissingQuote,
	"missing_input":        textMissingInput,
	"unsupported_currency": textUnsupportedCurrency,
	"no_data":              textNoData,
	"not_built":            textNotBuilt,
	"no_reader":            textNoReader,
	"stale_input":          textStale,
}

func gapText(reason string) string {
	if t, ok := gapTexts[reason]; ok {
		return t
	}
	return textNoDataYet
}
