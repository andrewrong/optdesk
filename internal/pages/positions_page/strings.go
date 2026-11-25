package positions_page

// The words of the page, written once.
const (
	labelClose = ")"

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

	textBehindStart = "Option ledger is behind the capture; open in the ledger, not held: "

	sentenceStocks    = "Stocks "
	sentenceRows      = " rows, options "
	sentenceLegs      = " legs."
	sentenceNothing   = "No positions."
	textStatementNote = "; this account has no live gateway."
	textStatementFrom = "From the broker statement of "
	textBarStart      = "Last price of "
	textBarMid        = " is the last stored close (last trading price of the day) of "
	textBarEnd        = "; a day still open may change"
	textClosedStart   = "No day result for "
	textClosedMid     = ": the market was closed on "
	textNoHeld        = "No positions"

	unitContract = " contracts"
	unitShare    = " shares"
	perShare     = "/share"
)

var (
	stockHeads = []string{
		"code:" + headCode,
		"name:" + headName,
		"role:" + headOwner,
		"quantity:" + headQuantity,
		"cost:" + headCost,
		"price:" + headPrice,
		"day:" + headDay,
		"unrealised:" + headUnrealised,
	}
	optionHeads = []string{"code:" + headCode, "name:" + headName, "expiry:" + headExpiry, "days_left:" + headDaysLeft, "role:" + headOwner, "quantity:" + headQuantity,
		"cost:" + headCost, "price:" + headPrice, "day:" + headDay, "unrealised:" + headUnrealised, "open_premium:" + headOpenPremium}
	tradeHeads = []string{"time:" + headTime, "account:" + headAccount, "action:" + headAction, "instrument:" + headInstrument, "quantity:" + headQuantity, "price:" + headTradePrice}
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
