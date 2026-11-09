package market_page

// Fixed texts of the page.
const (
	pageTitle    = "市场"
	pageSentence = "天气来自标普 500 相对 200 日线的位置；分位和期限来自已存储的数据，其余项暂未生成。"

	weatherHeadline    = "标普 500 相对 200 日线的位置"
	weatherSentenceFmt = "与 200 日线相差 "
	weatherNoRow       = "天气暂无数据：还没有存储"
	weatherMissing     = "天气暂缺：输入不全"
	weatherStale       = "天气数据已过期，数据日 "

	indicesTitle    = "指数波动率"
	indicesSentence = "分位是指数在近 3 年中的位置；其余列暂未生成。"
	indicesNote     = "分位来自 Cboe 波动率指数，不是平值隐含波动率。"

	termTitle        = "期限结构"
	termSentenceFmt  = "VIX 与 3 个月 VIX 之比 "
	termSentenceNone = "期限结构比值暂无存储。"

	breadthNotBuilt = "市场宽度暂未生成：没有对应的数据"
	chartNotBuilt   = "走势图暂未生成：没有存储的 200 日线"
	ivNotBuilt      = "隐含波动率暂未生成：还没有存储"
	rv20NotBuilt    = "20 日已实现波动率暂未生成"
	ivRvNotBuilt    = "隐含减已实现暂未生成"
	n225NoData      = "日经波动率暂无数据：没有对应的 FRED 序列"

	pctShortHistory = "历史不满 3 年，不给分位"
	pctStale        = "分位数据已过期，数据日 "
	pctStaleNoDay   = "分位数据已过期"
	pctMissing      = "分位暂缺：输入不全"
	pctNoRow        = "分位暂无数据：还没有存储"

	termNoPoint    = "暂无存储的点"
	termStaleToday = "最新点已过期"
	termNoOlder    = "五天前暂无存储的点"

	staleLabelPrefix = "数据日 "
)

var (
	pageNotes = []string{
		"分位只在存储的历史满 3 年后给出。",
		"期限结构的点取自已存储的序列，不补不算。",
	}
	indicesColumns = []string{"指数", "隐含波动率", "20 日已实现", "隐含减已实现", "3 年分位"}
	termLabels     = []string{"1M", "3M"}
)
