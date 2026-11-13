package market_page

// The figure ids and series ids this page reads. A test checks that each holds in the
// rules table and in the compute_figures manifest.
const (
	figVixPctile   = "market.vix_pctile_3y"
	figVxnPctile   = "market.vxn_pctile_3y"
	figRvxPctile   = "market.rvx_pctile_3y"
	figSp500Vs200d = "market.sp500_vs_200d"
	figVixTerm     = "market.vix_term_ratio"
	figIVIbkr      = "market.iv_ibkr"
	figRv20        = "market.rv20"
	figIvRv20      = "market.iv_rv20"

	kindMarket     = "market"
	kindUnderlying = "underlying"

	seriesVIX   = "VIXCLS"
	seriesVXN   = "VXNCLS"
	seriesRVX   = "RVXCLS"
	seriesVXV   = "VXVCLS"
	seriesSP500 = "SP500"
)

// dataTimeSeries is the order of data_times.
var dataTimeSeries = []string{seriesVIX, seriesVXN, seriesRVX, seriesVXV, seriesSP500}
