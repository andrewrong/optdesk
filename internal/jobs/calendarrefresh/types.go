package calendarrefresh

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// CalendarObserver is called once per changed day of the calendar_refresh job, before the job writes; the
// wiring logs one line per call. It must not block and its return value is not used.
type CalendarObserver func(clock.SessionDayChange)

// NewCalendarRefreshFunc is the constructor of the calendar_refresh job (internal/jobs/calendarrefresh.New).
type NewCalendarRefreshFunc func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, CalendarObserver) (contracts.Job, error)
