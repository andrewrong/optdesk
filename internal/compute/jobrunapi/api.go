// Package jobrunapi declares the figures of the running jobs: for each run its elapsed minutes, the
// usual minutes of its job and a slow flag, computed once in compute and copied by the page builders
// and the notifier. Declarations only; no clock, no I/O.
//
// The figures are the only place the slow rule is applied: a page or the notifier copies Figure.Slow
// and never recomputes it. A slow run is a notice only: no type here can stop or refuse a job.
package jobrunapi

import "time"

// Input is one running job as the runner stored it.
type Input struct {
	JobID     string
	StartedAt time.Time
	KeysDone  int
	KeysTotal int
	Step      string
}

// Usual is the usual duration of one job: the median of its finished runs and how many runs it rests on.
type Usual struct {
	JobID   string
	Median  time.Duration
	Samples int
}

// Figure is the figures of one run. HasUsual is false when the job has no usual yet (UsualMinutes is 0
// and Slow is false). Slow is a notice only.
type Figure struct {
	JobID          string
	ElapsedMinutes int
	UsualMinutes   int
	HasUsual       bool
	Slow           bool
	KeysDone       int
	KeysTotal      int
	Step           string
}

// FiguresFunc returns one Figure per run, in the order of runs, for the moment now. It is the only place
// SlowFunc is applied.
type FiguresFunc func(now time.Time, runs []Input, usuals []Usual) []Figure

// SlowFunc says whether a run that has lasted elapsed is slow against the usual duration of its job,
// which rests on samples runs. The answer is a notice only; it never stops or refuses a job.
type SlowFunc func(elapsed, usual time.Duration, samples int) bool
