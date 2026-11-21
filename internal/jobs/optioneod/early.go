package optioneod

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// reasonBeforeSlot is the word of the Retryable error of a run that started before the slot of its day.
const reasonBeforeSlot = "before_slot"

// beforeSlot tells whether now is before the slot instant of day. For a close-anchored slot ("close-<m>m") the instant
// is the calendar close of day plus the schedule offset; without a calendar, on a day that is not a session or with a
// zero close it is never early. Otherwise it is the day at the manifest slot HHMM (seconds 0) in the manifest zone;
// a slot that does not parse is never early.
func (j *job) beforeSlot(ctx context.Context, day contracts.Day, now time.Time) bool {
	if _, ok := closeSlotMinutes(j.m.Slot); ok {
		if j.calendar == nil {
			return false
		}
		s, err := j.calendar.Session(ctx, j.m.sched.calendar, day)
		if err != nil || !s.IsSession || s.CloseAt.IsZero() {
			return false
		}
		return now.Before(s.CloseAt.Add(j.m.sched.offset))
	}
	at, err := time.ParseInLocation("2006-01-02 1504", string(day)+" "+j.m.Slot, j.loc)
	if err != nil {
		return false
	}
	return now.Before(at)
}
