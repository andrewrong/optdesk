package underlyings

import "github.com/andrewrong/optdesk/internal/store/underlying"

// The four job groups that follow a name.
const (
	groupBars         = "bars"
	groupDividends    = "dividends"
	groupOptions      = "options"
	groupFundamentals = "fundamentals"
)

// Follows is the one rule of "followed by" (design underlyings Decision 6): the job groups, in the order bars,
// dividends, options, fundamentals, that follow a name. An active name is followed by bars; dividends need an
// issuer; options need the options switch; fundamentals need the switch and an issuer. A paused or retired name keeps
// only bars and dividends, and only when it is held. Pending and unresolved names are followed by nothing. The
// result is never nil. The rule is pinned by contracts/fixtures/underlyings.follow.json.
func Follows(status string, options, fundamentals, issuerSet, held bool) []string {
	out := []string{}
	switch status {
	case underlying.StatusActive:
	case underlying.StatusPaused, underlying.StatusRetired:
		if !held {
			return out
		}
		out = append(out, groupBars)
		if issuerSet {
			out = append(out, groupDividends)
		}
		return out
	default:
		return out
	}
	out = append(out, groupBars)
	if issuerSet {
		out = append(out, groupDividends)
	}
	if options {
		out = append(out, groupOptions)
	}
	if fundamentals && issuerSet {
		out = append(out, groupFundamentals)
	}
	return out
}
