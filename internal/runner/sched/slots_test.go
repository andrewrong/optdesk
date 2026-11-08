package sched

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/atombender/go-jsonschema/pkg/types"
)

// fakeCal answers from SessionDay rows; a calendar with no rows is unknown, a day with no row is no session.
type fakeCal struct{ rows []contracts.SessionDay }

type unknownErr struct{}

func (unknownErr) Error() string             { return "unknown calendar" }
func (unknownErr) Kind() contracts.ErrorKind { return contracts.UnknownCalendar }

func (f fakeCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	known := false
	for _, r := range f.rows {
		if r.CalendarID != id {
			continue
		}
		known = true
		if r.SessionDay.Format(dayLayout) == string(day) {
			return clock.Session{CalendarID: id, Day: day, IsSession: r.IsSession, OpenAt: r.OpenAt, CloseAt: r.CloseAt, EarlyClose: r.EarlyClose}, nil
		}
	}
	if !known {
		return clock.Session{}, unknownErr{}
	}
	return clock.Session{CalendarID: id, Day: day}, nil
}

type noValueErr struct{}

func (noValueErr) Error() string             { return "no later session" }
func (noValueErr) Kind() contracts.ErrorKind { return contracts.NoValue }

// NextSession returns the first session row of the calendar after day, in row order by date; a calendar
// with no rows is unknown and none after day is a NoValue error.
func (f fakeCal) NextSession(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	known := false
	var best *contracts.SessionDay
	for i, r := range f.rows {
		if r.CalendarID != id {
			continue
		}
		known = true
		d := r.SessionDay.Format(dayLayout)
		if !r.IsSession || d <= string(day) {
			continue
		}
		if best == nil || d < best.SessionDay.Format(dayLayout) {
			best = &f.rows[i]
		}
	}
	if !known {
		return clock.Session{}, unknownErr{}
	}
	if best == nil {
		return clock.Session{}, noValueErr{}
	}
	return clock.Session{CalendarID: id, Day: contracts.Day(best.SessionDay.Format(dayLayout)), IsSession: true,
		OpenAt: best.OpenAt, CloseAt: best.CloseAt, EarlyClose: best.EarlyClose}, nil
}

