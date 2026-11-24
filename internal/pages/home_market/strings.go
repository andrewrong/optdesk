package home_market

// Fixed texts of the card.
const (
	cardTitle = "Market"

	ivSource        = "IBKR implied volatility"
	rv20NoRow       = "20-day realised volatility: no data, not stored yet"
	rv20Missing     = "20-day realised volatility: no data, inputs incomplete"
	levelNoRow      = " no data: not stored yet"
	levelStale      = " data is stale, data day "
	levelBad        = " data unusable"
	indexIvLabel    = "Index IV"
	indexIvNoRow    = " index IV: no data, not stored yet"
	indexIvStale    = " index IV: data is stale, data day "
	indexIvBad      = " index IV: data unusable"
	indexIvUnmapped = " index IV: no volatility index is mapped (SPX, NDX and RUT only)"
)

var (
	columns     = []string{"Index", "Implied volatility", "20-day realised", "Difference"}
	seriesNames = []string{"Close", "200-day average", "Range"}
	notes       = []string{"The chart draws only the stored S&P 500 closes; the 200-day average and range are not built yet."}
)
