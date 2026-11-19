package jobrunapi_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
)

// fakeFigures is a made-up FiguresFunc: slow when elapsed is over twice the usual.
func fakeFigures(now time.Time, runs []jobrunapi.Input, usuals []jobrunapi.Usual) []jobrunapi.Figure {
	out := []jobrunapi.Figure{}
	for _, r := range runs {
		f := jobrunapi.Figure{
			JobID: r.JobID, ElapsedMinutes: int(now.Sub(r.StartedAt) / time.Minute),
			KeysDone: r.KeysDone, KeysTotal: r.KeysTotal, Step: r.Step,
		}
		f.Slow = fakeSlow(now.Sub(r.StartedAt), 0, 0)
		for _, u := range usuals {
			if u.JobID == r.JobID {
				f.HasUsual, f.UsualMinutes = true, int(u.Median/time.Minute)
				f.Slow = fakeSlow(now.Sub(r.StartedAt), u.Median, u.Samples)
			}
		}
		out = append(out, f)
	}
	return out
}

func fakeSlow(elapsed, usual time.Duration, samples int) bool {
	if samples == 0 {
		return elapsed > 60*time.Minute // no history: a fixed bound
	}
	return elapsed > 2*usual
}

// pageSide and notifySide copy the figures and never decide slowness themselves.
func pageSide(f jobrunapi.FiguresFunc, now time.Time, runs []jobrunapi.Input, us []jobrunapi.Usual) map[string]bool {
	m := map[string]bool{}
	for _, fig := range f(now, runs, us) {
		m[fig.JobID] = fig.Slow
	}
	return m
}

func notifySide(f jobrunapi.FiguresFunc, now time.Time, runs []jobrunapi.Input, us []jobrunapi.Usual) []string {
	var ids []string
	for _, fig := range f(now, runs, us) {
		if fig.Slow {
			ids = append(ids, fig.JobID)
		}
	}
	return ids
}

func TestFiguresFuncContract(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	runs := []jobrunapi.Input{
		{JobID: "a", StartedAt: now.Add(-50 * time.Minute), KeysDone: 3, KeysTotal: 10, Step: "fetch"},
		{JobID: "b", StartedAt: now.Add(-5 * time.Minute)},
		{JobID: "c", StartedAt: now.Add(-90 * time.Minute)},
	}
	usuals := []jobrunapi.Usual{
		{JobID: "a", Median: 20 * time.Minute, Samples: 7},
		{JobID: "b", Median: 20 * time.Minute, Samples: 7},
	}
	var f jobrunapi.FiguresFunc = fakeFigures

	got := f(now, runs, usuals)
	want := []jobrunapi.Figure{
		{JobID: "a", ElapsedMinutes: 50, UsualMinutes: 20, HasUsual: true, Slow: true, KeysDone: 3, KeysTotal: 10, Step: "fetch"},
		{JobID: "b", ElapsedMinutes: 5, UsualMinutes: 20, HasUsual: true},
		{JobID: "c", ElapsedMinutes: 90, Slow: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("figures = %+v, want %+v", got, want)
	}
	if page := pageSide(f, now, runs, usuals); !page["a"] || page["b"] || !page["c"] {
		t.Errorf("page side slow = %v", page)
	}
	if ids := notifySide(f, now, runs, usuals); !reflect.DeepEqual(ids, []string{"a", "c"}) {
		t.Errorf("notify side slow = %v", ids)
	}
	if got := f(now, nil, nil); len(got) != 0 {
		t.Errorf("no runs gave %v", got)
	}
}

func TestSlowFuncContract(t *testing.T) {
	t.Parallel()
	var s jobrunapi.SlowFunc = fakeSlow
	cases := []struct {
		name           string
		elapsed, usual time.Duration
		samples        int
		want           bool
	}{
		{"well over", 50 * time.Minute, 20 * time.Minute, 7, true},
		{"within", 30 * time.Minute, 20 * time.Minute, 7, false},
		{"no samples under the bound", 50 * time.Minute, 20 * time.Minute, 0, false},
		{"no samples over the bound", 90 * time.Minute, 0, 0, true},
	}
	for _, c := range cases {
		if got := s(c.elapsed, c.usual, c.samples); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNoHistoryRunOverAnHourIsSlow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	var f jobrunapi.FiguresFunc = fakeFigures
	got := f(now, []jobrunapi.Input{{JobID: "c", StartedAt: now.Add(-90 * time.Minute)}}, nil)
	if len(got) != 1 || got[0].HasUsual || !got[0].Slow {
		t.Fatalf("a run with no history at 90 min must be slow (Decision 7), got %+v", got)
	}
}
