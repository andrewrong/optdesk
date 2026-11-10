package risk_page

// The words of the page, written once.
const (
	pageTitle    = "家庭风险"
	pageSentence = "Delta 和 Vega 来自最近一次采集的持仓与期权结构；Beta、波动区间和标准指标的数据源还没有接入。"
	vegaTitle    = "按到期日的 Vega"

	titleDelta   = "按标的的 Delta"
	titleBeta    = "Beta 加权 Delta"
	titleSigma   = "波动区间"
	titleMetrics = "标准风险指标"

	meaningDelta = "每个标的涨跌一美元，持仓的盈亏变化多少美元；股票按股数计。"
	meaningVega  = "波动率变化一个点，期权结构的盈亏变化多少美元。"
	meaningBeta  = "各标的 Delta 按 Beta 折算到基准指数。"
	meaningSigma = "一倍、两倍标准差下的盈亏范围。"

	headName     = "标的"
	headDelta    = "Delta"
	headExpiry   = "到期日"
	headVega     = "Vega"
	headBeta     = "Beta"
	headWeighted = "加权 Delta"

	benchmarkText = "基准指数暂缺"
	sigmaAria     = "波动区间暂缺"

	textNotBuiltBeta    = "Beta 加权 Delta 需要各标的的 Beta 和基准价格，数据源还没有接入"
	textNotBuiltSigma   = "波动区间需要标的的隐含波动率，数据源还没有接入"
	textNotBuiltMetrics = "标准风险指标需要风险指标数据源，还没有接入"
	textMetricsMissing  = "暂缺"
	labelMetricsMissing = "标准风险指标"

	textNoReader   = "没有读取器，暂缺"
	textNoCapture  = "这个账户还没有采集"
	textUnknown    = "未知账户"
	textDeltaGap   = "部分 Delta 无法计算"
	textVegaGap    = "部分 Vega 无法计算"
	textForeign    = "有非美元的期权结构，未计入"
	textOtherKinds = "有未识别的期权结构，未计入风险"
	textStale      = "最近一次采集太旧，暂缺"
	textNoExpiry   = "有没有到期日的期权结构，未计入风险"
	textStaleLabel = "按采集日 "
	unitDelta      = " 美元/美元"
	unitVega       = " 美元/波动点"
)
