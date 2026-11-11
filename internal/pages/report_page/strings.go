package report_page

// Fixed texts of the page. All are English constants and each is 1 to 200 characters, as the schema asks.
const (
	pageTitle   = "Report"
	pageSay     = "The risks block comes from stored readings; the other four blocks are not built yet."
	bannerLeft  = "Report"
	bannerRight = "Some content is not built yet"

	// noDataTimes is the data_times of a block that has no producer.
	noDataTimes = "no data yet"
	// chartAria is the chart_aria of global_assets.
	chartAria = "No chart data yet"
	// gapSuffix follows the block title in the text of a not_built Gap.
	gapSuffix = "not built yet"
	// reasonNotBuilt is the Gap reason of a block that has no producer.
	reasonNotBuilt = "not_built"
)

type blockText struct{ key, title, sub, say string }

var (
	textActions = blockText{"actions", "Actions", "Executed trades and pending actions.", "Actions data is not built yet."}
	textMoves   = blockText{"account_moves", "Account moves", "Net value and day move of each account.", "Account data is not built yet."}
	textWatch   = blockText{"market_watch", "Market watch", "Main indices and focus companies.", "Market watch data is not built yet."}
	textGlobal  = blockText{"global_assets", "Global assets", "Proxy assets and sector rotation.", "Global assets data is not built yet."}
)
