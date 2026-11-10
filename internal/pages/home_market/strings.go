package home_market

// Fixed texts of the card.
const (
	cardTitle    = "市场"
	cardSubtitle = "指数走势、隐含波动率和 Cboe 波动率指数"
	sayVix       = "VIX 收在 "
	sayNone      = "VIX 暂无可用数据"

	ivNoRow          = "隐含波动率暂无数据：还没有存储"
	ivMissing        = "隐含波动率暂缺：输入不全"
	ivSource         = "IBKR 隐含波动率"
	rv20NotBuilt     = "20 日已实现波动率暂未生成"
	diffNotBuilt     = "隐含减已实现暂未生成"
	levelNoRow       = " 暂无数据：还没有存储"
	levelStale       = " 数据已过期，数据日 "
	levelBad         = " 数据不可用"
	chartNoData      = "走势图暂无数据：没有存储的标普 500 收盘"
	ariaNone         = "走势图暂无数据"
	ariaFmt          = "标普 500 收盘走势，共 %d 个交易日，从 %s 到 %s"
	noDay            = "无数据"
	noDataText       = "暂无数据"
	noDataTimes      = "暂无数据时间"
	staleLabelPrefix = "数据日 "
	dataTimesPrefix  = "数据时间 "
	volNote          = "指数水平以 Cboe 发布的点数计，不并入表格。"
)

var (
	columns     = []string{"指数", "隐含波动率", "20 日已实现", "差值"}
	seriesNames = []string{"收盘", "200 日均线", "区间"}
	notes       = []string{"走势图只画标普 500 的存储收盘，200 日均线和区间暂未生成。"}
)
