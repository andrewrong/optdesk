// Package sched turns the plain schedule of jobs into start instants.
package sched

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

const dayLayout = "2006-01-02"

// The anchors of a runner.Slot.
const (
	anchorOpen  = "session_open"
	anchorClose = "session_close"
	anchorTime  = "time"
)

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
// error naming the job. It reads no clock, store or file.
func NextStarts(ctx context.Context, cal clock.Calendar, jobs []runner.Slot, from time.Time, days int) ([]Start, error) {
	zones := map[string]*time.Location{}
	for _, j := range jobs {
		if j.Anchor == anchorTime {
			if _, ok := zones[j.Zone]; !ok {
				loc, err := time.LoadLocation(j.Zone)
				if err != nil || j.Zone == "" {
					return nil, fmt.Errorf("job %s: unknown zone %q", j.JobID, j.Zone)
				}
				zones[j.Zone] = loc
			}
		} else if j.Anchor != anchorOpen && j.Anchor != anchorClose {
			return nil, fmt.Errorf("job %s: unknown anchor %q", j.JobID, j.Anchor)
		}
	}
	f := from.UTC()
	first := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, time.UTC)
	var out []Start
	for d := 0; d < days; d++ {
		date := first.AddDate(0, 0, d)
		day := contracts.Day(date.Format(dayLayout))
		for _, j := range jobs {
			s, err := cal.Session(ctx, j.CalendarID, day)
			if err != nil {
				return nil, fmt.Errorf("job %s: %w", j.JobID, err)
			}
			if !s.IsSession {
				continue
			}
			var at time.Time
			switch j.Anchor {
			case anchorOpen:
				at = s.OpenAt.Add(j.Offset)
			case anchorClose:
				at = s.CloseAt.Add(j.Offset)
			default:
				at = wallTime(date, zones[j.Zone], j.Offset)
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

// wallTime is the instant at which the wall clock of loc shows offset since midnight of date.
func wallTime(date time.Time, loc *time.Location, offset time.Duration) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, int(offset), loc)
}
