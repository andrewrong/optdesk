// Package shared holds helpers the page packages use in common; it imports no sibling.
package shared

import (
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// LookbackDays is how many calendar days before the as-of day a close may be.
const LookbackDays = 14

// Bar is the close of one day from the trades bars.
type Bar struct {
	Day   contracts.Day
	Close string
}

// Print is the underlying price of one option_eod row; At is its event time, nil when unknown.
type Print struct {
	Day   contracts.Day
	Price string
	At    *time.Time
}

// Source is the candidates Pick chooses from.
type Source struct {
	Bars   []Bar
	Prints []Print
}

// Close is the picked close: its day, its price text and the source, "bar" or "option_eod".
type Close struct {
	Day        contracts.Day
	Price, Via string
}

// Pick returns the one underlying close of the as-of day. Candidates are bars and prints with a
// positive price, a day not after asOf and within LookbackDays calendar days. The newer day wins;
// on the same day the bar; among prints of a day the greatest At, a nil At last, equal At the later
// row. It reports false when no candidate is left.
func Pick(s Source, asOf contracts.Day) (Close, bool) {
	end, err := time.Parse(time.DateOnly, string(asOf))
	if err != nil {
		return Close{}, false
	}
	start := end.AddDate(0, 0, -LookbackDays)
	inWindow := func(d contracts.Day) bool {
		t, err := time.Parse(time.DateOnly, string(d))
		return err == nil && !t.After(end) && !t.Before(start)
	}

	var best Close
	found := false
	for _, b := range s.Bars {
		if !positive(b.Close) || !inWindow(b.Day) {
			continue
		}
		if !found || b.Day > best.Day {
			best, found = Close{Day: b.Day, Price: b.Close, Via: "bar"}, true
		}
	}
	var pick *Print
	for i := range s.Prints {
		p := &s.Prints[i]
		if !positive(p.Price) || !inWindow(p.Day) {
			continue
		}
		if pick == nil || p.Day > pick.Day || (p.Day == pick.Day && !worse(p.At, pick.At)) {
			pick = p
		}
	}
	if pick != nil && (!found || pick.Day > best.Day) {
		best, found = Close{Day: pick.Day, Price: pick.Price, Via: "option_eod"}, true
	}
	return best, found
}

// earlier reports whether a ranks worse than b: a nil time ranks last, equal times do not.
func worse(a, b *time.Time) bool {
	switch {
	case a == nil:
		return b != nil
	case b == nil:
		return false
	}
	return a.Before(*b)
}

// positive is a sign test on a decimal text: no minus sign and at least one non-zero digit.
func positive(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s[0] == '-' {
		return false
	}
	nonzero := false
	for _, r := range strings.TrimPrefix(s, "+") {
		switch {
		case r == '.':
		case r >= '1' && r <= '9':
			nonzero = true
		case r == '0':
		default:
			return false
		}
	}
	return nonzero
}
