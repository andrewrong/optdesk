package household_page

// The words of the page, written once.
const (
	pageTitle    = "家庭资产"
	pageSentence = "净值、现金和期权义务来自最近一次采集；持仓估值需要价格，暂缺。"

	titleOverview      = "总览"
	titleComposition   = "持仓构成"
	titleMovers        = "谁带动了今天"
	titleTrend         = "净值走势"
	titleStress        = "压力测试"
	titleIncome        = "每月收入"
	titleMonthChange   = "月度资产变化"
	titleCashAfter     = "指派后现金"
	titleObligations   = "期权义务"
	titleConcentration = "集中度"
	titleAllocation    = "资产配置"
	titleLongTerm      = "长期持有"
	titleAccounts      = "账户"

	meaningOverview      = "账户的净清算值，来自最近一次采集。"
	meaningComposition   = "按持仓价格计的构成，价格暂缺。"
	meaningMovers        = "当天变化来自哪些持仓，需要持仓价格。"
	meaningTrend         = "最近交易日的净清算值，每个交易日一个点。"
	meaningStress        = "全部卖出认沽被指派后的现金，以及标的下跌各档。"
	meaningIncome        = "本月卖出认沽已收权利金。"
	meaningMonthChange   = "净清算值从上月最后一个交易日到最近一次采集。"
	meaningCashAfter     = "现金减去全部卖出认沽被指派所需的现金。"
	meaningObligations   = "每一笔未平仓卖出认沽被指派时要付的现金。"
	meaningConcentration = "单一标的占比，需要持仓价格。"
	meaningAllocation    = "资产类别配置，需要持仓价格。"
	meaningLongTerm      = "长期持有的持仓，需要持仓价格。"
	meaningAccounts      = "这个账户的净清算值、现金、股票和期权。"

	textMissingQuote = "需要持仓价格，暂缺"
	textMonthSteps   = "交易结果需要持仓价格，各步骤暂缺；只列月初和月末"
	textNoReader     = "没有读取器，暂缺"
	textUnknown      = "未知账户"
	textNoCapture    = "这个账户还没有采集"
	textStale        = "最近一次采集超过 3 个交易日"
	textStaleLabel   = "账户数据日 "

	nameMonthStart = "月初"
	nameMonthEnd   = "月末"
	whoseMain      = "本人"
	whoseOther     = "其他账户"
	labelAccount   = "账户（"
	labelClose     = "）"
	labelDay       = "最近"
	labelDays      = "个交易日"
)

var accountColumns = []string{"账户", "净清算值", "现金", "股票", "期权", "数据日"}

// gapTexts are the words of each gap reason a line of pagefig can carry.
var gapTexts = map[string]string{
	"missing_quote":        textMissingQuote,
	"missing_input":        "缺少输入数据，暂缺",
	"unsupported_currency": "缺少汇率或币种不支持，暂缺",
	"no_data":              "没有数据，暂缺",
	"no_reader":            textNoReader,
	"stale_input":          textStale,
}

func gapText(reason string) string {
	if t, ok := gapTexts[reason]; ok {
		return t
	}
	return "暂缺"
}
