package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ clock.NewCalendarFunc = NewCalendar

// row types one session_day row; the instants are typed expectations, UTC.
func row(day string, isSession bool, open, closeAt string, early bool) contracts.SessionDay {
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	r := contracts.SessionDay{CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d}, IsSession: isSession, EarlyClose: early}
	if isSession {
		r.OpenAt = mustTime(open)
		r.CloseAt = mustTime(closeAt)
	}
	return r
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// fakeSource serves typed rows of calendar XNYS only, filtered to the asked range.
type fakeSource struct {
	rows  []contracts.SessionDay
	calls int
}

func (f *fakeSource) SessionDays(_ context.Context, id string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	f.calls++
	if id != "XNYS" {
		return nil, nil
	}
	var out []contracts.SessionDay
	for _, r := range f.rows {
		d := r.SessionDay.Format("2006-01-02")
		if d >= string(from) && d <= string(to) {
			out = append(out, r)
		}
	}
	return out, nil
}

func typedRows() *fakeSource {
	return &fakeSource{rows: []contracts.SessionDay{
		row("2026-10-30", true, "2026-10-30T13:30:00Z", "2026-10-30T20:00:00Z", false),
		row("2026-10-31", false, "", "", false),
		row("2026-11-01", false, "", "", false),
		row("2026-11-02", true, "2026-11-02T14:30:00Z", "2026-11-02T21:00:00Z", false),
		row("2026-11-03", true, "2026-11-03T14:30:00Z", "2026-11-03T21:00:00Z", false),
		row("2026-11-04", true, "2026-11-04T14:30:00Z", "2026-11-04T21:00:00Z", false),
		row("2026-11-05", true, "2026-11-05T14:30:00Z", "2026-11-05T21:00:00Z", false),
		row("2026-11-06", true, "2026-11-06T14:30:00Z", "2026-11-06T21:00:00Z", false),
		row("2026-11-07", false, "", "", false),
		row("2026-11-08", false, "", "", false),
		row("2026-11-25", true, "2026-11-25T14:30:00Z", "2026-11-25T21:00:00Z", false),
		row("2026-11-26", false, "", "", false),
		row("2026-11-27", true, "2026-11-27T14:30:00Z", "2026-11-27T18:00:00Z", true),
		row("2026-11-28", false, "", "", false),
	}}
}

func etClose(t *testing.T, s clock.Session) string {
	t.Helper()
	et, err := InZone(s.CloseAt, "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	return et.Format("15:04")
}

func TestSessionTypedRows(t *testing.T) {
	t.Parallel()
	var c clock.Clock = clockwork.NewFakeClockAt(mustTime("2026-11-04T12:00:00Z"))
	_ = c.Now()
	cal := NewCalendar(typedRows())
	ctx := context.Background()

	s, err := cal.Session(ctx, "XNYS", "2026-11-26")
	if err != nil || s.IsSession {
		t.Fatalf("Thanksgiving: %+v %v", s, err)
	}
	s, err = cal.Session(ctx, "XNYS", "2026-11-27")
	if err != nil || !s.IsSession || !s.EarlyClose || etClose(t, s) != "13:00" {
		t.Fatalf("day after Thanksgiving: %+v %v", s, err)
	}
	s, err = cal.Session(ctx, "XNYS", "2026-11-04")
	if err != nil || !s.IsSession || s.EarlyClose || etClose(t, s) != "16:00" || s.Day != "2026-11-04" || s.CalendarID != "XNYS" {
		t.Fatalf("ordinary day: %+v %v", s, err)
	}
}

func TestDSTOpenAt(t *testing.T) {
	t.Parallel()
	cal := NewCalendar(typedRows())
	for day, want := range map[contracts.Day]string{
		"2026-11-02": "14:30", "2026-11-06": "14:30", "2026-10-30": "13:30",
	} {
		s, err := cal.Session(context.Background(), "XNYS", day)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.OpenAt.UTC().Format("15:04"); got != want {
			t.Errorf("%s open_at UTC = %s, want %s", day, got, want)
		}
	}
}

func TestNextSessionSkipsHolidays(t *testing.T) {
	t.Parallel()
	cal := NewCalendar(typedRows())
	for _, tc := range []struct{ from, want contracts.Day }{
		{"2026-10-30", "2026-11-02"}, // weekend
		{"2026-11-25", "2026-11-27"}, // holiday
		{"2026-11-26", "2026-11-27"}, // from a non-session day
	} {
		s, err := cal.NextSession(context.Background(), "XNYS", tc.from)
		if err != nil || s.Day != tc.want || !s.IsSession {
			t.Errorf("from %s: %+v %v, want %s", tc.from, s, err, tc.want)
		}
	}
	// After 11-27 only a non-session row and then nothing: no value, not a guess.
	_, err := cal.NextSession(context.Background(), "XNYS", "2026-11-27")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.NoValue {
		t.Errorf("past the data: want NoValue, got %v", err)
	}
}

func TestUnknownCalendar(t *testing.T) {
	t.Parallel()
	cal := NewCalendar(typedRows())
	_, err1 := cal.Session(context.Background(), "NOPE", "2026-11-04")
	_, err2 := cal.NextSession(context.Background(), "NOPE", "2026-11-04")
	for _, err := range []error{err1, err2} {
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.UnknownCalendar {
			t.Errorf("want UnknownCalendar, got %v", err)
		}
	}
}

func TestZoneConversions(t *testing.T) {
	t.Parallel()
	instant := mustTime("2026-11-02T14:30:00Z")
	tests := []struct {
		name string
		conv func(time.Time) (time.Time, error)
		want string
	}{
		{"ET", func(x time.Time) (time.Time, error) { return ET(x), nil }, "2026-11-02 09:30 EST"},
		{"JST", func(x time.Time) (time.Time, error) { return JST(x), nil }, "2026-11-02 23:30 JST"},
		{"owner zone given by caller", func(x time.Time) (time.Time, error) { return InZone(x, "Europe/Berlin") }, "2026-11-02 15:30 CET"},
		{"summer ET", func(time.Time) (time.Time, error) {
			return ET(mustTime("2026-10-30T13:30:00Z")), nil
		}, "2026-10-30 09:30 EDT"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.conv(instant)
			if err != nil {
				t.Fatal(err)
			}
			if s := got.Format("2006-01-02 15:04 MST"); s != tc.want {
				t.Errorf("got %s, want %s", s, tc.want)
			}
		})
	}
	_, err := InZone(instant, "Not/AZone")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Errorf("bad zone: want Permanent, got %v", err)
	}
}
