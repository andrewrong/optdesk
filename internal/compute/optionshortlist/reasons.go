package optionshortlist

// Codes of the dropped rows. Design: docs/design/candidates-page-v2.md#Decisions (11).
const (
	codeBadRow         = "bad_row"
	codeNoBid          = "no_bid"
	codeNoOpenInterest = "no_open_interest"
	codeNoSpot         = "no_spot"
	codeOTMLow         = "otm_low"
	codeOTMHigh        = "otm_high"
	codeNoDelta        = "no_delta"
	codeDeltaOut       = "delta_out"
	codeOverCap        = "over_cap"
)

// dropOrder is the fixed precedence of the codes: a row counts under the first that applies, and the
// checks run in this order.
var dropOrder = []string{
	codeBadRow, codeNoBid, codeNoOpenInterest, codeNoSpot, codeOTMLow,
	codeOTMHigh, codeNoDelta, codeDeltaOut, codeOverCap,
}

// Stored reasons and marks the rule reads.
const (
	markNoOpenInterest   = "no_open_interest"
	markUnreadableGates  = "unreadable_gates"
	quoteLowOpenInterest = "low_open_interest"
	quoteWideSpread      = "wide_spread"
	eventsStaleChain     = "stale_chain"
)

// noBidQuote maps the quote reasons that mean the bid cannot be sold into to the code no_bid.
var noBidQuote = map[string]bool{
	"no_quote": true, "no_bid": true, "no_ask": true, "small_bid_size": true,
}
