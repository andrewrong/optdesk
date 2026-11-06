package reconcile

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// DefaultCalendarID is the calendar the runner reads when a rule names none.
const DefaultCalendarID = "XNYS"

// calendarIDOf returns the calendar the rule names, or DefaultCalendarID when it names none.
func calendarIDOf(rule runner.WindowRule) string {
	if rule.CalendarID == "" {
		return DefaultCalendarID
	}
	return rule.CalendarID
}

// windowFor returns the window of datasetID on session s under rule, and false when s is not a session.
func windowFor(rule runner.WindowRule, datasetID string, s clock.Session) (store.Window, bool) {
	if !s.IsSession {
		return store.Window{}, false
	}
	return store.Window{
		DatasetID: datasetID,
		Day:       s.Day,
		OpenAt:    s.OpenAt.Add(rule.OpenOffset),
		CloseAt:   s.CloseAt.Add(rule.CloseOffset),
	}, true
}
