package risks_block

// The fixed texts of the block, in the language of the pages. The front end renders them
// as they are.
const (
	blockKey   = "risks"
	blockTitle = "风险"
	blockSub   = "市场提醒来自已存储的读数，每条规则每天一次。"

	kindMarketAlert = "market_alert"
	iconVolatility  = "volatility"
	iconTrend       = "trend"

	// itemsGapText is the text of the Gap that stands for items when every rule lacks data.
	itemsGapText = "风险规则全部暂缺"
)

// sayNone and saySome are the say line; the day comes first.
const (
	sayNone     = " 没有市场提醒。"
	saySomePre  = " 有 "
	saySomePost = " 条市场提醒。"
)

// gapSuffix follows the rule title in the text of a Gap.
const gapSuffix = " 暂缺"

// Words of data_times and of an item's detail.
const (
	newestReadingLabel = "最新读数日 "
	noReadingWord      = "无"
	alertDataLabel     = "；提醒数据日 "
	generatedLabel     = "；生成于 "
	detailDataDayOpen  = "（数据日 "
	detailDataDayClose = "）"
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
