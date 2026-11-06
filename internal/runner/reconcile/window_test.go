package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
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
	rule := WindowRule{CalendarID: "XNYS", CloseOffset: time.Hour}
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
	rule := WindowRule{OpenOffset: -30 * time.Minute}
	w, ok := rule.Window("ds", s)
	if !ok || w.OpenAt != utc("2026-11-27T14:00:00Z") || w.CloseAt != s.CloseAt || w.DatasetID != "ds" || w.Day != s.Day {
		t.Fatalf("window = %+v ok=%v", w, ok)
	}
	if _, ok := rule.Window("ds", clock.Session{Day: "2026-11-26"}); ok {
		t.Fatal("non-session day got a window")
	}
}
