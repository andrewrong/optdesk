package option_tracking_page

// Texts of the page. Option terms stay English, other text is Chinese. A gap text is the plain
// sentence of its tracking code; the code word is not shown.
const (
	pageTitle   = "My short option tracking"
	flexNote    = "Flex arrives the next day; positions opened or assigned today may not show yet; see the check table"
	markNote    = "Mid price, the midpoint of the closing bid/ask; not a traded price"
	noOpenText  = "No open short options"
	leftOutTail = " long legs not listed, see the option positions page"
	staleText   = "Some open contracts have no fresh closing price, the related cells show a gap"
	defaultName = "main"

	labelAccount = "Account ("
	labelClose   = ")"

	dash       = "—"
	unreadable = "Unreadable"
	notApply   = "Not applicable"
	inLife     = "In term"
	earlyBadge = "Early"
	basisHead  = "Basis "
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
	guardUnknownTxt = "Unconfirmed"
)

// codeTexts is the sentence of a gap text.
var codeTexts = map[string]string{
	codeNoMark:     "No fresh closing price",
	codeNoEventRow: "No candidate row of this company in the last 7 days, earnings and ex-dividend dates unknown",
	codeNoGuardRow: "No held call guard row, not treated as safe",
	codeNoFlexRows: "No Flex record for this account",
	codeNoAccount:  "No account position record, check table not built",
	codeNoValue:    "No such value",
	codeUnreadable: unreadable,
	"no_reader":    "No reader",

	"no_sessions":   "No session reader, the page cannot tell the trading day",
	"no_eod_reader": "No end-of-day price reader, earnings and ex-dividend dates unknown",
	"no_candidates": "No candidate reader, earnings and ex-dividend dates unknown",
	"no_names":      "No symbol reader, earnings and ex-dividend dates unknown",
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

var badgeMoneyness = map[string]string{"itm": "in the money", "otm": "out of the money", "atm": "at the money"}

var badgeGuard = map[string]string{"ok": "ok", "block": "Blocked", "blocked": "Blocked", "unknown": guardUnknownTxt, "n/a": notApply}

var guardTone = map[string]string{"ok": "up", "block": "down", "blocked": "down"}

var reasonTexts = map[string]string{
	"early_assignment_risk": "Early assignment risk before the ex-dividend date",
	"no_dividend_info":      "No dividend information",
	"stale_dividend":        "Dividend information is stale",
}

var checkNotes = map[string]string{
	"not_in_flex":    "Held in the account, not in Flex: maybe opened today, Flex arrives the next day",
	"not_in_account": "Open in Flex, gone from the account: maybe assigned or expired today, Flex not arrived yet",
}
