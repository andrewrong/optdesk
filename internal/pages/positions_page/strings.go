package positions_page

// The words of the page, written once.
const (
	pageTitle = "Positions"

	titleStocks  = "Stocks"
	titleOptions = "Options"
	titleTrades  = "Recent trades"

	labelAccount = "Account ("
	labelClose   = ")"

	textNoReader  = "No reader: no data yet"
	textUnknown   = "Unknown account"
	textNoCapture = "This account has no positions captured yet"
	textStale     = "The latest capture is over 3 trading days old"
	textStaleData = "Account data day "

	textNoMark       = "No price: no data yet"
	textNA           = "n/a"
	textDayProfit    = "Day result needs the previous trading day price: no data yet"
	textNoLedger     = "Not in the option ledger: no data yet"
	textAmbiguous    = "Several entries in the option ledger: no data yet"
	textBadExpiry    = "Expiry cannot be read: no data yet"
	textBadNumber    = "Number cannot be read: no data yet"
	textCurrency     = "Currency not supported: no data yet"
	textNoTime       = "Fill time unknown"
	textNoPrior      = "No earlier capture: action no data yet"
	textManyFills    = "More than one fill of the contract that day: action no data yet"
	textNoTrades     = "No trades in the last 30 trading days"
	textSkippedStart = "Skipped: "
	textSkippedEnd   = " rows whose quantity cannot be shown are not listed"

	sentenceStocks    = "Stocks "
	sentenceRows      = " rows, options "
	sentenceLegs      = " legs."
	sentenceNothing   = "No positions."
	textStatementNote = "; this account has no live gateway."
	textStatementFrom = "From the broker statement of "
	textNoHeld        = "No positions"

	actionSellOpen  = "Sell to open"
	actionBuyOpen   = "Buy to open"
	actionSellClose = "Sell to close"
	actionBuyClose  = "Buy to close"
	actionExpire    = "Expired"
	actionExercise  = "Exercised"

	unitContract = " contracts"
	unitShare    = " shares"
	perShare     = "/share"

	sellPut  = "Sell put"
	buyPut   = "Buy put"
	sellCall = "Sell call"
	buyCall  = "Buy call"
)

var (
	stockHeads = []string{
		"code:Code",
		"name:Name",
		"role:Owner",
		"quantity:Quantity",
		"cost:Cost (per share)",
		"price:Last price (per share)",
		"day:Day result",
		"unrealised:Unrealised result",
	}
	optionHeads = []string{"code:Code", "name:Name", "expiry:Expiry", "days_left:Days left", "role:Owner", "quantity:Quantity",
		"cost:Cost (per share)", "price:Last price (per share)", "day:Day result", "unrealised:Unrealised result", "open_premium:Open premium"}
	tradeHeads = []string{"time:Time", "account:Account", "action:Action", "instrument:Underlying · structure", "quantity:Quantity", "price:Price"}
)

// rightAligned are the columns of numbers.
var rightAligned = map[string]bool{"quantity": true, "cost": true, "price": true, "day": true, "unrealised": true, "days_left": true, "open_premium": true}

// gapTexts are the words of each gap reason a line of pagefig can carry.
var gapTexts = map[string]string{
	"missing_quote":        textNoMark,
	"missing_input":        textBadNumber,
	"unsupported_currency": textCurrency,
	"no_data":              "No data: no data yet",
	"no_reader":            textNoReader,
	"stale_input":          textStale,
}

func gapText(reason string) string {
	if t, ok := gapTexts[reason]; ok {
		return t
	}
	return "no data yet"
}
