package market_state_page

// Chinese texts of the chart and the volatility block.
const (
	// chartLegend is the caption of price_chart: what the three drawn lines are.
	chartLegend = "黄线为 200 日简单均线，阴影为布林带（20 日，2 倍标准差，总体标准差）。"

	// chartAriaFmt takes the number of days, the first day and the last day.
	chartAriaFmt = "标普 500 收盘价、200 日均线与布林带，共 %d 天，%s 至 %s。"

	volGapTitle = "隐含波动率减已实现波动率"
	volGapHint  = "数值为隐含波动率（VIX、VXN、RVX 除以 100）减 20 日已实现波动率（21 个收盘价的对数收益，样本标准差，乘以 252 的平方根），单位为波动率点。正数表示期权定价高于实际波动，负数表示低于实际波动。"

	volGapDayPrefix = "数据日 "
	volGapNoRow     = "隐含减已实现：暂无数据，尚未存储"
	volGapMissing   = "隐含减已实现：输入不全"
	volGapStale     = "隐含减已实现：数据已过期"
	volGapNoReader  = "隐含减已实现：未接入读取器"
)
