package calendarrefresh

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the calendarrefresh job; each name is an alias of the one in package jobs.
type (
	CalendarObserver       = jobs.CalendarObserver
	NewCalendarRefreshFunc = jobs.NewCalendarRefreshFunc
)
