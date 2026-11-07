package reconcile

import (
	"fmt"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
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

// ruleOf returns the window rule of the datasets job plans: the values of its Window method when it
// has one (an empty calendar id means that of def), else def, the rule the reconciler was built with.
func ruleOf(job contracts.Job, def runner.WindowRule) runner.WindowRule {
	w, ok := job.(runner.Windowed)
	if !ok {
		return def
	}
	cal, open, closeOff := w.Window()
	if cal == "" {
		cal = def.CalendarID
	}
	return runner.WindowRule{CalendarID: cal, OpenOffset: open, CloseOffset: closeOff}
}

// checkCalendars refuses a job whose calendar differs from the one of the day loop (def, with the
// default applied): the loop has one calendar, so a mismatch would give a silently wrong window.
func checkCalendars(jobs []contracts.Job, def runner.WindowRule) error {
	want := calendarIDOf(def)
	for _, j := range jobs {
		if got := calendarIDOf(ruleOf(j, def)); got != want {
			return fmt.Errorf("job %s: window calendar %q differs from the reconciler calendar %q", j.ID(), got, want)
		}
	}
	return nil
}
