package positions_page

// TextNoStoredName is the Name cell of a stock whose company name the underlyings registry does not hold.
// Every string literal with a Chinese character of this package is declared in this file.
const TextNoStoredName = "未存储名称"

// The words of the page that read Chinese first with the English beside.
const (
	pageTitle = "持仓 Positions"

	titleStocks  = "股票 Stocks"
	titleOptions = "期权 Options"
	titleTrades  = "最近成交 Recent trades"

	labelAccount = "账户 Account ("

	actionSellOpen  = "卖出开仓 Sell to open"
	actionBuyOpen   = "买入开仓 Buy to open"
	actionSellClose = "卖出平仓 Sell to close"
	actionBuyClose  = "买入平仓 Buy to close"
	actionExpire    = "到期 Expired"
	actionExercise  = "行权 Exercised"

	sellPut  = "卖出看跌 Sell put"
	buyPut   = "买入看跌 Buy put"
	sellCall = "卖出看涨 Sell call"
	buyCall  = "买入看涨 Buy call"
)

// The labels of the column heads; the slices in strings.go join them to the column ids.
const (
	headCode        = "代码 Code"
	headName        = "名称 Name"
	headOwner       = "持有人 Owner"
	headQuantity    = "数量 Quantity"
	headCost        = "成本（每股） Cost (per share)"
	headPrice       = "最新价（每股） Last price (per share)"
	headDay         = "当日结果 Day result"
	headUnrealised  = "未实现结果 Unrealised result"
	headExpiry      = "到期日 Expiry"
	headDaysLeft    = "剩余天数 Days left"
	headOpenPremium = "开仓权利金 Open premium"
	headTime        = "时间 Time"
	headAccount     = "账户 Account"
	headAction      = "动作 Action"
	headInstrument  = "标的 · 结构 Underlying · structure"
	headTradePrice  = "价格 Price"
)
