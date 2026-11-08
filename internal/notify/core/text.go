package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

const (
	maxLines = 20
	maxBytes = 3500
)

func reason(r string) string {
	switch r {
	case "expired", "crashed_repeatedly", "preflight_failed":
		return r
	}
	return "unknown"
}

func line(g store.Gap) string {
	if g.JobID != "" {
		return fmt.Sprintf("job %s day %s %s", g.JobID, g.Day, reason(g.Reason))
	}
	return fmt.Sprintf("dataset %s day %s %s", g.DatasetID, g.Day, reason(g.Reason))
}

func sortGaps(gaps []store.Gap) {
	sort.SliceStable(gaps, func(i, j int) bool {
		a, b := gaps[i], gaps[j]
		if !a.FirstSeenAt.Equal(b.FirstSeenAt) {
			return a.FirstSeenAt.Before(b.FirstSeenAt)
		}
		if a.JobID != b.JobID {
			return a.JobID < b.JobID
		}
		return a.DatasetID < b.DatasetID
	})
}

// buildText is a pure function of the gaps: it sorts a copy, prints at most 20 gap lines and
// stays under 3500 bytes, and returns the text with the gaps printed in it.
func buildText(gaps []store.Gap) (string, []store.Gap) {
	s := append([]store.Gap(nil), gaps...)
	sortGaps(s)
	n := len(s)
	if n > maxLines {
		n = maxLines
	}
	for ; n > 1; n-- {
		if t := render(s, n); len(t) <= maxBytes {
			return t, s[:n]
		}
	}
	return render(s, n), s[:n]
}

func render(s []store.Gap, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "optdesk: %d job(s) missed work", len(s))
	for _, g := range s[:n] {
		b.WriteString("\n")
		b.WriteString(line(g))
	}
	if m := len(s) - n; m > 0 {
		fmt.Fprintf(&b, "\nand %d more", m)
	}
	return b.String()
}
