package macroevents

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestPlanKeys(t *testing.T) {
	t.Parallel()
	j, err := New(&fakeSource{}, &fakeWriter{}, newClock())
	if err != nil {
		t.Fatal(err)
	}
	keys, h1, err := j.Plan(context.Background(), "2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"boj", "cpi", "employment_situation", "fomc", "fomc_minutes", "gdp", "h41", "jolts", "pce", "ppi", "quarter_end", "retail_sales", "us_tax"}
	var got []string
	for _, k := range keys {
		if k.DatasetID != "obs.macro_event" {
			t.Errorf("dataset %q", k.DatasetID)
		}
		got = append(got, k.SubjectKey)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("keys = %v", got)
	}
	_, same, _ := j.Plan(context.Background(), "2026-11-14")
	_, next, _ := j.Plan(context.Background(), "2026-11-15")
	if h1 != same || h1 == next {
		t.Fatalf("hashes %s %s %s", h1, same, next)
	}
}

func TestManifestSchedule(t *testing.T) {
	t.Parallel()
	j, err := New(&fakeSource{}, &fakeWriter{}, newClock())
	if err != nil {
		t.Fatal(err)
	}
	s := j.(interface {
		Schedule() (string, string, string, time.Duration)
		RunBound() time.Duration
		RestartSensitive() bool
	})
	cal, anchor, zone, off := s.Schedule()
	if j.ID() != "macro_events" || cal != "XNYS" || anchor != "time" || zone != "America/New_York" ||
		off != 6*time.Hour || s.RunBound() != 5*time.Minute || s.RestartSensitive() {
		t.Fatalf("schedule %s %s %s %s", cal, anchor, zone, off)
	}
}
