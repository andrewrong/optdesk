package option_tracking_page

// Texts of the page. Option terms stay English, other text is Chinese. A gap text starts with
// the tracking code (Decision 11) so a reader and a test find the cause by its first word.
const (
	pageTitle   = "我的卖方期权追踪"
	flexNote    = "Flex 次日到账,今天新开或被指派的可能还没出现;见核对表"
	markNote    = "中间价,收盘 bid/ask 的中点;不是成交价"
	noOpenText  = "没有未平仓的卖方期权"
	leftOutTail = " 个多头腿未列出,见期权持仓页"
	staleText   = "有未平仓合约没有新鲜的收盘价,相关格显示为缺口"
	defaultName = "main"

	dash       = "—"
	unreadable = "无法读取"
	notApply   = "不适用"
	inLife     = "在存续期内"
	earlyBadge = "提前"
	basisHead  = "依据 "
	reasonSep  = " · "
	termSep    = " "
	noteSep    = ":"
	idSep      = "-"
	titleSep   = " · "
)

// Tracking codes and the common.v2 reasons they use.
const (
	codeNoMark      = "no_mark"
	codeNoEventRow  = "no_event_row"
	codeNoGuardRow  = "no_guard_row"
	codeNoFlexRows  = "no_flex_rows"
	codeNoAccount   = "no_account"
	codeNoValue     = "no_value"
	codeUnreadable  = "unreadable"
	reasonQuote     = "missing_quote"
	reasonInput     = "missing_input"
	reasonData      = "no_data"
	reasonNoReader  = "no_reader"
	guardUnknownTxt = "未确认"
)

// codeTexts is the sentence after the code of a gap text.
var codeTexts = map[string]string{
	codeNoMark:     "没有新鲜的收盘价",
	codeNoEventRow: "近 7 天没有这家公司的候选行,财报和除息日未知",
	codeNoGuardRow: "没有已持 call 守卫行,不当作安全",
	codeNoFlexRows: "这个账户没有 Flex 记录",
	codeNoAccount:  "没有账户持仓记录,核对表未生成",
	codeNoValue:    "没有这个值",
	codeUnreadable: unreadable,
	"no_reader":    "没有读取器",
}

// reasonOf is the common.v2 Gap reason of a tracking code.
var reasonOf = map[string]string{
	codeNoMark:     reasonQuote,
	codeNoEventRow: reasonInput,
	codeNoGuardRow: reasonInput,
	codeNoFlexRows: reasonData,
	codeNoAccount:  reasonData,
	codeNoValue:    reasonData,
	codeUnreadable: reasonData,
}

// gatherCodes maps a gap name of gather to the tracking code of the page gap.
var gatherCodes = map[string]string{
	gapNoSessions:   "no_sessions",
	gapNoEodReaders: "no_eod_reader",
	gapNoCandidates: "no_candidates",
	gapNoNames:      "no_names",
	gapNoAccount:    codeNoAccount,
}

var badgeMoneyness = map[string]string{"itm": "价内", "otm": "价外", "atm": "平价"}

var badgeGuard = map[string]string{"ok": "ok", "block": "阻止", "blocked": "阻止", "unknown": guardUnknownTxt, "n/a": notApply}

var guardTone = map[string]string{"ok": "up", "block": "down", "blocked": "down"}

var reasonTexts = map[string]string{
	"early_assignment_risk": "除息前有提前被行权的风险",
	"no_dividend_info":      "没有股息信息",
	"stale_dividend":        "股息信息已过期",
}

var checkNotes = map[string]string{
	"not_in_flex":    "账户持有,Flex 里没有:可能今天新开,Flex 次日到账",
	"not_in_account": "Flex 里未平仓,账户已没有:可能今天被指派或到期,Flex 待到账",
}
