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
	// volGapSameDay is the day text of a figure whose data day is the page day: the day is not repeated.
	volGapSameDay  = "当日数据"
	volGapNoRow    = "隐含减已实现：暂无数据，尚未存储"
	volGapMissing  = "隐含减已实现：输入不全"
	volGapStale    = "隐含减已实现：数据已过期"
	volGapNoReader = "隐含减已实现：未接入读取器"
)

// Chinese explanation texts of the indicator rows, the sector table and the freshness line.
const (
	windowSp500Vs200d = "水平：FRED 最新收盘价对比最近 200 个收盘价的均值"
	windowDrawdown    = "60 个交易日水平：最新收盘价对比最近 60 个收盘价中的最高值"
	windowVix         = "VIX 最新收盘价；分位覆盖最近 3 个日历年"
	windowVixTerm     = "水平：当日 VIX 除以 VIX3M；1.00 表示两者相等"
	windowHySpread    = "FRED 最新收盘值；分位覆盖已存储的全部日期"

	pctWindowVix = "当日之前的 3 个日历年，不含当日；严格低于当日数值的天数占比"
	// vixPctOtherDayFmt takes the data day of the stored percentile and the day of the level.
	vixPctOtherDayFmt = "暂无分位：已存储的分位对应 %s，与 VIX 水平的数据日 %s 不是同一天"
	vixPctDayUnknown  = "日期未知"

	pctWindowHy = "已存储的全部日期，自第一个存储日起；FRED 约提供 3 年数据"

	sectorCaption = "基金涨跌幅减 SPY 涨跌幅，窗口为截至所示终止日的 21 个与 63 个交易日；排名只计两个收盘价都有的基金；为试验窗口，不是预测。"
	barsCaption   = "柱表示各基金相对 SPY 的 63 日差距，顺序同表格，单位为 pp；没有数值的基金没有柱。"
	// rotationNote names both windows and the tail offsets.
	rotationNote = "x 为相对 SPY 的 63 日差距，y 为 21 日差距减 63 日差距的三分之一；尾迹点为 end_day 之前 5、10、15、20 个交易日；表示位置，不是预测；点击某只基金的圆点可绘制其尾迹"

	freshnessHead = "行业基金与 SPY：日收盘价，于纽约证券交易所（NYSE）收盘后 10 分钟存储，最近收盘日 "
	freshnessNone = "尚未存储"
	freshnessTail = "；非盘中数据。标普 500、VIX、VIX3M 与高收益利差各行：FRED 日度序列，于纽约时间 07:30 存储，每行标明各自的数据日。"
)
