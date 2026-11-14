package market_state_page

// Fixed texts of the page. English, and no word that names a state: the page reads where things
// stand and gives no verdict.
const (
	pageTitle   = "Market state"
	marketTitle = "Market"

	labelSp500Vs200d = "S&P 500 against its 200-day average"
	labelDrawdown    = "S&P 500 drawdown"
	labelVix         = "VIX"
	labelVixTerm     = "VIX against VIX3M"
	labelHySpread    = "High-yield spread"

	windowSp500Vs200d = "Level: newest FRED close against the mean of the newest 200 closes"
	windowDrawdown    = "Level over 60 sessions: newest close against the highest of the newest 60 closes"
	windowVix         = "Newest VIX close; the percentile covers the last 3 calendar years"
	windowVixTerm     = "Level: same-day VIX divided by VIX3M; 1.00 means equal"
	windowHySpread    = "Newest FRED close; the percentile covers all days stored"

	pctWindowVix = "3 calendar years before the day, the day excluded; the share of days strictly below"
	pctWindowHy  = "all days stored, since the first stored day; FRED serves about 3 years"

	sectorCaption = "Fund change minus SPY change over 21 and 63 sessions ending on the end day shown; " +
		"a rank counts only funds with both closes; trial windows, not a forecast."
	barsCaption = "Bars show the 63-session gap to SPY of each fund in the table's order, in pp; " +
		"a fund with no figure has no bar."
	axisX = "63-session gap to SPY, pp"
	axisY = "21-session gap minus a third of the 63-session gap, pp"
	// rotationNote names both windows and the tail offsets.
	rotationNote = "x is the 63-session gap to SPY, y the 21-session gap minus a third of the 63-session gap; " +
		"tail points are 5, 10, 15 and 20 sessions before end_day; position, not a forecast"

	freshnessHead = "Sector funds and SPY: daily closes stored 10 minutes after the NYSE close, last close "
	freshnessNone = "none stored"
	freshnessTail = "; not intraday. S&P 500, VIX, VIX3M and high-yield rows: daily FRED series stored at " +
		"07:30 New York, each row shows its own day."

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
