package jobrun_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrun"
	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
)

var (
	_ jobrunapi.SlowFunc    = jobrun.Slow
	_ jobrunapi.FiguresFunc = jobrun.Figures
)

func TestSlowTableRows(t *testing.T) {
	t.Parallel()
	m := time.Minute
	cases := []struct {
		name           string
		elapsed, usual time.Duration
		samples        int
		want           bool
	}{
		{"exactly 3x is not slow", 60 * m, 20 * m, 5, false},
		{"just over 3x is slow", 60*m + time.Second, 20 * m, 5, true},
		{"floor: 2 minute job at 7 minutes", 7 * m, 2 * m, 9, false},
		{"floor: exactly 10 minutes", 10 * m, 2 * m, 9, false},
		{"floor: over 10 minutes", 10*m + time.Second, 2 * m, 9, true},
		{"4 samples ignore usual, under 60", 59 * m, 1 * m, 4, false},
		{"4 samples ignore usual, over 60", 61 * m, 20 * m, 4, true},
		{"no usual exactly 60", 60 * m, 0, 0, false},
		{"no usual over 60", 60*m + time.Second, 0, 0, true},
	}
	for _, c := range cases {
		if got := jobrun.Slow(c.elapsed, c.usual, c.samples); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFiguresMinutes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	runs := []jobrunapi.Input{
		{JobID: "a", StartedAt: now.Add(-70*time.Minute - 30*time.Second), KeysDone: 3, KeysTotal: 10, Step: "fetch"},
		{JobID: "b", StartedAt: now.Add(-7 * time.Minute)},
		{JobID: "c", StartedAt: now.Add(-90 * time.Minute)},
		{JobID: "d", StartedAt: now.Add(-30 * time.Minute)},
		{JobID: "e", StartedAt: now.Add(time.Minute)},
	}
	usuals := []jobrunapi.Usual{
		{JobID: "a", Median: 20*time.Minute + 40*time.Second, Samples: 7},
		{JobID: "b", Median: 2 * time.Minute, Samples: 20},
		{JobID: "d", Median: time.Minute, Samples: 4},
	}
	want := []jobrunapi.Figure{
		{JobID: "a", ElapsedMinutes: 70, UsualMinutes: 20, HasUsual: true, Slow: true, KeysDone: 3, KeysTotal: 10, Step: "fetch"},
		{JobID: "b", ElapsedMinutes: 7, UsualMinutes: 2, HasUsual: true},
		{JobID: "c", ElapsedMinutes: 90, Slow: true},
		{JobID: "d", ElapsedMinutes: 30},
		{JobID: "e"},
	}
	if got := jobrun.Figures(now, runs, usuals); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := jobrun.Figures(now, nil, nil); len(got) != 0 {
		t.Fatalf("no runs gave %v", got)
	}
}
