package option_boards

import (
	"encoding/json"
	"errors"
	"regexp"
)

var (
	gatePattern = regexp.MustCompile(`^[a-z_]{1,32}$`)
	codePattern = regexp.MustCompile(`^[a-z_]{1,48}$`)
)

var errBadGates = errors.New("option_boards: unreadable gates")

// The stored gate statuses.
const (
	gatePass    = "pass"
	gateBlock   = "block"
	gateUnknown = "unknown"
)

// The gates the pages read.
const (
	gateEvents = "events"
	gateQuote  = "quote"
)

// gateResult is one stored gate result.
type gateResult struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// parseGates reads the stored JSON array text of the gate results; a text that is not one, or a result the page
// could not show, is an error.
func parseGates(text string) ([]gateResult, error) {
	var gates []gateResult
	if err := json.Unmarshal([]byte(text), &gates); err != nil || gates == nil {
		return nil, errBadGates
	}
	for _, g := range gates {
		if !gatePattern.MatchString(g.Gate) {
			return nil, errBadGates
		}
		switch g.Status {
		case gatePass:
		case gateBlock, gateUnknown:
			if !codePattern.MatchString(g.Reason) {
				return nil, errBadGates
			}
		default:
			return nil, errBadGates
		}
	}
	return gates, nil
}

// gateStatus is the status of the named gate; ok is false when the row holds no such gate.
func gateStatus(gates []gateResult, name string) (status string, ok bool) {
	for _, g := range gates {
		if g.Gate == name {
			return g.Status, true
		}
	}
	return "", false
}

// parseMarks reads the stored JSON array text of the marks.
func parseMarks(text string) ([]string, error) {
	var marks []string
	if err := json.Unmarshal([]byte(text), &marks); err != nil || marks == nil {
		return nil, errBadGates
	}
	return marks, nil
}

// markTexts are the badge texts of the marks of option-candidates.md Decision 3.
var markTexts = map[string]string{
	"earnings_unconfirmed":    "Earnings date not confirmed",
	"earnings_beyond_horizon": "Earnings passed, none within 60 days",
}

// markText is the Chinese badge text of a mark; a mark with no text is shown as it is stored.
func markText(mark string) string {
	if s, ok := markTexts[mark]; ok {
		return s
	}
	return mark
}

// markBadges is the badge text of each mark, in the stored order.
func markBadges(marks []string) []string {
	out := make([]string, len(marks))
	for i, m := range marks {
		out[i] = markText(m)
	}
	return out
}
