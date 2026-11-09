package optionnotice

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

// guardNotices raises call_guard (warn) for a held short call that has a stored blocked
// guard row; the stored verdict and its reason codes are read, never recomputed.
func guardNotices(cur []held, guards []compute.NoticeGuard) []compute.Notice {
	var out []compute.Notice
	for _, g := range guards {
		if !strings.EqualFold(g.Status, "blocked") {
			continue
		}
		gd, ok := parseDay(g.Expiry)
		strike, sok := parseDec(g.Strike)
		if !ok || !sok {
			continue
		}
		for _, h := range cur {
			if !h.short() || h.right != "C" || h.symbol != g.Symbol || h.expDays != gd || h.strike.Cmp(strike) != 0 {
				continue
			}
			var facts []compute.NoticeFact
			for _, r := range g.Reasons {
				facts = append(facts, fact("reason", r))
			}
			out = append(out, newNotice("call_guard", sevWarn, h, facts...))
		}
	}
	return out
}

// earningsNotices raises earnings_inside (warn) for a short option whose issuer reports
// earnings from the day itself up to the expiry day.
func earningsNotices(cur []held, earnings []compute.NoticeEarnings, day int) []compute.Notice {
	next := map[string]int{}
	for _, e := range earnings {
		if e.Next == nil {
			continue
		}
		if d, ok := parseDay(string(*e.Next)); ok {
			next[e.Symbol] = d
		}
	}
	var out []compute.Notice
	for _, h := range cur {
		d, ok := next[h.symbol]
		if !h.short() || !ok || d < day || d > h.expDays {
			continue
		}
		out = append(out, newNotice("earnings_inside", sevWarn, h, fact("earnings", dayText(d))))
	}
	return out
}
