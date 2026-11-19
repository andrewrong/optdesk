// Package jobrun is the pure rule and the figures of a running job: elapsed minutes, usual minutes and a
// slow flag. It implements jobrunapi.SlowFunc and jobrunapi.FiguresFunc. No I/O and no clock beyond the
// now argument. A slow run is a notice only; nothing here stops or refuses a job.
// Design: docs/design/scheduler-per-job.md, Decision 7.
package jobrun

import (
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
)

const (
	// slowFactor: a run is slow past three times its usual. No scaling by source or key count: runs of
	// one job differ in key count, and the median is robust to the odd long run, so the factor is wide.
	slowFactor = 3
	// slowFloor: a 2-minute job that took 7 raises nothing; below this elapsed is never slow.
	slowFloor = 10 * time.Minute
	// usualSamples: usual is taken only from this many closed runs or more.
	usualSamples = 5
	// usualRuns: the median rests on the last this many closed ok runs of one job (read by the caller).
	usualRuns = 20
	// noHistoryAfter: a job with no usual is slow past this, twice the 30-minute key bound, so it is
	// flagged late and not early.
	noHistoryAfter = 60 * time.Minute
)

var _ = usualRuns

// Slow says whether a run that has lasted elapsed is slow against usual, which rests on samples runs.
func Slow(elapsed, usual time.Duration, samples int) bool {
	if samples < usualSamples {
		return elapsed > noHistoryAfter
	}
	return elapsed > max(slowFactor*usual, slowFloor)
}

// Figures returns one Figure per run, in the order of runs, for the moment now.
func Figures(now time.Time, runs []jobrunapi.Input, usuals []jobrunapi.Usual) []jobrunapi.Figure {
	byJob := make(map[string]jobrunapi.Usual, len(usuals))
	for _, u := range usuals {
		byJob[u.JobID] = u
	}
	out := make([]jobrunapi.Figure, 0, len(runs))
	for _, r := range runs {
		elapsed := max(now.Sub(r.StartedAt), 0)
		u := byJob[r.JobID]
		f := jobrunapi.Figure{
			JobID: r.JobID, ElapsedMinutes: int(elapsed / time.Minute),
			KeysDone: r.KeysDone, KeysTotal: r.KeysTotal, Step: r.Step,
			Slow: Slow(elapsed, u.Median, u.Samples),
		}
		if u.Samples >= usualSamples {
			f.HasUsual, f.UsualMinutes = true, int(u.Median/time.Minute)
		}
		out = append(out, f)
	}
	return out
}
