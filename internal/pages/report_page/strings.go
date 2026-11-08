package report_page

// Fixed texts of the page. All are Chinese constants and each is 1 to 200 characters, as the schema asks.
const (
	pageTitle   = "报告"
	pageSay     = "风险块来自已存储的读数，其余四块尚未生成。"
	bannerLeft  = "报告"
	bannerRight = "部分内容尚未生成"

	// noDataTimes is the data_times of a block that has no producer.
	noDataTimes = "暂无数据"
	// chartAria is the chart_aria of global_assets.
	chartAria = "暂无图表数据"
	// gapSuffix follows the block title in the text of a not_built Gap.
	gapSuffix = "暂未生成"
	// reasonNotBuilt is the Gap reason of a block that has no producer.
	reasonNotBuilt = "not_built"
)

type blockText struct{ key, title, sub, say string }

var (
	textActions = blockText{"actions", "操作", "已执行的交易与待办操作。", "操作数据尚未生成。"}
	textMoves   = blockText{"account_moves", "账户变动", "各账户净值与当日变动。", "账户数据尚未生成。"}
	textWatch   = blockText{"market_watch", "市场观察", "主要指数与重点公司。", "市场观察数据尚未生成。"}
	textGlobal  = blockText{"global_assets", "全球资产", "代理资产与板块轮动。", "全球资产数据尚未生成。"}
)
