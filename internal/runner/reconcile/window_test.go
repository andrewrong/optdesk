package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestWindowThanksgivingAndEarlyClose(t *testing.T) {
	t.Parallel()
	cal := fakeCalendar{sessions: []clock.Session{
		{CalendarID: "XNYS", Day: "2026-11-25", IsSession: true, OpenAt: utc("2026-11-25T14:30:00Z"), CloseAt: utc("2026-11-25T21:00:00Z")},
		// 2026-11-26 is Thanksgiving: not a session.
		{CalendarID: "XNYS", Day: "2026-11-27", IsSession: true, EarlyClose: true, OpenAt: utc("2026-11-27T14:30:00Z"), CloseAt: utc("2026-11-27T18:00:00Z")},
	}}
	rule := runner.WindowRule{CalendarID: "XNYS", CloseOffset: time.Hour}
	// A session with an early CloseAt: the window close is that instant plus the offset given.
	early := cal.sessions[1]
	if w, ok := windowFor(runner.WindowRule{CloseOffset: 5 * time.Hour}, "ds", early); !ok || w.CloseAt != early.CloseAt.Add(5*time.Hour) {
		t.Fatalf("early close window = %+v ok=%v", w, ok)
	}
	if _, ok := windowFor(rule, "ds", clock.Session{Day: "2026-11-26"}); ok {
		t.Fatal("non-session day got a window")
	}
	tests := []struct {
		name    string
		day     contracts.Day
		cursor  contracts.Day
		wantPut []store.Window
	}{
		{"normal day", "2026-11-25", "2026-11-24", []store.Window{{DatasetID: "ds", Day: "2026-11-25",
			OpenAt:  utc("2026-11-25T14:30:00Z"),
			CloseAt: utc("2026-11-25T22:00:00Z")}}},
		{"thanksgiving", "2026-11-26", "2026-11-25", nil},
		// early close 13:00 ET = 18:00 UTC, plus the one hour offset of the rule.
		{"early close", "2026-11-27", "2026-11-26", []store.Window{{DatasetID: "ds", Day: "2026-11-27",
			OpenAt:  utc("2026-11-27T14:30:00Z"),
			CloseAt: utc("2026-11-27T19:00:00Z")}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			led := newLedger()
			led.cursor = store.Cursor{Day: tc.cursor}
			job := &fakeJob{keys: []contracts.ExpectedKey{{DatasetID: "ds", SubjectKey: "a"}}}
			clk := clockwork.NewFakeClockAt(utc("2026-11-25T12:00:00Z"))
			r := NewWithRule(led, cal, clk, []contracts.Job{job}, rule)
			if err := r.Reconcile(context.Background(), tc.day); err != nil {
				t.Fatal(err)
			}
			if len(led.puts) != len(tc.wantPut) {
				t.Fatalf("puts = %v, want %v", led.puts, tc.wantPut)
			}
			for i, w := range tc.wantPut {
				if led.puts[i] != w {
					t.Fatalf("put %d = %+v, want %+v", i, led.puts[i], w)
				}
			}
		})
	}
}

func TestWindowRuleOffsets(t *testing.T) {
	t.Parallel()
	s := clock.Session{Day: "2026-11-27", IsSession: true, OpenAt: utc("2026-11-27T14:30:00Z"), CloseAt: utc("2026-11-27T18:00:00Z")}
	rule := runner.WindowRule{OpenOffset: -30 * time.Minute}
	w, ok := windowFor(rule, "ds", s)
	if !ok || w.OpenAt != utc("2026-11-27T14:00:00Z") || w.CloseAt != s.CloseAt || w.DatasetID != "ds" || w.Day != s.Day {
		t.Fatalf("window = %+v ok=%v", w, ok)
	}
	if _, ok := windowFor(rule, "ds", clock.Session{Day: "2026-11-26"}); ok {
		t.Fatal("non-session day got a window")
	}
}

func windowCal() fakeCalendar {
	return fakeCalendar{sessions: []clock.Session{
		{CalendarID: "XNYS", Day: "2026-11-04", IsSession: true, OpenAt: utc("2026-11-04T14:30:00Z"), CloseAt: utc("2026-11-04T21:00:00Z")},
		// early close 13:00 ET = 18:00 UTC
		{CalendarID: "XNYS", Day: "2026-11-27", IsSession: true, EarlyClose: true, OpenAt: utc("2026-11-27T14:30:00Z"), CloseAt: utc("2026-11-27T18:00:00Z")},
	}}
}

func keyOf(ds string) contracts.ExpectedKey {
	return contracts.ExpectedKey{DatasetID: ds, SubjectKey: "a"}
}

