package risk_page

// The words of the page, written once.
const (
	pageTitle    = "Household risk"
	pageSentence = "Delta and Vega come from the latest capture; Beta-weighted Delta, the volatility range and the standard metrics are not built: " +
		"no job stores Beta, the index price or risk metrics."
	vegaTitle = "Vega by expiry"

	titleDelta   = "Delta by underlying"
	titleBeta    = "Beta-weighted Delta"
	titleSigma   = "Volatility range"
	titleMetrics = "Standard risk metrics"

	meaningDelta = "How many dollars the result of the positions changes when each underlying moves one dollar; stocks count by shares."
	meaningVega  = "How many dollars the result of the option structures changes when volatility moves one point."
	meaningBeta  = "Delta of each underlying converted to the benchmark index by Beta."
	meaningSigma = "Result range at one and two standard deviations."

	headName     = "Underlying"
	headDelta    = "Delta"
	headExpiry   = "Expiry"
	headVega     = "Vega"
	headBeta     = "Beta"
	headWeighted = "Weighted Delta"

	benchmarkText = "Benchmark index: no price stored"
	sigmaAria     = "Volatility range: not built"

	textNotBuiltBeta    = "Beta-weighted Delta is not built: no job stores the Beta of each underlying or the benchmark index price"
	textNotBuiltSigma   = "The volatility range is not built: how it is defined from the stored IV is not designed yet"
	textNotBuiltMetrics = "The standard risk metrics are not built: no job stores a risk metrics source"
	textMetricsMissing  = "not built"
	labelMetricsMissing = "Standard risk metrics"

	textNoReader   = "No reader: no data yet"
	textNoCapture  = "This account has no capture yet"
	textUnknown    = "Unknown account"
	textDeltaGap   = "Some Delta cannot be computed"
	textVegaGap    = "Some Vega cannot be computed"
	textForeign    = "Non-USD option structures are not counted"
	textOtherKinds = "Unrecognised option structures are not counted in risk"
	textStale      = "The latest capture is too old: no data yet"
	textNoExpiry   = "Option structures without an expiry are not counted in risk"
	textStaleLabel = "By capture day "
	unitDelta      = " USD per $1 move"
	textNoVega     = "No option structure is counted in Vega"
	unitVega       = " USD/vol point"
)
