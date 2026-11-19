// running.go builds the running lines above the grid (docs/design/scheduler-per-job.md, Decision 8). It copies the
// figures of Input.Figures and writes their text; it computes nothing, the slow rule included.
package data_health_page

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
)

const clockLayout = "15:04"

// runningVM is one element of running[]: the figures as given, and the line that prints them.
type runningVM struct {
	Line           string `json:"line"`
	Job            string `json:"job"`
	StartedAt      string `json:"started_at"`
	KeysDone       int    `json:"keys_done"`
	KeysTotal      int    `json:"keys_total"`
	Step           string `json:"step"`
	ElapsedMinutes int    `json:"elapsed_minutes"`
	UsualMinutes   int    `json:"usual_minutes"`
	Slow           bool   `json:"slow"`
}

// runningLines returns one line per open run, in the order of Figures. Without Runs or Figures, or when a read
// fails, it returns nil and the page has no running lines.
func runningLines(ctx context.Context, in Input) []runningVM {
	if in.Runs == nil || in.Figures == nil {
		return nil
	}
	open, err := in.Runs.Running(ctx)
	if err != nil {
		return nil
	}
	usuals, err := in.Runs.Usuals(ctx)
	if err != nil {
		return nil
	}
	runs := make([]jobrunapi.Input, len(open))
	for i, o := range open {
		runs[i] = jobrunapi.Input{JobID: o.JobID, StartedAt: o.StartedAt, KeysDone: o.KeysDone, KeysTotal: o.KeysTotal, Step: o.Step}
	}
	us := make([]jobrunapi.Usual, len(usuals))
	for i, u := range usuals {
		us[i] = jobrunapi.Usual{JobID: u.JobID, Median: u.Median, Samples: u.Samples}
	}
	var out []runningVM
	used := make([]bool, len(open))
	for _, f := range in.Figures(in.Now, runs, us) {
		// a figure belongs to the first unused open run of its own job, whatever the order it comes in
		i := -1
		for k, o := range open {
			if !used[k] && o.JobID == f.JobID {
				i = k
				break
			}
		}
		if i < 0 {
			continue
		}
		used[i] = true
		out = append(out, runningVM{
			Line: runningLine(f, open[i].StartedAt), Job: f.JobID, StartedAt: open[i].StartedAt.UTC().Format(time.RFC3339),
			KeysDone: f.KeysDone, KeysTotal: f.KeysTotal, Step: f.Step,
			ElapsedMinutes: f.ElapsedMinutes, UsualMinutes: f.UsualMinutes, Slow: f.Slow,
		})
	}
	return out
}

// runningLine is the text of one run, the same as the console's "Running now".
func runningLine(f jobrunapi.Figure, startedAt time.Time) string {
	parts := []string{f.JobID, "started " + startedAt.UTC().Format(clockLayout) + " UTC", fmt.Sprintf(zhDoneOfFmt, f.KeysDone, f.KeysTotal)}
	if f.Step != "" {
		parts = append(parts, f.Step)
	}
	if f.HasUsual {
		parts = append(parts, fmt.Sprintf(zhRanUsuallyFmt, f.ElapsedMinutes, f.UsualMinutes))
	} else {
		parts = append(parts, fmt.Sprintf(zhRanNoHistoryFmt, f.ElapsedMinutes))
	}
	if f.Slow {
		parts = append(parts, zhSlowNote)
	}
	return strings.Join(parts, "; ")
}
