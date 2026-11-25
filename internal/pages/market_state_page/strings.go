package market_state_page

// Fixed texts of the page. Labels and units are English (the explanation texts are in
// zh_texts.go), and no word that names a state: the page reads where things
// stand and gives no verdict.
const (
	labelSp500Vs200d = "S&P 500 against its 200-day average"
	labelDrawdown    = "S&P 500 drawdown"
	labelNasdaq      = "Nasdaq-100 drawdown"
	labelVix         = "VIX"
	labelVixTerm     = "VIX against VIX3M"
	labelHySpread    = "High-yield spread"

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
