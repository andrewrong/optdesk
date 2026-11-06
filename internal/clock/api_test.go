package clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type fakeSource struct{ rows []contracts.SessionDay }

func (f fakeSource) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	return f.rows, nil
}

type unknownCal struct{}

func (unknownCal) Error() string             { return "unknown calendar" }
func (unknownCal) Kind() contracts.ErrorKind { return contracts.UnknownCalendar }

type fakeCalendar struct{ src clock.SessionSource }

func (c fakeCalendar) Session(ctx context.Context, id string, day contracts.Day) (clock.Session, error) {
	if id != "XNYS" {
		return clock.Session{}, unknownCal{}
	}
	rows, err := c.src.SessionDays(ctx, id, day, day)
	if err != nil || len(rows) == 0 {
		return clock.Session{}, err
	}
	return clock.Session{CalendarID: id, Day: day, IsSession: rows[0].IsSession, OpenAt: rows[0].OpenAt, CloseAt: rows[0].CloseAt}, nil
}

func (c fakeCalendar) NextSession(ctx context.Context, id string, day contracts.Day) (clock.Session, error) {
	return c.Session(ctx, id, day)
}

// Fakes assigned to the declared function types and interfaces.
var (
	_ clock.Clock           = clockwork.NewFakeClock()
	_ clock.Calendar        = fakeCalendar{}
	_ clock.SessionSource   = fakeSource{}
	_ clock.NewClockFunc    = func() clock.Clock { return clockwork.NewFakeClock() }
	_ clock.NewCalendarFunc = func(s clock.SessionSource) clock.Calendar { return fakeCalendar{s} }
)

func TestClockContract(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	var c clock.Clock = clockwork.NewFakeClockAt(start)
	if !c.Now().Equal(start) {
		t.Fatalf("Now = %v", c.Now())
	}
}

func TestCalendarContract(t *testing.T) {
	t.Parallel()
	open := time.Date(2026, 11, 4, 14, 30, 0, 0, time.UTC)
	src := fakeSource{rows: []contracts.SessionDay{{CalendarID: "XNYS", IsSession: true, OpenAt: open, CloseAt: open.Add(6*time.Hour + 30*time.Minute)}}}
	var cal clock.Calendar = fakeCalendar{src}
	s, err := cal.Session(context.Background(), "XNYS", "2026-11-04")
	if err != nil || !s.IsSession || !s.OpenAt.Equal(open) {
		t.Fatalf("session %+v err %v", s, err)
	}
	_, err = cal.Session(context.Background(), "NOPE", "2026-11-04")
	te, ok := err.(contracts.TypedError)
	if !ok || te.Kind() != contracts.UnknownCalendar {
		t.Fatalf("want UnknownCalendar, got %v", err)
	}
}
