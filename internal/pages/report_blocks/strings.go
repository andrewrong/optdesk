package report_blocks

// Fixed texts of the three blocks. Each is 1 to 200 characters, as the schema asks.
const (
	keyActions   = "actions"
	keyMoves     = "account_moves"
	keyWatch     = "market_watch"
	titleActions = "操作"
	titleMoves   = "账户变动"
	titleWatch   = "市场观察"
	subActions   = "已执行的交易与待办操作。"
	subMoves     = "各账户净值与当日变动。"
	subWatch     = "主要指数与重点公司。"

	noDataTimes = "暂无数据"

	sayActionsNone = "当日没有成交。"
	sayActionsSome = "当日成交笔数："
	sayActionsGap  = "操作数据暂时无法读取。"
	sayMoves       = "每个账户一行，不合计；保证金余量与指派后现金只在本页显示。"
	sayMovesGap    = "账户数据暂时无法读取。"
	sayWatch       = "标普500来自已存储的序列，重点公司暂缺报价。"
	sayWatchGap    = "市场观察数据暂时无法读取。"

	textNoReader   = "读取器未接入"
	textNoCapture  = "没有账户数据"
	textStale      = "账户数据已过期"
	textNoPrior    = "没有上一日持仓，无法判断开平"
	textManyFills  = "同日多笔成交，开平未判定"
	textMissQuote  = "需要持仓报价，暂缺"
	textNoSeries   = "没有标普500序列数据"
	textStaleSeria = "标普500序列已过期"
	textKindGap    = "无法判断开平"
	textJP         = "日本时段的数据尚未接入"
	textMissInput  = "输入缺失"
	textUnitMix    = "币种不一致"

	labelSP500 = "标普500"
	srcSP500   = "FRED SP500"
	seriesID   = "SP500"

	kindOpen     = "开仓"
	kindClose    = "平仓"
	kindExpire   = "到期"
	kindExercise = "行权"

	dataTimesPre  = "账户数据日 "
	dataTimesExec = "成交日 "
	dataTimesSer  = "SP500 数据日 "
)

var (
	columnsTrades   = []string{"时间", "账户", "操作", "合约", "数量", "价格"}
	columnsAccounts = []string{"账户", "净值", "当日金额", "当日百分比", "最大变动"}
	columnsIndices  = []string{"指数", "收盘", "日涨跌", "五日涨跌"}
)

// gapText is the text of a Gap by its reason; the reasons are those of a figure.
func gapText(reason string) string {
	switch reason {
	case "no_reader":
		return textNoReader
	case "no_data":
		return textNoCapture
	case "stale_input":
		return textStale
	case "missing_quote":
		return textMissQuote
	case "unsupported_currency":
		return textUnitMix
	case "jp_slice_missing":
		return textJP
	default:
		return textMissInput
	}
}
