package market_page

// Fixed texts of the page.
const (
	pageTitle    = "Market"
	pageSentence = "Weather, percentile and term come from stored data; " +
		"the trend chart and market breadth are not built yet, " +
		"the Nikkei percentile and volatility columns have no stored series."

	weatherHeadline    = "S&P 500 against its 200-day average"
	weatherSentenceFmt = "Gap to the 200-day average: "
	weatherNoRow       = "Weather: no data yet, nothing stored"
	weatherMissing     = "Weather: n/a, input incomplete"
	weatherStale       = "Weather data is stale, data day "

	indicesTitle    = "Index volatility"
	indicesSentence = "Percentile is the index position over 3 years; the volatility columns are stored data, " +
		"implied minus realised is built for SPX, NDX and RUT, not the Nikkei."
	indicesNote = "Percentile comes from the Cboe volatility index, not from at-the-money implied volatility."

	termTitle        = "Term structure"
	termSentenceFmt  = "VIX to 3-month VIX ratio "
	termSentenceNone = "Term structure ratio: nothing stored."
	// termSentenceStale is followed by MM-DD of the points and a full stop.
	termSentenceStale = "Term structure ratio: not stored for "

	breadthNotBuilt = "Market breadth: not built yet, no matching data"
	chartNotBuilt   = "Trend chart: not built yet, only the latest gap to the 200-day average is stored, no daily line"
	ivNotBuilt      = "implied volatility: not built yet"
	ivNoRow         = "implied volatility: no data, not stored yet"
	ivMissing       = "implied volatility: no data, inputs incomplete"
	rv20NotBuilt    = "20-day realised volatility: not built yet"
	rv20NoRow       = "20-day realised volatility: no data, not stored yet"
	rv20Missing     = "20-day realised volatility: no data, inputs incomplete"
	ivRvNotBuilt    = "implied minus realised: not built yet"
	ivRvNoRow       = "implied minus realised: no data, not stored yet"
	ivRvMissing     = "implied minus realised: no data, inputs incomplete"
	// ivRvStale is followed by MM-DD of the stored data day.
	ivRvStale      = "implied minus realised data is stale, data day "
	ivRvStaleNoDay = "implied minus realised data is stale"
	n225NoData     = "Nikkei volatility: no data yet, no matching FRED series"

	pctShortHistory = "History under 3 years, no percentile"
	pctStale        = "Percentile data is stale, data day "
	pctStaleNoDay   = "Percentile data is stale"
	pctMissing      = "Percentile: n/a, input incomplete"
	pctNoRow        = "Percentile: no data yet, nothing stored"

	termNoPoint    = "No stored point"
	termStaleToday = "Latest point is stale"
	termNoOlder    = "No stored point five days ago"

	staleLabelPrefix = "Data day "
)

var (
	pageNotes = []string{
		"Percentile is given only when the stored history covers 3 years.",
		"Term structure points come from the stored series, nothing is filled in or computed.",
	}
	indicesColumns = []string{"Index", "Implied volatility", "20-day realised", "Implied minus realised", "3-year percentile"}
	termLabels     = []string{"1M", "3M"}
)
