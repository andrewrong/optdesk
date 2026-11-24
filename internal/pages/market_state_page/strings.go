package market_state_page

// Fixed texts of the page. Labels, axis names and units are English (the explanation texts are in
// zh_texts.go), and no word that names a state: the page reads where things
// stand and gives no verdict.
const (
	pageTitle   = "Market state"
	marketTitle = "Market"

	labelSp500Vs200d = "S&P 500 against its 200-day average"
	labelDrawdown    = "S&P 500 drawdown"
	labelVix         = "VIX"
	labelVixTerm     = "VIX against VIX3M"
	labelHySpread    = "High-yield spread"

	axisX = "63-session gap to SPY, pp"
	axisY = "21-session gap minus a third of the 63-session gap, pp"

	staleLabelPrefix = "Data day "

	smhSymbol = "SMH"
	smhLabel  = "Semiconductors"
	spySymbol = "SPY"

	unitPoints = " pp"
)

// Texts of a Gap, by reason; the subject is put in front.
const (
	gapNoReader  = "no reader wired"
	gapNoData    = "no data stored"
	gapMissing   = "an input is missing"
	gapStale     = "the newest input is stale"
	gapNoDay     = "no data day stored"
	gapNoHistory = "fewer than two daily points stored"
	gapNotRanked = "not ranked"
	gapNoClose   = "no close of the benchmark"
	gapNoPct     = "no percentile: "
)
