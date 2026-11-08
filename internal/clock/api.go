// Package clock owns the injected clock and the session calendar.
package clock

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Clock tells the time; it is a subset of clockwork.Clock so that the fake satisfies it.
type Clock interface {
	Now() time.Time
}

// Session is one calendar day as the calendar answers it.
type Session struct {
	CalendarID string
	Day        contracts.Day
	IsSession  bool
	OpenAt     time.Time
	CloseAt    time.Time
	EarlyClose bool
}

// Calendar answers session questions by calendar id and day; an unknown calendar is a
// contracts.TypedError of kind UnknownCalendar.
type Calendar interface {
	Session(ctx context.Context, calendarID string, day contracts.Day) (Session, error)
	// NextSession returns the first session strictly after day.
	NextSession(ctx context.Context, calendarID string, day contracts.Day) (Session, error)
}

// SessionSource is what clock needs from storage: the session days of a calendar between two days, inclusive.
type SessionSource interface {
	SessionDays(ctx context.Context, calendarID string, from, to contracts.Day) ([]contracts.SessionDay, error)
}

// NewClockFunc is the constructor of the real clock (internal/clock/realclock.NewReal).
type NewClockFunc func() Clock

// NewCalendarFunc is the constructor of the calendar (internal/clock/calendar.NewCalendar).
type NewCalendarFunc func(SessionSource) Calendar

// SessionDayChange is one day on which two session-day slices differ. Before is nil for a day only
// in the fresh slice; After is nil for a day only in the old slice.
type SessionDayChange struct {
	Day    contracts.Day
	Before *contracts.SessionDay
	After  *contracts.SessionDay
}

// SessionDayDiffer compares two session-day slices by day. The result is sorted by day and empty
// when the slices agree.
type SessionDayDiffer interface {
	Diff(old, fresh []contracts.SessionDay) []SessionDayChange
}

// NewSessionDayDifferFunc is the constructor of the differ (internal/clock/calendar.NewDiffer).
type NewSessionDayDifferFunc func() SessionDayDiffer

// CommandFunc runs argv to its end and returns its exit code. err is non-nil only when the command
// could not start or be waited for.
type CommandFunc func(ctx context.Context, argv []string) (exitCode int, err error)

// NewCommandSessionSourceFunc is the constructor of the session source that runs the pyside
// generator as a command (internal/clock/calendarcli.New). argv is the command prefix and workDir
// the directory for its temporary file.
type NewCommandSessionSourceFunc func(argv []string, workDir string, run CommandFunc) SessionSource
