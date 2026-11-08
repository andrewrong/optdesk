package gaps

import (
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// Lines gives one line per gap of gs, in order; every gap is shown. ReportedAt, when set, is
// printed as "said at <time>" for information only and never hides or repeats a gap.
// A key gap names its dataset, a preflight gap its job.
func Lines(gs []store.Gap, now time.Time) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		who := g.DatasetID
		if who == "" {
			who = g.JobID
		}
		line := fmt.Sprintf("%s %s: %s, open %s", who, g.Day, g.Reason, now.Sub(g.FirstSeenAt).Truncate(time.Minute))
		if !g.ReportedAt.IsZero() {
			line += ", said at " + g.ReportedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, line)
	}
	return out
}
