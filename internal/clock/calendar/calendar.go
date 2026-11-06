// Package calendar answers session questions from a clock.SessionSource.
// It holds no open or close time of its own: every instant comes from the source rows.
package calendar

import (
	"context"
	"fmt"
	"time"
	_ "time/tzdata" // zone rules travel with the binary

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout = "2006-01-02"
	// window is the number of days NextSession asks the source for at a time; maxWindows bounds the search.
	window     = 14
	maxWindows = 4
)

// Fixed zones of the markets this system reads.
const (
	zoneET  = "America/New_York"
	zoneJST = "Asia/Tokyo"
)

type typedError struct {
	kind contracts.ErrorKind
	msg  string
	err  error
}

func (e *typedError) Error() string {
	if e.err != nil {
		return e.msg + ": " + e.err.Error()
	}
	return e.msg
}
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }
func (e *typedError) Unwrap() error             { return e.err }

type cal struct{ src clock.SessionSource }

// NewCalendar returns a Calendar that reads sessions from src. A calendar the source
// has no rows for is unknown; a known calendar has a row for every day, session or not.
func NewCalendar(src clock.SessionSource) clock.Calendar { return cal{src: src} }

// Session answers one day; an empty answer from the source is UnknownCalendar.
func (c cal) Session(ctx context.Context, calendarID string, day contracts.Day) (clock.Session, error) {
	rows, err := c.src.SessionDays(ctx, calendarID, day, day)
	if err != nil {
		return clock.Session{}, &typedError{kind: contracts.Retryable, msg: "session source", err: err}
	}
	for _, r := range rows {
		if r.SessionDay.Format(dayLayout) == string(day) {
			return toSession(r, day), nil
		}
	}
	return clock.Session{}, unknown(calendarID)
}

// NextSession returns the first session strictly after day. Nothing at all in the first window
// means UnknownCalendar; no session within the searched span means NoValue.
func (c cal) NextSession(ctx context.Context, calendarID string, day contracts.Day) (clock.Session, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return clock.Session{}, &typedError{kind: contracts.Permanent, msg: "bad day " + string(day), err: err}
	}
	for w := 0; w < maxWindows; w++ {
		from := d.AddDate(0, 0, 1+w*window)
		to := from.AddDate(0, 0, window-1)
		rows, err := c.src.SessionDays(ctx, calendarID, contracts.Day(from.Format(dayLayout)), contracts.Day(to.Format(dayLayout)))
		if err != nil {
			return clock.Session{}, &typedError{kind: contracts.Retryable, msg: "session source", err: err}
		}
		if w == 0 && len(rows) == 0 {
			return clock.Session{}, unknown(calendarID)
		}
		if len(rows) == 0 {
			break
		}
		var best *contracts.SessionDay
		for i := range rows {
			r := &rows[i]
			if r.IsSession && (best == nil || r.SessionDay.Before(best.SessionDay.Time)) {
				best = r
			}
		}
		if best != nil {
			return toSession(*best, contracts.Day(best.SessionDay.Format(dayLayout))), nil
		}
	}
	return clock.Session{}, &typedError{kind: contracts.NoValue, msg: fmt.Sprintf("no session of %s after %s", calendarID, day)}
}

func toSession(r contracts.SessionDay, day contracts.Day) clock.Session {
	return clock.Session{CalendarID: r.CalendarID, Day: day, IsSession: r.IsSession, OpenAt: r.OpenAt, CloseAt: r.CloseAt, EarlyClose: r.EarlyClose}
}

func unknown(calendarID string) error {
	return &typedError{kind: contracts.UnknownCalendar, msg: "unknown calendar " + calendarID}
}

// ET converts t to America/New_York.
func ET(t time.Time) time.Time { return mustIn(t, zoneET) }

// JST converts t to Asia/Tokyo.
func JST(t time.Time) time.Time { return mustIn(t, zoneJST) }

// InZone converts t to the IANA zone the caller names (the owner zone); the package hard-codes none.
// An unknown name is a Permanent contracts.TypedError.
func InZone(t time.Time, zone string) (time.Time, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, &typedError{kind: contracts.Permanent, msg: "zone " + zone, err: err}
	}
	return t.In(loc), nil
}

func mustIn(t time.Time, zone string) time.Time {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		panic(err) // the zone names above are constants and time/tzdata is linked in
	}
	return t.In(loc)
}