func TestWindowPerJob(t *testing.T) {
	t.Parallel()
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	short := windowedJob{fakeJob: &fakeJob{id: "short", keys: []contracts.ExpectedKey{keyOf("d5")}}, close: 5 * time.Hour}
	long := windowedJob{fakeJob: &fakeJob{id: "long", keys: []contracts.ExpectedKey{keyOf("d96")}}, close: 96 * time.Hour}
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	r := New(led, windowCal(), clk, []contracts.Job{short, long})
	if err := r.Reconcile(context.Background(), "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	got := map[string]time.Time{}
	for _, w := range led.puts {
		got[w.DatasetID] = w.CloseAt
	}
	cl := utc("2026-11-04T21:00:00Z")
	if len(led.puts) != 2 || got["d5"] != cl.Add(5*time.Hour) || got["d96"] != cl.Add(96*time.Hour) {
		t.Fatalf("puts = %+v", led.puts)
	}
}

func TestWindowedJobWithoutMethod(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		rule runner.WindowRule
		want time.Time
	}{
		{"zero rule", runner.WindowRule{}, utc("2026-11-04T21:00:00Z")},
		{"rule offset", runner.WindowRule{CloseOffset: 96 * time.Hour}, utc("2026-11-08T21:00:00Z")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			led := newLedger()
			led.cursor = store.Cursor{Day: "2026-11-03"}
			job := &fakeJob{keys: []contracts.ExpectedKey{keyOf("ds")}}
			clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
			r := NewWithRule(led, windowCal(), clk, []contracts.Job{job}, tc.rule)
			if err := r.Reconcile(context.Background(), "2026-11-04"); err != nil {
				t.Fatal(err)
			}
			if len(led.puts) != 1 || led.puts[0].CloseAt != tc.want {
				t.Fatalf("puts = %+v, want close %v", led.puts, tc.want)
			}
		})
	}
}

func TestReconcileEarlyCloseOptionWindow(t *testing.T) {
	t.Parallel()
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-26"}
	job := windowedJob{fakeJob: &fakeJob{id: "option_eod", keys: []contracts.ExpectedKey{keyOf("obs.option_eod")}}, close: 5 * time.Hour}
	clk := clockwork.NewFakeClockAt(utc("2026-11-27T12:00:00Z"))
	r := New(led, windowCal(), clk, []contracts.Job{job})
	if err := r.Reconcile(context.Background(), "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	// 16:23 America/New_York on 2026-11-27 is 21:23 UTC.
	if len(led.puts) != 1 || led.puts[0].CloseAt.Before(utc("2026-11-27T21:23:00Z")) {
		t.Fatalf("puts = %+v, want close at or after 16:23 local", led.puts)
	}
}

func TestEmptyRetriedUntilOwnClose(t *testing.T) {
	t.Parallel()
	empty := map[contracts.ExpectedKey]contracts.JobResult{keyOf("ds"): {}}
	run := func(t *testing.T, now string, job contracts.Job) (*fakeLedger, map[contracts.ExpectedKey]store.PresentStatus) {
		t.Helper()
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-03"}
		clk := clockwork.NewFakeClockAt(utc(now))
		if err := New(led, windowCal(), clk, []contracts.Job{job}).Reconcile(context.Background(), "2026-11-04"); err != nil {
			t.Fatal(err)
		}
		p, _ := led.Present(context.Background(), "2026-11-04")
		return led, p
	}
	own := func() contracts.Job {
		return windowedJob{fakeJob: &fakeJob{keys: []contracts.ExpectedKey{keyOf("ds")}, results: empty}, close: 5 * time.Hour}
	}
	t.Run("before own close", func(t *testing.T) {
		t.Parallel()
		led, p := run(t, "2026-11-04T22:00:00Z", own())
		if len(p) != 0 {
			t.Fatalf("present = %v, want unmarked", p)
		}
		runs, _ := led.Runs(context.Background(), "2026-11-04")
		if len(runs) != 1 || runs[0].ReasonCode != "empty_before_close" {
			t.Fatalf("runs = %+v", runs)
		}
	})
	t.Run("after own close", func(t *testing.T) {
		t.Parallel()
		_, p := run(t, "2026-11-05T03:00:00Z", own())
		if p[keyOf("ds")] != store.StatusNoValue {
			t.Fatalf("present = %v, want no_value", p)
		}
	})
	t.Run("job without window", func(t *testing.T) {
		t.Parallel()
		_, p := run(t, "2026-11-04T22:00:00Z", &fakeJob{keys: []contracts.ExpectedKey{keyOf("ds")}, results: empty})
		if p[keyOf("ds")] != store.StatusNoValue {
			t.Fatalf("present = %v, want no_value at once", p)
		}
	})
}
