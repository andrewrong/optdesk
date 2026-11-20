package option_tracking_page

// Texts of the page. Option terms stay English, other text is Chinese. A gap text is the plain
// sentence of its tracking code; the code word is not shown.
const (
	pageTitle   = "My short option tracking"
	flexNote    = flexNoteZh
	markNote    = markNoteZh
	noOpenText  = "No open short options"
	leftOutTail = leftOutTailZh
	staleText   = staleTextZh
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
	codeNoMark     = "no_mark"
	codeNoEventRow = "no_event_row"
	codeNoGuardRow = "no_guard_row"
	codeNoFlexRows = "no_flex_rows"
	codeNoAccount  = "no_account"
	codeNoValue    = "no_value"
	codeNoClose    = "no_close"

	codeMarkOnly    = "mark_only"
	codeUnreadable  = "unreadable"
	reasonQuote     = "missing_quote"
	reasonInput     = "missing_input"
	reasonData      = "no_data"
	reasonNoReader  = "no_reader"
	guardUnknownTxt = "Unconfirmed"

	badgeClose = "close"      // badge of a close from a bar
	badgePrint = "last print" // badge of a close from an option_eod print
)

// codeTexts is the sentence of a gap text.
var codeTexts = map[string]string{
	codeNoMark:     noMarkZh,
	codeNoEventRow: noEventRowZh,
	codeNoGuardRow: noGuardRowZh,
	codeNoFlexRows: noFlexRowsZh,
	codeNoAccount:  noAccountZh,
	codeNoValue:    noValueZh,
	codeNoClose:    noCloseZh,
	codeMarkOnly:   markOnlyZh,
	codeUnreadable: unreadable,
	"no_reader":    noReaderZh,

	"no_sessions":   noSessionsZh,
	"no_eod_reader": noEodReaderZh,
	"no_candidates": noCandidatesZh,
	"no_names":      noNamesZh,
}

// reasonOf is the common.v2 Gap reason of a tracking code.
var reasonOf = map[string]string{
	codeNoMark:     reasonQuote,
	codeNoEventRow: reasonInput,
	codeNoGuardRow: reasonInput,
	codeNoFlexRows: reasonData,
	codeNoAccount:  reasonData,
	codeNoValue:    reasonData,
	codeNoClose:    reasonQuote,
	codeMarkOnly:   reasonData,
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
	"early_assignment_risk": earlyAssignZh,
	"no_dividend_info":      noDividendZh,
	"stale_dividend":        staleDividendZh,
}

var checkNotes = map[string]string{
	"not_in_flex":    notInFlexZh,
	"not_in_account": notInAccountZh,
}
