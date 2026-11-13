package report_blocks

// Fixed texts of the three blocks. Each is 1 to 200 characters, as the schema asks.
const (
	keyActions   = "actions"
	keyMoves     = "account_moves"
	keyWatch     = "market_watch"
	keyGlobal    = "global_assets"
	titleActions = "Actions"
	titleMoves   = "Account moves"
	titleWatch   = "Market watch"
	subActions   = "Executed trades and pending actions."
	subMoves     = "Net value and day move of each account."
	subWatch     = "Main indices and focus companies."
	titleGlobal  = "Global assets"
	subGlobal    = "Proxy assets and sector rotation."

	sayGlobal        = "Stocks, bonds, gold, commodities and oil, and the sector funds against SPY, from stored closes."
	sayGlobalGap     = "Global assets data cannot be read now."
	dataTimesGlobal  = "Closes of "
	chartAriaGlobal  = "Day and five-day change of five proxy assets, and sector excess over SPY in percentage points"
	headingSector    = "Sector excess over SPY"
	titleRotation    = "Sector leaders"
	rotationLeaders  = "5-day leaders: "
	rotationLaggard  = "; laggard: "
	rotationLeader20 = "; 20-day leader: "
	unitPoints       = " pt"
	benchmarkSymbol  = "SPY"

	noDataTimes = "no data yet"

	sayActionsNone = "No fills today."
	sayActionsSome = "Fills today: "
	sayActionsGap  = "Actions data cannot be read now."
	sayMoves       = "One row per account, no total; margin headroom and cash after assignment show on this page only."
	sayMovesGap    = "Account data cannot be read now."
	sayWatch       = "S&P 500 comes from the stored series; focus company quotes are n/a."
	sayWatchGap    = "Market watch data cannot be read now."

	textNoReader   = "reader not connected"
	textNoCapture  = "no account data"
	textStale      = "account data is stale"
	textNoPrior    = "no prior day positions, open or close unknown"
	textManyFills  = "several fills on one day, open or close not decided"
	textMissQuote  = "needs a position quote, n/a"
	textNoSeries   = "no S&P 500 series data"
	textStaleSeria = "S&P 500 series is stale"
	textKindGap    = "open or close unknown"
	textJP         = "Japan session data not connected yet"
	textMissInput  = "input missing"
	textUnitMix    = "currencies differ"

	labelSP500 = "S&P 500"
	srcSP500   = "FRED SP500"
	seriesID   = "SP500"

	kindOpen     = "Open"
	kindClose    = "Close"
	kindExpire   = "Expiry"
	kindExercise = "Exercise"

	dataTimesPre  = "Account data day "
	dataTimesExec = "Fill day "
	dataTimesSer  = "SP500 data day "
)

var (
	columnsTrades   = []string{"Time", "Account", "Action", "Contract", "Quantity", "Price"}
	columnsAccounts = []string{"Account", "Net value", "Day amount", "Day percent", "Largest move"}
	columnsIndices  = []string{"Index", "Close", "Day change", "Five-day change"}
	columnsSector   = []string{"Sector", "ETF", "1d", "5d", "20d"}
)

// gapText is the text of a Gap by its reason; the reasons are those of a figure.
func gapText(reason string) string {
	switch reason {
	case "no_reader":
		return textNoReader
	case "no_data":
		return textNoCapture
	case "stale_input":
		return textStale
	case "missing_quote":
		return textMissQuote
	case "unsupported_currency":
		return textUnitMix
	case "jp_slice_missing":
		return textJP
	default:
		return textMissInput
	}
}
