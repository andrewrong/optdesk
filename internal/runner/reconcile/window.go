package reconcile

import (
	"context"
	"errors"
	"fmt"
	"sort"

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

// daySessions maps a calendar id to its session on one day; a calendar with no session is absent.
type daySessions map[string]clock.Session

// jobCalendar is the calendar id of the window of job (the rule's when the job names none).
func (r *reconciler) jobCalendar(job contracts.Job) string {
	return calendarIDOf(ruleOf(job, r.rule))
}

// calendars lists the distinct calendars of the jobs in order; with no job, the rule's calendar.
func (r *reconciler) calendars() []string {
	seen := map[string]bool{}
	var out []string
	for _, j := range r.jobs {
		if id := r.jobCalendar(j); !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		out = append(out, calendarIDOf(r.rule))
	}
	sort.Strings(out)
	return out
}

// probeKnown returns the calendars of the jobs, other than the rule's, that answer Session on the
// day of the clock now: the known set. A reconciler of one calendar makes no call.
func (r *reconciler) probeKnown(ctx context.Context) map[string]bool {
	known := map[string]bool{}
	if r.cal == nil {
		return known
	}
	today := contracts.Day(r.clk.Now().UTC().Format("2006-01-02"))
	for _, id := range r.calendars() {
		if id == calendarIDOf(r.rule) {
			continue
		}
		if _, err := r.cal.Session(ctx, id, today); err == nil {
			known[id] = true
		}
	}
	return known
}

func kindOf(err error) (contracts.ErrorKind, bool) {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te.Kind(), true
	}
	return "", false
}

// walk lists the days after from up to and including to that are a session of at least one calendar
// of the jobs, in order, with the sessions of each. A calendar of the rule that fails is an error; a
// known calendar that runs out (UnknownCalendar or NoValue) is logged and its list ends there, so
// its jobs are dropped for the days it cannot answer; any other calendar's error is returned naming
// its first job (NoValue alone just ends the list).
func (r *reconciler) walk(ctx context.Context, from, to contracts.Day) ([]contracts.Day, map[contracts.Day]daySessions, error) {
	r.knownOnce.Do(func() { r.known = r.probeKnown(ctx) })
	by := map[contracts.Day]daySessions{}
	for _, id := range r.calendars() {
		list, stop := r.sessions(ctx, id, from, to)
		for _, s := range list {
			if by[s.Day] == nil {
				by[s.Day] = daySessions{}
			}
			by[s.Day][id] = s
		}
		if stop == nil {
			continue
		}
		kind, typed := kindOf(stop)
		noValue := typed && kind == contracts.NoValue
		switch {
		case id == calendarIDOf(r.rule):
			if !noValue {
				return nil, nil, stop
			}
		case r.known[id] && typed && (noValue || kind == contracts.UnknownCalendar):
			r.log.Warn(fmt.Sprintf("calendar %s: %v, jobs dropped", id, stop))
		case noValue:
		default:
			return nil, nil, fmt.Errorf("job %s: calendar %s: %w", r.firstJobOf(id), id, stop)
		}
	}
	days := make([]contracts.Day, 0, len(by))
	for d := range by {
		days = append(days, d)
	}
	sort.Slice(days, func(a, b int) bool { return days[a] < days[b] })
	return days, by, nil
}

func (r *reconciler) firstJobOf(calendarID string) string {
	for _, j := range r.jobs {
		if r.jobCalendar(j) == calendarID {
			return j.ID()
		}
	}
	return ""
}

// sessions lists the sessions of calendar id after from up to and including to. With no cursor
// (empty from) only day to itself is considered. The returned error is the one that ended the list
// early, NoValue included; the sessions read before it are returned.
func (r *reconciler) sessions(ctx context.Context, id string, from, to contracts.Day) ([]clock.Session, error) {
	if from == "" {
		s, err := r.cal.Session(ctx, id, to)
		if err != nil || !s.IsSession {
			return nil, err
		}
		return []clock.Session{s}, nil
	}
	var out []clock.Session
	for from < to {
		s, err := r.cal.NextSession(ctx, id, from)
		if err != nil {
			return out, err
		}
		if s.Day > to {
			break
		}
		out = append(out, s)
		from = s.Day
	}
	return out, nil
}
