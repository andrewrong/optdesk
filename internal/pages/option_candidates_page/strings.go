package option_candidates_page

// Texts of the page. Option terms stay English, other text is Chinese (option-pages-v22 Decision 6).
const (
	pageTitle    = "期权候选"
	estimateNote = "Bid (EOD 估) 是收盘 bid,下一交易日才卖,不是成交价"
	jpNote       = "日本期权候选:后续设计 (option-candidates-jp)"
	bidLabel     = "Bid (EOD 估)"
	bidHint      = "收盘 bid,下一交易日才卖,不是成交价"
	staleBadge   = "过期"
	dash         = "—"
	unreadable   = "无法读取"
	passText     = "通过"

	gapNoReader   = "没有候选读取器,无法显示"
	gapNoRun      = "近 7 天没有候选运行记录"
	gapNoRows     = "近 7 天没有这一类候选"
	staleTextHead = "候选行来自 "
	staleTextTail = ",不是当日"
	fitSeparator  = " "
	listSeparator = ";"
)

var groupTitles = map[string]string{
	kindShortPut:      "卖 put",
	kindCoveredCall:   "备兑 call",
	kindHeldCallGuard: "已持 call 守卫",
}

// markHints are the short Chinese hints of the marks; a mark not listed is shown as it is stored.
var markHints = map[string]string{
	"itm":                     "价内",
	"earnings_unconfirmed":    "财报日未确认",
	"earnings_beyond_horizon": "财报在合约之外",
	"no_dividend":             "无股息",
}

// fitHints are the hints of the stored fit words.
var fitHints = map[string]string{
	"fit":     "资金够",
	"no_fit":  "资金不够",
	"unknown": "未知",
}

// reasonTexts holds the sentence of each known code: the codes of option-candidates.md Decisions 7, 11 and 12.
var reasonTexts = map[string]string{
	"no_buy_price":           "没有设定买入价",
	"earnings_before_expiry": "财报日在到期日之前",
	"yield_below_cash":       "年化收益不高于短期国债利率",
	"stale_chain":            "期权链不是最新交易日的",
	"no_fx":                  "币种不同,不换算",
	"no_free_shares":         "没有可备兑的空闲股数",
	"strike_floor":           "行权价低于卖出价下限",
	"early_assignment_risk":  "除息前有提前被行权的风险",
}
