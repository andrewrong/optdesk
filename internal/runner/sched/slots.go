// Package sched turns the plain schedule of jobs into start instants.
package sched

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/shared"
)

const dayLayout = "2006-01-02"

// Start is one start instant of a job, in UTC.
type Start struct {
	JobID string
	At    time.Time
}

// NextStarts returns the start instants of jobs on the days consecutive calendar days beginning with
// the UTC date of from, in UTC, sorted by instant (ties by order of jobs), dropping instants before
// from. A day that is not a session of the slot's calendar gives no start. session_open and
// session_close are read from cal (so an early close moves the slot) plus Offset; time is Offset of
// wall time since midnight of the day in the named Zone. An unknown calendar, zone or anchor is an
// error naming the job. It reads no clock, store or file, and drops no slot: every calendar error is
// returned (nextStarts drops the slots of a known calendar that runs out).
func NextStarts(ctx context.Context, cal clock.Calendar, jobs []runner.Slot, from time.Time, days int) ([]Start, error) {
	return nextStarts(ctx, cal, jobs, from, days, nil, slog.Default())
}

// nextStarts is NextStarts with the known calendars: the calendars that answered when the ticker was
// built. A slot whose calendar is known, is not defaultCalendar (the rule's) and fails with
// UnknownCalendar or NoValue is dropped from that day on, with one log line per call and calendar;
// the slots of the other jobs are returned. Any other error is returned naming the job.
func nextStarts(
	ctx context.Context, cal clock.Calendar, jobs []runner.Slot, from time.Time, days int, known map[string]bool, log *slog.Logger,
) ([]Start, error) {
	zones := map[string]*time.Location{}
	for _, j := range jobs {
		if j.Anchor == shared.AnchorTime {
			if _, ok := zones[j.Zone]; !ok {
				loc, err := time.LoadLocation(j.Zone)
				if err != nil || j.Zone == "" {
					return nil, fmt.Errorf("job %s: unknown zone %q", j.JobID, j.Zone)
				}
				zones[j.Zone] = loc
			}
		} else if j.Anchor != shared.AnchorOpen && j.Anchor != shared.AnchorClose {
			return nil, fmt.Errorf("job %s: unknown anchor %q", j.JobID, j.Anchor)
		}
	}
	f := from.UTC()
	first := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, time.UTC)
	var out []Start
	dropped := map[string]bool{}
	for d := 0; d < days; d++ {
		date := first.AddDate(0, 0, d)
		day := contracts.Day(date.Format(dayLayout))
		for _, j := range jobs {
			if dropped[j.CalendarID] {
				continue
			}
			s, err := cal.Session(ctx, j.CalendarID, day)
			if err != nil {
				if j.CalendarID != defaultCalendar && known[j.CalendarID] && runsOut(err) {
					dropped[j.CalendarID] = true
					log.Warn(fmt.Sprintf("calendar %s: %v, jobs dropped", j.CalendarID, err))
					continue
				}
				return nil, fmt.Errorf("job %s: %w", j.JobID, err)
			}
			if !s.IsSession {
				continue
			}
			at, err := shared.At(j, s)
			if err != nil {
				return nil, err
			}
			if at.Before(from) {
				continue
			}
			out = append(out, Start{JobID: j.JobID, At: at.UTC()})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].At.Before(out[b].At) })
	return out, nil
}

// runsOut reports whether err says the calendar has no answer for the day: UnknownCalendar or NoValue.
func runsOut(err error) bool {
	var te contracts.TypedError
	return errors.As(err, &te) && (te.Kind() == contracts.UnknownCalendar || te.Kind() == contracts.NoValue)
}
