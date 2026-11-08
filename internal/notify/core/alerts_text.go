package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

func levelRank(l string) int {
	switch l {
	case "d":
		return 0
	case "w":
		return 1
	}
	return 2
}

func levelWord(l string) string {
	switch l {
	case "d":
		return "D"
	case "w":
		return "W"
	}
	return "N"
}

func sortAlerts(rs []compute.AlertResult) {
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if ra, rb := levelRank(a.Level), levelRank(b.Level); ra != rb {
			return ra < rb
		}
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		return a.RuleID < b.RuleID
	})
}

func alertLine(r compute.AlertResult) string {
	return fmt.Sprintf("%s %s: %s (data of %s) %s", levelWord(r.Level), r.Title, r.Detail, r.DataDay, r.Day)
}

// buildAlertText is a pure function of the results: it sorts a copy by level (d, w, n), day and
// rule id, prints at most 20 lines under 3500 bytes, and returns the text with the results
// printed in it. It carries the rule title, the figure detail and the days, nothing else.
func buildAlertText(rs []compute.AlertResult) (string, []compute.AlertResult) {
	s := append([]compute.AlertResult(nil), rs...)
	sortAlerts(s)
	n := min(len(s), maxLines)
	for ; n > 1; n-- {
		if t := renderAlerts(s, n); len(t) <= maxBytes {
			return t, s[:n]
		}
	}
	return renderAlerts(s, n), s[:n]
}

func renderAlerts(s []compute.AlertResult, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "optdesk: %d market alert(s)", len(s))
	for _, r := range s[:n] {
		b.WriteString("\n")
		b.WriteString(alertLine(r))
	}
	if m := len(s) - n; m > 0 {
		fmt.Fprintf(&b, "\nand %d more", m)
	}
	return b.String()
}
