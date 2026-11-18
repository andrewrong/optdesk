package option_pnl_page

// Texts of the page. Design: docs/design/options-pnl-page.md#Decisions.
const (
	pageTitle = "P&L split"

	modelNote = "model: European Black-Scholes, 365-day year, no dividends; an ex-dividend drop lands in the solved IV; " +
		"the early-exercise premium is not modelled"
	quoteDayNote = "quote day as the ledger counts it: ask above 0, bid at or above 0 and an underlying price"
	callsTail    = " call lots skipped: this page shows short puts only"
	longsTail    = " long put lots skipped: this page shows short puts only"

	sampleFew  = "fewer than 20 ended lots: no conclusion about what pays"
	sampleEnuf = "20 or more ended lots"

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
	codeNoReader:     "No ledger reader",
	codeNoFlexRows:   "No Flex record for this account",
	codeNoSessions:   "No session reader, the page cannot tell the trading day",
	codeNoQuoteReder: "No stored quote reader, the page cannot split profit",
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
	noQuotesText   = "no stored quotes: no split"
	noQuotesBadge  = "no quotes"
	ledgerMissing  = "not stored"
	markedAtMid    = "marked at the stored mid of "
	circularNote   = "The open-lot check is circular: our stored mid is on both sides, so it checks fills, commissions and the charge, not the market value"
	leftOutHead    = " lots left out of the sums: "
	noBookLine     = "no lot of this currency could be split"
	brokerDiffers  = "differs (not explained)"
	brokerSame     = "same"
	badgeTwoDay    = "2-day step"
	badgeWide      = "wide quote"
	badgeLate      = "late entry"
	badgeRoll      = "looks like a roll"
	badgeUnsplit   = "not split: "
	noQuoteDays    = "no stored quote days"
	notComputed    = "not computed"
	assignmentText = "total + assignment intrinsic = ledger"
)

var classTexts = map[string]string{
	classNoMark:   "with no stored quote",
	classNoClose:  "with no stored close",
	classNoLedger: "with no stored ledger P&L",
	classRefused:  "with input refused",
}

var classReasons = map[string]string{
	classNoMark:   reasonQuote,
	classNoClose:  reasonQuote,
	classNoLedger: reasonData,
	classRefused:  reasonInput,
}

// hints are the plain-language gloss written next to a jargon label.
var hints = map[string]string{
	"rent":       "what you earned for selling insurance: time minus move cost",
	"time":       "the option loses value as days pass; the seller keeps it",
	"move_cost":  "convexity: the bigger the stock move, the more the put gains against you",
	"direction":  "the bet that the stock does not fall; your valuation judgement, not a volatility edge",
	"iv_remark":  "implied volatility, the market's price for future swings, moved; it reverts",
	"unsplit":    "steps whose IV could not be solved; reason on the lot",
	"entry":      "fill price against the stored mid of bid and ask",
	"exit":       "fill price of the buy-back against the stored mid of bid and ask",
	"total":      "the option side; the assignment intrinsic below is not in it",
	"assigned":   "value moved to the shares when a put is assigned",
	"ledger":     "the ledger's figure for this lot",
	"difference": "ledger minus total minus assignment intrinsic",
}
