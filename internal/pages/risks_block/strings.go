package risks_block

// The fixed texts of the block, in the language of the pages. The front end renders them
// as they are.
const (
	blockKey   = "risks"
	blockTitle = "Risk"
	blockSub   = "Market alerts come from stored readings, once a day for each rule."

	kindMarketAlert = "market_alert"
	iconVolatility  = "volatility"
	iconTrend       = "trend"

	// itemsGapText is the text of the Gap that stands for items when every rule lacks data.
	itemsGapText = "All risk rules: n/a"
)

// sayNone and saySome are the say line; the day comes first.
const (
	sayNone     = " No market alerts."
	saySomePre  = " "
	saySomePost = " market alerts."
)

// gapSuffix follows the rule title in the text of a Gap.
const gapSuffix = ": n/a"

// Words of data_times and of an item's detail.
const (
	newestReadingLabel = "Newest reading day "
	noReadingWord      = "none"
	alertDataLabel     = "; alert data day "
	moreWord           = " and more"
	generatedLabel     = "; generated at "
	detailDataDayOpen  = " (data day "
	detailDataDayClose = ")"
)

// iconByFigure maps a figure id to the icon key of its item; a figure not listed gets
// iconVolatility.
var iconByFigure = map[string]string{
	"market.ivr_own":        iconVolatility,
	"market.ivp_own":        iconVolatility,
	"market.vix_pctile_3y":  iconVolatility,
	"market.vxn_pctile_3y":  iconVolatility,
	"market.rvx_pctile_3y":  iconVolatility,
	"market.vix_term_ratio": iconVolatility,
	"market.sp500_vs_200d":  iconTrend,
}
