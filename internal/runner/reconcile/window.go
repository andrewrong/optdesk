package reconcile

import (
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/store"
)

// DefaultCalendarID is the calendar the runner reads when a rule names none.
const DefaultCalendarID = "XNYS"

// WindowRule turns a session into the window of a dataset: the window opens
// OpenOffset after the session open and closes CloseOffset after the session
// close, both instants taken from the calendar. A non-session day has no window.
type WindowRule struct {
	// CalendarID names the calendar; empty means DefaultCalendarID.
	CalendarID  string
	OpenOffset  time.Duration
	CloseOffset time.Duration
}

func (r WindowRule) calendarID() string {
	if r.CalendarID == "" {
		return DefaultCalendarID
	}
	return r.CalendarID
}

// Window returns the window of datasetID on session s, and false when s is not a session.
func (r WindowRule) Window(datasetID string, s clock.Session) (store.Window, bool) {
	if !s.IsSession {
		return store.Window{}, false
	}
	return store.Window{
		DatasetID: datasetID,
		Day:       s.Day,
		OpenAt:    s.OpenAt.Add(r.OpenOffset),
		CloseAt:   s.CloseAt.Add(r.CloseOffset),
	}, true
}
