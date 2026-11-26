package optiontier

import "github.com/andrewrong/optdesk/internal/compute/optiontierapi"

// Look-at badges (amber), info badges and the note.
const (
	badgeNoOpenInterest  = "no_open_interest"
	badgeLowOpenInterest = "low_open_interest"
	badgeOneSidedQuote   = "one_sided_quote"
	badgeWideQuote       = "wide_quote"
	badgeStaleQuote      = "stale_quote"
	badgeEarningsBefore  = "earnings_before_expiry"
	badgeCashShort       = "cash_short"
	badgeDeltaAboveBound = "delta_above_bound"
	badgeDeltaUnknown    = "delta_unknown"
	badgeNoJPYRate       = "no_jpy_rate"
	badgeNoJPYFunds      = "no_jpy_funds"

	infoEarningsUnconfirmed = "earnings_unconfirmed"
	infoEarningsBeyond      = "earnings_beyond_horizon"
	infoEarningsUnknown     = "earnings_unknown"
	infoCashUnknown         = "cash_unknown"

	noteThinPremium   = "thin_premium"
	noteAboveBuyPrice = "above_buy_price"

	eventsEarningsBeforeExpiry = "earnings_before_expiry"
)

// lookOrder and infoOrder are the fixed order of the badges in a verdict, in Kept and in Common.
var (
	lookOrder = []string{badgeNoOpenInterest, badgeLowOpenInterest, badgeOneSidedQuote, badgeWideQuote, badgeStaleQuote,
		badgeEarningsBefore, badgeCashShort, badgeDeltaAboveBound, badgeDeltaUnknown,
		badgeNoJPYRate, badgeNoJPYFunds}
	infoOrder = []string{infoEarningsUnconfirmed, infoEarningsBeyond, infoEarningsUnknown, infoCashUnknown}
)

// quoteBadge maps a stored quote reason to its look-at badge.
var quoteBadge = map[string]string{
	"no_open_interest":  badgeNoOpenInterest,
	"low_open_interest": badgeLowOpenInterest,
	"no_quote":          badgeOneSidedQuote,
	"no_bid":            badgeOneSidedQuote,
	"no_ask":            badgeOneSidedQuote,
	"small_bid_size":    badgeOneSidedQuote,
	"wide_spread":       badgeWideQuote,
	"delayed_data":      badgeStaleQuote,
}

// eventsBadge maps a stored events reason to its look-at badge or its info badge (not both).
var (
	eventsLook = map[string]string{
		eventsEarningsBeforeExpiry: badgeEarningsBefore,
		"stale_chain":              badgeStaleQuote,
	}
	eventsInfo = map[string]string{
		"earnings_unconfirmed": infoEarningsUnconfirmed,
		"beyond_horizon":       infoEarningsBeyond,
		"no_earnings":          infoEarningsUnknown,
		"stale_earnings":       infoEarningsUnknown,
		"earnings_unsupported": infoEarningsUnknown,
		"earnings_error":       infoEarningsUnknown,
	}
	// markInfo maps a stored mark word to an info badge; the mark no_open_interest is a look-at badge.
	markInfo = map[string]string{
		"earnings_unconfirmed":    infoEarningsUnconfirmed,
		"earnings_beyond_horizon": infoEarningsBeyond,
		"earnings_unknown":        infoEarningsUnknown,
	}
)

// storedLookAt is the set of look-at badges the stored reasons and marks give.
func storedLookAt(r optiontierapi.Row) map[string]bool {
	set := map[string]bool{}
	if b, ok := quoteBadge[r.QuoteReason]; ok {
		set[b] = true
	}
	if b, ok := eventsLook[r.EventsReason]; ok {
		set[b] = true
	}
	for _, m := range []string{badgeNoOpenInterest, badgeNoJPYRate, badgeNoJPYFunds} {
		if hasMark(r.Marks, m) {
			set[m] = true
		}
	}
	return set
}

// storedInfos is the set of info badges the stored reasons and marks give.
func storedInfos(r optiontierapi.Row) map[string]bool {
	set := map[string]bool{}
	if b, ok := eventsInfo[r.EventsReason]; ok {
		set[b] = true
	}
	for _, m := range r.Marks {
		if b, ok := markInfo[m]; ok {
			set[b] = true
		}
	}
	return set
}

// ordered lists the members of set in the fixed order; never nil.
func ordered(set map[string]bool, order []string) []string {
	out := []string{}
	for _, c := range order {
		if set[c] {
			out = append(out, c)
		}
	}
	return out
}

// fold writes Badges and Infos of every state and returns Common: the stored info marks every row
// carries, in the fixed order, removed from each row's Infos. cash_unknown is a per-row info and is
// never folded.
func fold(states []*rowState) []string {
	common := []string{}
	for _, c := range infoOrder {
		if c == infoCashUnknown || len(states) == 0 {
			continue
		}
		every := true
		for _, s := range states {
			every = every && s.infos[c]
		}
		if every {
			common = append(common, c)
			for _, s := range states {
				delete(s.infos, c)
			}
		}
	}
	for _, s := range states {
		s.v.Badges = ordered(s.look, lookOrder)
		s.v.Infos = ordered(s.infos, infoOrder)
		if s.v.Notes == nil {
			s.v.Notes = []string{}
		}
	}
	return common
}
