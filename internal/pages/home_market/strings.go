package home_market

// Fixed texts of the card.
const (
	cardTitle    = "Market"
	cardSubtitle = "Index trend, implied volatility and the Cboe Volatility Index"
	sayVix       = "VIX closed at "
	sayNone      = "VIX: no data available"

	ivNoRow          = "Implied volatility: no data, not stored yet"
	ivMissing        = "Implied volatility: no data, inputs incomplete"
	ivSource         = "IBKR implied volatility"
	rv20NoRow        = "20-day realised volatility: no data, not stored yet"
	rv20Missing      = "20-day realised volatility: no data, inputs incomplete"
	ivRvNoRow        = "Implied minus realised: no data, not stored yet"
	ivRvMissing      = "Implied minus realised: no data, inputs incomplete"
	ivRvStaleText    = "Implied minus realised: data is stale"
	levelNoRow       = " no data: not stored yet"
	levelStale       = " data is stale, data day "
	levelBad         = " data unusable"
	chartNoData      = "Chart: no data, no stored S&P 500 closes"
	ariaNone         = "Chart: no data"
	ariaFmt          = "S&P 500 close trend, %d trading days, from %s to %s"
	noDay            = "no data"
	noDataText       = "no data yet"
	noDataTimes      = "no data time yet"
	staleLabelPrefix = "Data day "
	dataTimesPrefix  = "Data time "
	volNote          = "Index levels are in points as published by Cboe, not merged into the table."
)

var (
	columns     = []string{"Index", "Implied volatility", "20-day realised", "Difference"}
	seriesNames = []string{"Close", "200-day average", "Range"}
	notes       = []string{"The chart draws only the stored S&P 500 closes; the 200-day average and range are not built yet."}
)
