package household_page

// The words of the page, written once.
const (
	pageTitle    = "Household"
	pageSentence = "Net value, cash and option obligations come from the latest capture; position values need " +
		"prices, and positions without a price show no data yet."

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
	titleLongTerm      = "Long-term holdings"
	titleAccounts      = "Account"

	meaningOverview      = "Net liquidation of the account, from the latest capture."
	meaningComposition   = "Market value of held stocks at the capture-day price and their share of net value."
	meaningMovers        = "Which positions drove the change today; needs position prices."
	meaningTrend         = "Net liquidation on recent trading days, one point per trading day."
	meaningStress        = "Cash after all short puts are assigned, and each underlying-decline step."
	meaningIncome        = "Premium collected on short puts this month."
	meaningMonthChange   = "Net liquidation from the last trading day of the previous month to the latest capture."
	meaningCashAfter     = "Cash minus the cash needed if all short puts are assigned."
	meaningObligations   = "Cash to pay if each open short put is assigned."
	meaningConcentration = "Share of net value held in a single underlying, at the capture-day price."
	meaningAllocation    = "Market value of held stocks at the capture-day price."
	meaningLongTerm      = "Long-term positions; needs position prices."
	meaningAccounts      = "Net liquidation, cash, stocks and options of this account."

	textNoMovers     = "The change today needs prices of two trading days: no data yet"
	textNotBuilt     = "No holding-period source: no data yet"
	nameStock        = "Stock"
	textMissingQuote = "Needs position prices: no data yet"
	textMonthSteps   = "The trading result needs position prices, so the steps show no data yet; only month start and month end are listed"
	textNoRate       = "%s: no %s rate on or before %s"
	textRateDay      = " JPY amounts use the USDJPY rate of %s."
	textNoReader     = "No reader: no data yet"
	textUnknown      = "Unknown account"
	textNoCapture    = "This account has no capture yet"
	textStale        = "The latest capture is over 3 trading days old"
	textStaleLabel   = "Account data day "

	textNoMonthStart = "No capture of net liquidation on the last trading day before %s: the month start is missing"
	textNotInLedger  = "%d short put(s) in stored positions are not in the option ledger yet (no Flex record): %s"

	nameMonthStart = "Month start"
	nameMonthEnd   = "Month end"
	whoseMain      = "Me"
	whoseOther     = "Other account"
	labelAccount   = "Account ("
	labelClose     = ")"
	labelDay       = "last "
	labelDays      = " trading days"
)

var (
	headsComposition   = []string{"Position", "Amount", "Share"}
	headsConcentration = []string{"Company", "Share of household net value", "Market value"}
)

var accountColumns = []string{"Account", "Net liquidation", "Cash", "Stocks", "Options", "Data day"}

// gapTexts are the words of each gap reason a line of pagefig can carry.
var gapTexts = map[string]string{
	"missing_quote":        textMissingQuote,
	"missing_input":        "Input data is missing: no data yet",
	"unsupported_currency": "Exchange rate is missing or currency is unsupported: no data yet",
	"no_data":              "No data: no data yet",
	"not_built":            textNotBuilt,
	"no_reader":            textNoReader,
	"stale_input":          textStale,
}

func gapText(reason string) string {
	if t, ok := gapTexts[reason]; ok {
		return t
	}
	return "no data yet"
}
