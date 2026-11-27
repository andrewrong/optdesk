package risk_page

// Chinese texts of the page; this file is the one door for them (cmd/ot/english_test.go).
const (
	// pageTitle names the risk of one account; the family total is on the household page.
	pageTitle = "账户风险 Account risk"

	// textNoQuoteStructures is a format: the count of structures left out for lack of a quote.
	textNoQuoteStructures = "%d 个期权组合因缺报价未计入 Delta 和 Vega；Delta 和 Vega 为其余组合之和"

	pageSentence = "Delta 和 Vega 来自最新采集；Beta 加权 Delta、波动率区间和标准风险指标尚未建成：没有任务存储 Beta、基准指数价格或风险指标。"
	vegaTitle    = "按到期日的 Vega Vega by expiry"

	titleDelta   = "按标的的 Delta Delta by underlying"
	titleBeta    = "Beta 加权 Delta Beta-weighted Delta"
	titleSigma   = "波动率区间 Volatility range"
	titleMetrics = "标准风险指标 Standard risk metrics"

	meaningDelta = "Delta：每个标的价格变动 1 美元，持仓盈亏变多少美元；股票按股数计。"
	meaningVega  = "Vega：波动率变动 1 个点，期权盈亏变多少美元。"
	meaningBeta  = "Beta 加权 Delta：把各标的的 Delta 按 Beta 折算为基准指数的 Delta。"
	meaningSigma = "波动率区间：一个和两个标准差下的盈亏范围。"

	headName     = "标的 Underlying"
	headExpiry   = "到期日 Expiry"
	headWeighted = "加权 Delta Weighted Delta"

	benchmarkText = "基准指数 Benchmark index：未存储价格"
	sigmaAria     = "波动率区间 Volatility range：尚未建成"

	textNotBuiltBeta    = "Beta 加权 Delta 尚未建成：没有任务存储各标的的 Beta 或基准指数价格"
	textNotBuiltSigma   = "波动率区间尚未建成：如何由已存储的隐含波动率 IV 定义，还没有设计"
	textNotBuiltMetrics = "标准风险指标尚未建成：没有任务存储风险指标来源"
	textMetricsMissing  = "尚未建成"
	labelMetricsMissing = "标准风险指标 Standard risk metrics"

	textNoReader   = "没有读取器：暂无数据"
	textNoCapture  = "该账户还没有采集"
	textUnknown    = "未知账户"
	textDeltaGap   = "部分 Delta 无法计算"
	textVegaGap    = "部分 Vega 无法计算"
	textNoRate     = "没有存储 USDJPY 汇率：日元期权组合的 Vega 无法折算为美元"
	textOtherKinds = "无法识别的期权组合未计入风险"
	textStale      = "最新采集太旧：暂无数据"
	textNoExpiry   = "没有到期日的期权组合未计入风险"
	textStaleLabel = "按采集日 "
	textNoVega     = "没有期权组合计入 Vega"

	// unit texts of a Delta cell, one for each currency.
	unitDeltaUSD = "美元/每 $1"
	unitDeltaJPY = "日元/每 ¥1"
	// unit text of a Vega cell and of the Vega total.
	unitVega = " 美元/每波动率 1 点 USD/vol point"

	// textTotalSeparator joins the Delta totals of the currencies in the total cell.
	textTotalSeparator = "；"
)
