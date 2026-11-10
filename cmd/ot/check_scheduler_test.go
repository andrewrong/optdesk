package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeGapReader struct {
	exps     []store.Expectation
	gaps     []store.Gap
	err      error
	from, to contracts.Day
}

func (f *fakeGapReader) OpenGaps(context.Context) ([]store.Gap, error) { return f.gaps, f.err }
func (f *fakeGapReader) Expectations(_ context.Context, from, to contracts.Day) ([]store.Expectation, error) {
	f.from, f.to = from, to
	return f.exps, f.err
}
func (f *fakeGapReader) Close() error { return nil }

// 2026-11-10 is a Tuesday; the window is 2026-11-04 (Wed) to 2026-11-10.
var schedNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func runSched(t *testing.T, g *fakeGapReader, now time.Time) string {
	t.Helper()
	var out bytes.Buffer
	a := checkApp(t, map[string]string{"OT_HOME": "/h", store.DSNEnvPages: checkSecretDSN}, &out, nil, nil)
	a.deps.openGapReader = func(context.Context, string) (store.GapReader, error) { return g, nil }
	a.deps.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClockAt(now) }
	a.profile.Zone = time.UTC
	_ = runCheck(a, []string{"scheduler"}, map[string]checkService{"scheduler": checkScheduler})
	return out.String()
}

func okExp(ds, day string) store.Expectation {
	return store.Expectation{DatasetID: ds, ExpectedDay: contracts.Day(day), PresentDay: contracts.Day(day), DueBy: schedNow.Add(-time.Hour)}
}

func TestCheckSchedNothingPlanned(t *testing.T) {
	t.Parallel()
	g := &fakeGapReader{exps: []store.Expectation{
		{Closed: true, ExpectedDay: "2026-11-04"}, okExp("d1", "2026-11-05"), okExp("d1", "2026-11-06"),
		okExp("d1", "2026-11-09"), okExp("d1", "2026-11-10"),
	}}
	out := runSched(t, g, schedNow)
	if !strings.Contains(out, "PASS scheduler.window 2026-11-04 (nothing planned)") {
		t.Errorf("closed row: %s", out)
	}
	if g.from != "2026-11-04" || g.to != "2026-11-10" {
		t.Errorf("range %s..%s", g.from, g.to)
	}
}

func TestCheckSchedNoRows(t *testing.T) {
	t.Parallel()
	out := runSched(t, &fakeGapReader{}, schedNow)
	if !strings.Contains(out, "FAIL scheduler.window 2026-11-10 no window rows") || strings.Contains(out, "PASS") {
		t.Errorf("empty range: %s", out)
	}
	// A weekday after the last row day is FAIL; the weekend is explained.
	out = runSched(t, &fakeGapReader{exps: []store.Expectation{okExp("d1", "2026-11-05")}}, schedNow)
	if !strings.Contains(out, "FAIL scheduler.window 2026-11-09 no window rows") {
		t.Errorf("after last row: %s", out)
	}
}

func TestCheckSchedNonSessionDay(t *testing.T) {
	t.Parallel()
	g := &fakeGapReader{exps: []store.Expectation{
		okExp("d1", "2026-11-04"), okExp("d1", "2026-11-06"), okExp("d1", "2026-11-09"), okExp("d1", "2026-11-10"),
	}}
	out := runSched(t, g, schedNow)
	for _, want := range []string{
		"EXPLAINED scheduler.window 2026-11-05 non_session_day", // between two row days
		"EXPLAINED scheduler.window 2026-11-07 non_session_day", // Saturday
		"EXPLAINED scheduler.window 2026-11-08 non_session_day", // Sunday
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("unexpected FAIL: %s", out)
	}
}

func TestCheckSchedUnexplainedGap(t *testing.T) {
	t.Parallel()
	stale := okExp("d1", "2026-11-10")
	stale.PresentDay = "2026-11-09"
	empty := okExp("d2", "2026-11-10")
	empty.PresentDay = ""
	g := &fakeGapReader{
		exps: []store.Expectation{okExp("d1", "2026-11-04"), okExp("d1", "2026-11-05"), okExp("d1", "2026-11-06"), okExp("d1", "2026-11-09"), stale, empty},
		gaps: []store.Gap{
			{JobID: "j", DatasetID: "d1", Day: "2026-11-09", Reason: "expired"},
			{JobID: "j", DatasetID: "d2", Day: "2026-11-09", Reason: ""},
			{JobID: "j", DatasetID: "d3", Day: "2026-11-09", Reason: "closed_day"},
		},
	}
	out := runSched(t, g, schedNow)
	for _, want := range []string{
		"FAIL scheduler.window.d1 2026-11-10", "FAIL scheduler.window.d2 2026-11-10",
		"FAIL scheduler.gap.d1 2026-11-09 expired", "FAIL scheduler.gap.d2 2026-11-09",
		"EXPLAINED scheduler.gap.d3 2026-11-09 closed_day",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestCheckSchedWindowOpen(t *testing.T) {
	t.Parallel()
	open := store.Expectation{DatasetID: "d1", ExpectedDay: "2026-11-10", PresentDay: "2026-11-09", DueBy: schedNow.Add(time.Hour)}
	g := &fakeGapReader{exps: []store.Expectation{okExp("d1", "2026-11-09"), open}}
	out := runSched(t, g, schedNow)
	if !strings.Contains(out, "EXPLAINED scheduler.window.d1 2026-11-10 window_open") {
		t.Errorf("open window: %s", out)
	}
}

func TestCheckSchedAsOfDay(t *testing.T) {
	t.Parallel()
	g := &fakeGapReader{exps: []store.Expectation{okExp("d1", "2026-11-09"), okExp("d1", "2026-11-10")}}
	out := runSched(t, g, schedNow)
	if !strings.Contains(out, "PASS scheduler.window.d1 2026-11-09 ") || strings.Contains(out, "PASS scheduler.window.d1 2026-11-11") {
		t.Errorf("as-of day: %s", out)
	}
	// An open gap carries the gap's own day, not the day of the run.
	g.gaps = []store.Gap{{DatasetID: "d1", Day: "2026-11-06", Reason: "window_open"}}
	out = runSched(t, g, schedNow)
	if !strings.Contains(out, "EXPLAINED scheduler.gap.d1 2026-11-06 window_open") {
		t.Errorf("gap day: %s", out)
	}
	// A reader that fails is a FAIL line.
	out = runSched(t, &fakeGapReader{err: errors.New("boom")}, schedNow)
	if !strings.Contains(out, "FAIL scheduler.window") {
		t.Errorf("reader error: %s", out)
	}
}

func TestCheckSchedNoView(t *testing.T) {
	t.Parallel()
	stale := okExp("d1", "2026-11-09")
	stale.PresentDay = "2026-11-06"
	g := &fakeGapReader{exps: []store.Expectation{stale, okExp("d1", "2026-11-10")}}
	out := runSched(t, g, schedNow)
	for _, want := range []string{
		"EXPLAINED scheduler.cursor 2026-11-09 no_view",
		"EXPLAINED scheduler.keys - no_view",
		"EXPLAINED scheduler.runs - no_view",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}
