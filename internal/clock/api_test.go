package clock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/clock/ibkrhours"
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

	_ clock.SessionDayDiffer            = fakeDiffer{}
	_ clock.NewSessionDayDifferFunc     = func() clock.SessionDayDiffer { return fakeDiffer{} }
	_ clock.CommandFunc                 = fakeCommand{}.run
	_ clock.NewCommandSessionSourceFunc = newFakeCommandSource
)

// fakeDiffer reports each row of fresh beyond old as added and each row of old beyond fresh as removed.
type fakeDiffer struct{}

func (fakeDiffer) Diff(old, fresh []contracts.SessionDay) []clock.SessionDayChange {
	var out []clock.SessionDayChange
	for i := 0; i < len(old) || i < len(fresh); i++ {
		c := clock.SessionDayChange{Day: contracts.Day("2026-11-0" + string(rune('1'+i)))}
		if i < len(old) {
			c.Before = &old[i]
		}
		if i < len(fresh) {
			c.After = &fresh[i]
		}
		if c.Before == nil || c.After == nil {
			out = append(out, c)
		}
	}
	return out
}

// fakeCommand records the argv it was run with and answers a fixed result.
type fakeCommand struct {
	got  *[]string
	code int
	err  error
}

func (f fakeCommand) run(_ context.Context, argv []string) (int, error) {
	if f.got != nil {
		*f.got = argv
	}
	return f.code, f.err
}

// commandSource is a source that fails when the command does.
type commandSource struct {
	argv []string
	run  clock.CommandFunc
}

func (s commandSource) SessionDays(ctx context.Context, id string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	code, err := s.run(ctx, append(append([]string{}, s.argv...), "--calendar", id, "--from", string(from), "--to", string(to)))
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, unknownCal{}
	}
	return nil, nil
}

func newFakeCommandSource(argv []string, _ string, run clock.CommandFunc) clock.SessionSource {
	return commandSource{argv: argv, run: run}
}

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

func TestSessionDayDifferContract(t *testing.T) {
	t.Parallel()
	var newDiffer clock.NewSessionDayDifferFunc = func() clock.SessionDayDiffer { return fakeDiffer{} }
	d := newDiffer()
	row := contracts.SessionDay{CalendarID: "XNYS", IsSession: true}
	if got := d.Diff([]contracts.SessionDay{row}, []contracts.SessionDay{row}); len(got) != 0 {
		t.Fatalf("equal slices: %+v", got)
	}
	added := d.Diff(nil, []contracts.SessionDay{row})
	if len(added) != 1 || added[0].Before != nil || added[0].After == nil {
		t.Fatalf("added: %+v", added)
	}
	removed := d.Diff([]contracts.SessionDay{row}, nil)
	if len(removed) != 1 || removed[0].Before == nil || removed[0].After != nil {
		t.Fatalf("removed: %+v", removed)
	}
}

func TestCommandSessionSourceContract(t *testing.T) {
	t.Parallel()
	var got []string
	var run clock.CommandFunc = fakeCommand{got: &got}.run
	src := clock.NewCommandSessionSourceFunc(newFakeCommandSource)([]string{"uv", "run"}, t.TempDir(), run)
	if _, err := src.SessionDays(context.Background(), "XNYS", "2026-11-02", "2026-11-30"); err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || got[0] != "uv" || got[1] != "run" {
		t.Fatalf("argv %v", got)
	}
	failing := newFakeCommandSource(nil, t.TempDir(), fakeCommand{err: context.Canceled}.run)
	if _, err := failing.SessionDays(context.Background(), "XNYS", "2026-11-02", "2026-11-02"); err != context.Canceled {
		t.Fatalf("want start error, got %v", err)
	}
}

var _ clock.IBKRHours = ibkrhours.Parser{}

type fakeHours struct{}

func (fakeHours) Parse(liquidHours, timeZoneID string) ([]clock.HoursDay, error) {
	if timeZoneID == "" {
		return nil, errors.New("empty zone")
	}
	open := time.Date(2026, 11, 13, 14, 30, 0, 0, time.UTC)
	return []clock.HoursDay{{Day: "2026-11-13", IsSession: true, OpenAt: open, CloseAt: open.Add(6*time.Hour + 30*time.Minute)}}, nil
}

func TestIBKRHoursContract(t *testing.T) {
	t.Parallel()
	var h clock.IBKRHours = fakeHours{}
	days, err := h.Parse("20261113:0930-20261113:1600", "US/Eastern")
	if err != nil || len(days) != 1 || !days[0].IsSession || days[0].Day != "2026-11-13" {
		t.Fatalf("days %+v err %v", days, err)
	}
	if _, err := h.Parse("x", ""); err == nil {
		t.Fatal("want error for empty zone")
	}
	// The real parser satisfies the same interface and meets the same two promises.
	real := ibkrhours.New()
	rd, err := real.Parse("20261113:0930-20261113:1600", "US/Eastern")
	if err != nil || len(rd) != 1 || !rd[0].OpenAt.Equal(days[0].OpenAt) || !rd[0].CloseAt.Equal(days[0].CloseAt) {
		t.Fatalf("real %+v err %v", rd, err)
	}
	if _, err := real.Parse("x", ""); err == nil {
		t.Fatal("want error for empty zone")
	}
}
