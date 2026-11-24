package option_pnl_page

// Texts of the page. Design: docs/design/options-pnl-page.md#Decisions.
const (
	pageTitle = "P&L split"

	modelNote    = modelNoteZh
	quoteDayNote = quoteDayNoteZh
	callsTail    = callsTailZh
	longsTail    = " long put lots skipped: " + longPutSkippedZh

	sampleFew  = sampleFewZh
	sampleEnuf = sampleEnufZh

	defaultName = "main"

	// gap codes of the page (Decision 17)
	codeNoReader     = "no_reader"
	codeNoFlexRows   = "no_flex_rows"
	codeNoSessions   = "no_sessions"
	codeNoQuoteReder = "no_quote_reader"

	reasonNoReader = "no_reader"
	reasonData     = "no_data"
	reasonQuote    = "missing_quote"
	reasonInput    = "missing_input"

	notStored  = "not stored"
	minusSign  = "−"
	statusOpen = "open"
)

var codeTexts = map[string]string{
	codeNoReader:     gapNoReaderZh,
	codeNoFlexRows:   gapNoFlexRowsZh,
	codeNoSessions:   gapNoSessionsZh,
	codeNoQuoteReder: gapNoQuoteReaderZh,
}

var codeReasons = map[string]string{
	codeNoReader:     reasonNoReader,
	codeNoFlexRows:   reasonData,
	codeNoSessions:   reasonNoReader,
	codeNoQuoteReder: reasonNoReader,
}

// classes of a lot the page cannot split (Decision 7)
const (
	classNoMark    = "no_mark"
	classNoClose   = "no_close"
	classNoLedger  = "no_ledger"
	classRefused   = "refused"
	refusedPrefix  = "input refused: "
	noQuotesBadge  = "no quotes"
	ledgerMissing  = "not stored"
	markedAtMid    = "marked at the stored mid of "
	circularNote   = circularNoteZh
	leftOutHead    = leftOutHeadZh
	noBookLine     = noBookLineZh
	brokerDiffers  = brokerDiffersZh
	brokerSame     = "same"
	badgeTwoDay    = "2-day step"
	badgeWide      = "wide quote"
	badgeLate      = "late entry"
	badgeRoll      = "looks like a roll"
	badgeUnsplit   = "not split: "
	noQuoteDays    = noQuoteDaysZh
	notComputed    = notComputedZh
	assignmentText = cardHintAssignmentZh
)

var classTexts = map[string]string{
	classNoMark:   classNoMarkZh,
	classNoClose:  classNoCloseZh,
	classNoLedger: classNoLedgerZh,
	classRefused:  classRefusedZh,
}

var classReasons = map[string]string{
	classNoMark:   reasonQuote,
	classNoClose:  reasonQuote,
	classNoLedger: reasonData,
	classRefused:  reasonInput,
}

// hints are the plain-language gloss written next to a jargon label.
var hints = map[string]string{
	"rent":       ledgerHintRentZh,
	"time":       ledgerHintTimeZh,
	"move_cost":  ledgerHintMoveCostZh,
	"direction":  ledgerHintDirectionZh,
	"iv_remark":  ledgerHintIVRemarkZh,
	"unsplit":    cardHintUnsplitZh,
	"entry":      ledgerHintEntryZh,
	"exit":       ledgerHintExitZh,
	"total":      ledgerHintTotalZh,
	"assigned":   ledgerHintAssignedZh,
	"ledger":     cardHintLedgerZh,
	"difference": cardHintDifferenceZh,
}
