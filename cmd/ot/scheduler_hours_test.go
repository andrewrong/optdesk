package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
)

// allWithHours is allUSJPJobs plus the two calendar_hours jobs, which a rig with the notice-store marker line wires.
var allWithHours = slices.Sorted(slices.Values(append(slices.Clone(allUSJPJobs), jobCalendarHoursUS, jobCalendarHoursJP)))

// TestSchedulerReprobesHoursJobsAfterBrokerDown: the calendar_hours jobs disabled by a broker-down start are probed
// back in on the next tick, with the gap rows cleared.
func TestSchedulerReprobesHoursJobsAfterBrokerDown(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, strings.Replace(fullEnv, "master-dsn", hoursMarker, 1))
	r.a.deps.newCalendar = func(clock.SessionSource) clock.Calendar { return &jpCal{last: jpLastDay} }
	r.dialErrs = func(n int32) error {
		if n == 1 {
			return errors.New("no socket")
		}
		return nil
	}
	done := r.start()
	r.waitStarted(t, done)
	if d := r.sched.disabled; d[jobCalendarHoursUS] == "" || d[jobCalendarHoursJP] == "" {
		t.Fatalf("hours jobs not disabled after the broker-down start: %v", d)
	}
	r.tick()
	if d := r.sched.disabled; len(d) != 0 {
		t.Fatalf("still disabled after the broker came back: %v", d)
	}
	if last := r.gaps.seen[len(r.gaps.seen)-1]; len(last) != 0 {
		t.Fatalf("gaps %v", last)
	}
	_ = r.stop(t, done)
}
