package option_candidates_page

// Texts of the page. Option terms stay English, other text is Chinese (option-pages-v22 Decision 6).
const (
	pageTitle    = "Option candidates"
	estimateNote = "Bid (EOD estimate) is the closing bid, sold on the next trading day, not a traded price"
	jpNote       = "Japanese option candidates: later design (option-candidates-jp)"
	bidLabel     = "Bid (EOD estimate)"
	bidHint      = "Closing bid, sold on the next trading day, not a traded price"
	staleBadge   = "Stale"
	dash         = "—"
	unreadable   = "Unreadable"
	passText     = "Pass"

	gapNoReader   = "No candidate reader: cannot show"
	gapNoRun      = "No candidate run in the last 7 days"
	gapNoRows     = "No candidates of this kind in the last 7 days"
	staleTextHead = "Candidate rows are from "
	staleTextTail = ", not the current day"
	fitSeparator  = " "
	listSeparator = ";"
)

var groupTitles = map[string]string{
	kindShortPut:      "Short put",
	kindCoveredCall:   "Covered call",
	kindHeldCallGuard: "Held call guard",
}

// markHints are the short Chinese hints of the marks; a mark not listed is shown as it is stored.
var markHints = map[string]string{
	"itm":                     "in the money",
	"earnings_unconfirmed":    "earnings date not confirmed",
	"earnings_beyond_horizon": "earnings after the contract",
	"no_dividend":             "no dividend",
}

// fitHints are the hints of the stored fit words.
var fitHints = map[string]string{
	"fit":     "Enough cash",
	"no_fit":  "Not enough cash",
	"unknown": "Unknown",
}

// reasonTexts holds the sentence of each known code: the codes of option-candidates.md Decisions 7, 11 and 12.
var reasonTexts = map[string]string{
	"no_buy_price":           "No buy price set",
	"earnings_before_expiry": "Earnings date is before expiry",
	"yield_below_cash":       "Annualised yield is not above the short-term Treasury rate",
	"stale_chain":            "Option chain is not from the latest trading day",
	"no_fx":                  "Different currency, not converted",
	"no_free_shares":         "No free shares to cover",
	"strike_floor":           "Strike is below the sell price floor",
	"early_assignment_risk":  "Early assignment risk before the ex-dividend date",
}
