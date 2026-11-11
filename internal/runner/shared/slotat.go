// Package shared holds the logic that the sub-packages of the runner share.
package shared

import (
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/runner"
)

// The anchors of a runner.Slot.
const (
	AnchorOpen  = "session_open"
	AnchorClose = "session_close"
	AnchorTime  = "time"
)

const dayLayout = "2006-01-02"

// At is the instant of slot s on the day of sess, the session of s.CalendarID with IsSession true:
// the session's OpenAt or CloseAt plus Offset, or, for the time anchor, Offset since midnight of
// the day in s.Zone. An unknown anchor, or an unknown or empty zone for time, is an error naming
// the job.
func At(s runner.Slot, sess clock.Session) (time.Time, error) {
	switch s.Anchor {
	case AnchorOpen:
		return sess.OpenAt.Add(s.Offset), nil
	case AnchorClose:
		return sess.CloseAt.Add(s.Offset), nil
	case AnchorTime:
		loc, err := time.LoadLocation(s.Zone)
		if err != nil || s.Zone == "" {
			return time.Time{}, fmt.Errorf("job %s: unknown zone %q", s.JobID, s.Zone)
		}
		// The date is the session's day (sess.Day), taken in s.Zone: midnight of that date in the
		// zone, so across a DST change two days are 23 or 25 hours apart, not 24. sched/slots.go
		// walks UTC dates and passes the session of that date, whose Day is the same day, so both
		// callers get the same instant.
		d, err := time.Parse(dayLayout, string(sess.Day))
		if err != nil {
			return time.Time{}, fmt.Errorf("job %s: day %q: %w", s.JobID, sess.Day, err)
		}
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, int(s.Offset), loc), nil
	}
	return time.Time{}, fmt.Errorf("job %s: unknown anchor %q", s.JobID, s.Anchor)
}
