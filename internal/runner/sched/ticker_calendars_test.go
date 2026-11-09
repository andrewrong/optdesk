package sched

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/jonboulle/clockwork"
)

// jpxJob is a tickerJob on the calendar XJPX.
type jpxJob struct{ tickerJob }

func (j jpxJob) Schedule() (string, string, string, time.Duration) {
	return "XJPX", "session_close", "", 0
}
func (j jpxJob) Window() (string, time.Duration, time.Duration) { return "XJPX", 0, 0 }

func TestWalkedMergesCalendars(t *testing.T) {
	t.Parallel()
	var rows []contracts.SessionDay
	for _, d := range []string{"2026-11-24", "2026-11-25", "2026-11-27"} {
		rows = append(rows, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd(d), IsSession: true})
	}
	for _, d := range []string{"2026-11-25", "2026-11-26"} {
		rows = append(rows, contracts.SessionDay{CalendarID: "XJPX", SessionDay: sd(d), IsSession: true})
	}
	cal := fakeCal{rows: rows}
	clk := clockwork.NewFakeClockAt(utc("2026-11-27T12:00:00Z"))
	jobs := []contracts.Job{tickerJob{id: "us"}, jpxJob{tickerJob{id: "jp"}}}
	tk := New(newFakeFence(), &fakeLedger{}, &fakeOps{}, cal, clk, nil, nil, jobs, nil, nil).(*ticker)
	got := tk.walked(context.Background(), "2026-11-24", "2026-11-27")
	want := []contracts.Day{"2026-11-25", "2026-11-26", "2026-11-27"}
	if len(got) != len(want) {
		t.Fatalf("walked = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("walked = %v, want %v", got, want)
		}
	}
	// With no cursor, today alone, when it is a session of one calendar: 2026-11-26 is XJPX only.
	if got := tk.walked(context.Background(), "", "2026-11-26"); len(got) != 1 || got[0] != "2026-11-26" {
		t.Fatalf("walked with no cursor = %v", got)
	}
}
