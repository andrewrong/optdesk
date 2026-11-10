package positions_page

// The words of the page, written once.
const (
	pageTitle = "持仓"

	titleStocks  = "股票"
	titleOptions = "期权"
	titleTrades  = "最近操作"

	labelAccount = "账户（"
	labelClose   = "）"

	textNoReader  = "没有读取器，暂缺"
	textUnknown   = "未知账户"
	textNoCapture = "这个账户还没有采集持仓"
	textStale     = "最近一次采集超过 3 个交易日"
	textStaleData = "账户数据日 "

	textNoMark       = "没有价格，暂缺"
	textDayProfit    = "当日盈亏需要前一交易日的价格，暂缺"
	textNoLedger     = "期权账本里没有这一笔，暂缺"
	textAmbiguous    = "期权账本里有多笔，暂缺"
	textBadExpiry    = "到期日无法读取，暂缺"
	textBadNumber    = "数字无法读取，暂缺"
	textPrecision    = "小数位超过两位，暂缺"
	textCurrency     = "币种不支持，暂缺"
	textNoTime       = "成交时间未知"
	textNoPrior      = "没有更早的采集，动作暂缺"
	textManyFills    = "同一合约当天不止一笔，动作暂缺"
	textNoTrades     = "最近 30 个交易日没有操作"
	textSkippedStart = "有 "
	textSkippedEnd   = " 行的数量不是整数，没有列出"

	sentenceStocks  = "股票 "
	sentenceRows    = " 行，期权 "
	sentenceLegs    = " 腿。"
	sentenceNothing = "没有持仓。"
	textNoHeld      = "没有持仓"

	actionSellOpen  = "卖出开仓"
	actionBuyOpen   = "买入开仓"
	actionSellClose = "卖出平仓"
	actionBuyClose  = "买入平仓"
	actionExpire    = "到期作废"
	actionExercise  = "行权"

	unitContract = " 张"
	unitShare    = " 股"
	perShare     = "/股"

	sellPut  = "卖出认沽"
	buyPut   = "买入认沽"
	sellCall = "卖出认购"
	buyCall  = "买入认购"
)

var (
	stockHeads  = []string{"code:代码", "name:名称", "role:归属", "quantity:数量", "cost:成本（每股）", "price:现价（每股）", "day:当日盈亏", "unrealised:未实现盈亏"}
	optionHeads = []string{"code:代码", "name:名称", "expiry:到期日", "days_left:剩余天数", "role:归属", "quantity:数量",
		"cost:成本（每股）", "price:现价（每股）", "day:当日盈亏", "unrealised:未实现盈亏", "open_premium:剩余权利金"}
	tradeHeads = []string{"time:时间", "account:账户", "action:动作", "instrument:标的·结构", "quantity:数量", "price:价格"}
)

// rightAligned are the columns of numbers.
var rightAligned = map[string]bool{"quantity": true, "cost": true, "price": true, "day": true, "unrealised": true, "days_left": true, "open_premium": true}

// gapTexts are the words of each gap reason a line of pagefig can carry.
var gapTexts = map[string]string{
	"missing_quote":        textNoMark,
	"missing_input":        textBadNumber,
	"unsupported_currency": textCurrency,
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
