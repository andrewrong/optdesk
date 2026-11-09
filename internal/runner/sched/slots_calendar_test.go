package sched

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

// runOutCal wraps fakeCal: XJPX answers its rows and then UnknownCalendar for every day after lastJPX.
type runOutCal struct {
	fakeCal
	lastJPX contracts.Day
	xnysErr bool
}

func (c runOutCal) Session(ctx context.Context, id string, day contracts.Day) (clock.Session, error) {
	if (id == "XJPX" && day > c.lastJPX) || (id == "XNYS" && c.xnysErr && day > "2026-11-10") {
		return clock.Session{}, unknownErr{}
	}
	return c.fakeCal.Session(ctx, id, day)
}

func runOutRows() []contracts.SessionDay {
	var rows []contracts.SessionDay
	for _, d := range []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"} {
		rows = append(rows, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd(d), IsSession: true,
			OpenAt: utc(d + "T14:30:00Z"), CloseAt: utc(d + "T21:00:00Z")})
	}
	for _, d := range []string{"2026-11-10", "2026-11-11"} {
		rows = append(rows, contracts.SessionDay{CalendarID: "XJPX", SessionDay: sd(d), IsSession: true,
			OpenAt: utc(d + "T00:00:00Z"), CloseAt: utc(d + "T06:30:00Z")})
	}
	return rows
}

func TestSlotsDropRunOutCalendar(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	slots := []runner.Slot{
		{JobID: "us", CalendarID: "XNYS", Anchor: "session_close"},
		{JobID: "jp", CalendarID: "XJPX", Anchor: "session_close"},
	}
	known := map[string]bool{"XJPX": true}
	newCal := func() runOutCal { return runOutCal{fakeCal: fakeCal{rows: runOutRows()}, lastJPX: "2026-11-11"} }
	byJob := func(s []Start, id string) int {
		n := 0
		for _, x := range s {
			if x.JobID == id {
				n++
			}
		}
		return n
	}
	t.Run("drops after its last row", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		got, err := nextStarts(ctx, newCal(), slots, utc("2026-11-10T00:00:00Z"), 4, known, slog.New(slog.NewTextHandler(&buf, nil)))
		if err != nil {
			t.Fatal(err)
		}
		if byJob(got, "us") != 4 || byJob(got, "jp") != 2 {
			t.Fatalf("starts = %v, want 4 XNYS and the 2 XJPX slots before its last row", got)
		}
		if n := strings.Count(buf.String(), "jobs dropped"); n != 1 || !strings.Contains(buf.String(), "XJPX") {
			t.Fatalf("log = %q, want one line naming XJPX", buf.String())
		}
	})
	t.Run("a call after the last row", func(t *testing.T) {
		t.Parallel()
		got, err := nextStarts(ctx, newCal(), slots, utc("2026-11-12T00:00:00Z"), 2, known, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
		if err != nil || byJob(got, "us") != 2 || byJob(got, "jp") != 0 {
			t.Fatalf("starts = %v err = %v, want the XNYS slots only", got, err)
		}
	})
	t.Run("error on the rule calendar", func(t *testing.T) {
		t.Parallel()
		cal := newCal()
		cal.xnysErr = true
		_, err := nextStarts(ctx, cal, slots, utc("2026-11-10T00:00:00Z"), 4, known, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
		if err == nil || !strings.Contains(err.Error(), "us") {
			t.Fatalf("err = %v, want one naming the job", err)
		}
	})
	t.Run("calendar outside the known set", func(t *testing.T) {
		t.Parallel()
		_, err := nextStarts(ctx, newCal(), slots, utc("2026-11-10T00:00:00Z"), 4, map[string]bool{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
		if err == nil || !strings.Contains(err.Error(), "jp") {
			t.Fatalf("err = %v, want one naming the job", err)
		}
	})
	t.Run("exported NextStarts drops nothing", func(t *testing.T) {
		t.Parallel()
		_, err := NextStarts(ctx, newCal(), slots, utc("2026-11-10T00:00:00Z"), 4)
		if err == nil || !strings.Contains(err.Error(), "jp") {
			t.Fatalf("err = %v, want one naming the job", err)
		}
	})
}
