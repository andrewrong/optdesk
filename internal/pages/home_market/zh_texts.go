// zh_texts.go holds the Chinese hint of the Index IV column (docs/design/market-iv-rv20.md, Decision 15):
// string constants only, the one door of the English guard of cmd/ot.
package home_market

const indexIvHint = "指数隐含波动率：标普500用VIX，纳斯达克100用VXN，罗素2000用RVX，取FRED已存收盘值（不是期权实时报价）；标签里是数据日。"

// The Chinese words of the card. The names VIX, NDX, SPX, IV, S&P 500, Cboe and IBKR stay as written.
const (
	cardSubtitle = "指数走势、隐含波动率和 Cboe 波动率指数"
	sayVix       = "VIX 收于 "
	sayVixDay    = "，数据日 "
	sayNone      = "VIX：暂无数据"

	ivNoRow          = "隐含波动率：无数据，尚未存入"
	ivMissing        = "隐含波动率：无数据，输入不全"
	ivRvNoRow        = "隐含减已实现：无数据，尚未存入"
	ivRvMissing      = "隐含减已实现：无数据，输入不全"
	ivRvStaleText    = "隐含减已实现：数据已过期"
	ivRvStaleDay     = "，数据日 "
	chartNoData      = "图表：无数据，没有已存的 S&P 500 收盘价"
	ariaNone         = "图表：无数据"
	ariaFmt          = "S&P 500 收盘走势，%d 个交易日，从 %s 到 %s"
	noDay            = "无数据"
	noDataText       = "暂无数据"
	noDataTimes      = "暂无数据时间"
	staleLabelPrefix = "数据日 "
	dataTimesPrefix  = "数据时间 "
	volNote          = "指数点位按 Cboe 公布的点数显示，不并入表格。"

	numbersHead = "NDX 减 SPX：近 1 个月 "
	numbersMid  = " 点，近 3 个月 "
	numbersTail = " 点；试行阈值，不是预测"

	techNoData  = "科技状态：无数据，尚未存入"
	techStale   = "科技状态：数据已过期"
	techMissing = "科技状态：无数据，输入不全"
)