func sd(day string) types.SerializableDate {
	d, err := time.Parse(dayLayout, day)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: d}
}

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestSlotsUSCloseMinus30OnEarlyClose(t *testing.T) {
	t.Parallel()
	cal := fakeCal{rows: []contracts.SessionDay{
		{CalendarID: "XNYS", SessionDay: sd("2025-11-26"), IsSession: true, OpenAt: utc("2025-11-26T14:30:00Z"), CloseAt: utc("2025-11-26T21:00:00Z")},
		{CalendarID: "XNYS", SessionDay: sd("2025-11-28"), IsSession: true, EarlyClose: true,
			OpenAt: utc("2025-11-28T14:30:00Z"), CloseAt: utc("2025-11-28T18:00:00Z")},
	}}
	slot := runner.Slot{JobID: "eod", CalendarID: "XNYS", Anchor: "session_close", Offset: -30 * time.Minute}
	got, err := NextStarts(context.Background(), cal, []runner.Slot{slot}, utc("2025-11-26T00:00:00Z"), 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []Start{
		{JobID: "eod", At: utc("2025-11-26T20:30:00Z")},
		{JobID: "eod", At: utc("2025-11-28T17:30:00Z")}, // 13:00 ET close gives 12:30 ET
	}
	assertStarts(t, got, want)
}

func TestSlotsDSTWeek(t *testing.T) {
	t.Parallel()
	var rows []contracts.SessionDay
	for _, d := range []string{"2025-03-07", "2025-03-08", "2025-03-09", "2025-03-10", "2025-03-11"} {
		rows = append(rows, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd(d), IsSession: true})
	}
	cal := fakeCal{rows: rows}
	slot := runner.Slot{JobID: "morning", CalendarID: "XNYS", Anchor: "time", Zone: "America/New_York", Offset: 9*time.Hour + 30*time.Minute}
	got, err := NextStarts(context.Background(), cal, []runner.Slot{slot}, utc("2025-03-07T00:00:00Z"), 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []Start{
		{"morning", utc("2025-03-07T14:30:00Z")},
		{"morning", utc("2025-03-08T14:30:00Z")},
		{"morning", utc("2025-03-09T13:30:00Z")},
		{"morning", utc("2025-03-10T13:30:00Z")},
		{"morning", utc("2025-03-11T13:30:00Z")},
	}
	assertStarts(t, got, want)
}

func TestSlotsKLNoDST(t *testing.T) {
	t.Parallel()
	cal := fakeCal{rows: []contracts.SessionDay{
		{CalendarID: "XKLS", SessionDay: sd("2025-03-09"), IsSession: true},
		{CalendarID: "XKLS", SessionDay: sd("2025-07-09"), IsSession: true},
	}}
	slot := runner.Slot{JobID: "kl", CalendarID: "XKLS", Anchor: "time", Zone: "Asia/Singapore", Offset: 17 * time.Hour}
	for _, day := range []string{"2025-03-09", "2025-07-09"} {
		got, err := NextStarts(context.Background(), cal, []runner.Slot{slot}, utc(day+"T00:00:00Z"), 1)
		if err != nil {
			t.Fatal(err)
		}
		assertStarts(t, got, []Start{{"kl", utc(day + "T09:00:00Z")}})
	}
}

func TestSlotsNoSessionNoStart(t *testing.T) {
	t.Parallel()
	cal := fakeCal{rows: []contracts.SessionDay{
		{CalendarID: "XNYS", SessionDay: sd("2025-11-27"), IsSession: false}, // Thanksgiving
	}}
	slots := []runner.Slot{
		{JobID: "a", CalendarID: "XNYS", Anchor: "session_open"},
		{JobID: "b", CalendarID: "XNYS", Anchor: "session_close"},
		{JobID: "c", CalendarID: "XNYS", Anchor: "time", Zone: "America/New_York", Offset: 9 * time.Hour},
	}
	got, err := NextStarts(context.Background(), cal, slots, utc("2025-11-27T00:00:00Z"), 1)
	if err != nil {
		t.Fatal(err)
	}
	assertStarts(t, got, nil)
}

func TestSlotsUnknownZone(t *testing.T) {
	t.Parallel()
	cal := fakeCal{rows: []contracts.SessionDay{{CalendarID: "XNYS", SessionDay: sd("2025-11-26"), IsSession: true}}}
	slot := runner.Slot{JobID: "badzone", CalendarID: "XNYS", Anchor: "time", Zone: "Mars/Olympus", Offset: time.Hour}
	_, err := NextStarts(context.Background(), cal, []runner.Slot{slot}, utc("2025-11-26T00:00:00Z"), 1)
	if err == nil || !strings.Contains(err.Error(), "badzone") {
		t.Fatalf("want error naming the job, got %v", err)
	}
}

func TestSlotsUnknownCalendarAndAnchor(t *testing.T) {
	t.Parallel()
	cal := fakeCal{rows: []contracts.SessionDay{{CalendarID: "XNYS", SessionDay: sd("2025-11-26"), IsSession: true}}}
	for _, s := range []runner.Slot{
		{JobID: "nocal", CalendarID: "NOPE", Anchor: "session_open"},
		{JobID: "noanchor", CalendarID: "XNYS", Anchor: "sunrise"},
	} {
		_, err := NextStarts(context.Background(), cal, []runner.Slot{s}, utc("2025-11-26T00:00:00Z"), 1)
		if err == nil || !strings.Contains(err.Error(), s.JobID) {
			t.Fatalf("%s: want error naming the job, got %v", s.JobID, err)
		}
	}
}

func assertStarts(t *testing.T, got, want []Start) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].JobID != want[i].JobID || !got[i].At.Equal(want[i].At) || got[i].At.Location() != time.UTC {
			t.Fatalf("start %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
