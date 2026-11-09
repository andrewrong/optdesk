package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/gaphistory"
)

// maxMorningLines is the most lines of one morning text; the self-healed list is cut to fit.
const maxMorningLines = 20

// morningInput is everything the text of the morning report is a function of.
type morningInput struct {
	day         string         // KL date of the report, YYYY-MM-DD
	zone        *time.Location // zone of the cleared times
	selfHealed  []gaphistory.Gap
	clearedSent int // gaps cleared after they were pushed
	open        int // gaps still open
	sessionDay  contracts.Day
	late        int // datasets of sessionDay with a non-empty expectation gap
}

// morningText builds the text of Decision 2 from fixed words. It holds no id or reason of a
// gap that was pushed already, and nothing derived from an account.
func morningText(in morningInput) string {
	lines := []string{"optdesk morning report " + in.day}
	var tail []string
	if in.clearedSent > 0 {
		tail = append(tail, fmt.Sprintf("cleared after push: %d", in.clearedSent))
	}
	if in.open > 0 {
		tail = append(tail, fmt.Sprintf("open gaps: %d", in.open))
	}
	if in.late > 0 {
		tail = append(tail, fmt.Sprintf("late datasets of %s: %d", in.sessionDay, in.late))
	}
	if len(in.selfHealed) == 0 && len(tail) == 0 {
		lines = append(lines, "all quiet")
	}
	if n := len(in.selfHealed); n > 0 {
		lines = append(lines, "self-healed:")
		// header + "self-healed:" + tail + details line are fixed; the rest is for gap lines.
		room := maxMorningLines - len(lines) - len(tail) - 1
		show := n
		if show > room {
			show = room - 1 // one line for "and N more"
		}
		for _, g := range in.selfHealed[:show] {
			lines = append(lines, healedLine(g, in.zone))
		}
		if m := n - show; m > 0 {
			lines = append(lines, fmt.Sprintf("and %d more", m))
		}
	}
	lines = append(lines, tail...)
	lines = append(lines, "details are on the local pages")
	return strings.Join(lines, "\n")
}

func healedLine(g gaphistory.Gap, zone *time.Location) string {
	what := "dataset " + g.DatasetID
	if g.Reason == "preflight_failed" {
		what = "job " + g.JobID
	}
	return fmt.Sprintf("%s day %s %s cleared %s", what, g.Day, reason(g.Reason), g.ClearedAt.In(zone).Format("01-02 15:04"))
}
