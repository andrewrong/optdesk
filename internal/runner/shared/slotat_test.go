package shared_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/sched"
	"github.com/andrewrong/optdesk/internal/runner/shared"
)

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestAtSessionAnchors(t *testing.T) {
	t.Parallel()
	sess := clock.Session{Day: "2026-11-05", IsSession: true, OpenAt: utc("2026-11-05T14:30:00Z"), CloseAt: utc("2026-11-05T21:00:00Z")}
	for _, tc := range []struct {
		name string
		slot runner.Slot
		want time.Time
	}{
		{"open minus 45m", runner.Slot{JobID: "j", Anchor: shared.AnchorOpen, Offset: -45 * time.Minute}, utc("2026-11-05T13:45:00Z")},
		{"close plus 15m", runner.Slot{JobID: "j", Anchor: shared.AnchorClose, Offset: 15 * time.Minute}, utc("2026-11-05T21:15:00Z")},
	} {
		got, err := shared.At(tc.slot, sess)
		if err != nil || !got.Equal(tc.want) {
			t.Fatalf("%s: At = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
}

func TestAtTimeAcrossDST(t *testing.T) {
	t.Parallel()
	slot := runner.Slot{JobID: "j", Anchor: shared.AnchorTime, Zone: "America/New_York", Offset: 9*time.Hour + 30*time.Minute}
	// New York changes to standard time on 2026-11-01.
	before, err := shared.At(slot, clock.Session{Day: "2026-10-31", IsSession: true})
	if err != nil {
		t.Fatal(err)
	}
	after, err := shared.At(slot, clock.Session{Day: "2026-11-02", IsSession: true})
	if err != nil {
		t.Fatal(err)
	}
	if !before.Equal(utc("2026-10-31T13:30:00Z")) || !after.Equal(utc("2026-11-02T14:30:00Z")) {
		t.Fatalf("before = %v, after = %v", before, after)
	}
	if d := after.Sub(before); d != 49*time.Hour {
		t.Fatalf("distance = %v, want 49h (two days plus the hour the clocks went back)", d)
	}
	// One day apart across the change: 25 hours.
	mid, _ := shared.At(slot, clock.Session{Day: "2026-11-01", IsSession: true})
	if d := mid.Sub(before); d != 25*time.Hour {
		t.Fatalf("10-31 to 11-01 = %v, want 25h", d)
	}
}

func TestAtErrorsNameTheJob(t *testing.T) {
	t.Parallel()
	sess := clock.Session{Day: "2026-11-05", IsSession: true}
	for _, slot := range []runner.Slot{
		{JobID: "jx", Anchor: "bogus"},
		{JobID: "jx", Anchor: shared.AnchorTime, Zone: "Not/AZone"},
		{JobID: "jx", Anchor: shared.AnchorTime},
	} {
		_, err := shared.At(slot, sess)
		if err == nil || !strings.Contains(err.Error(), "job jx") {
			t.Fatalf("At(%+v) error = %v, want one naming job jx", slot, err)
		}
	}
}

type oneCal struct{ s []clock.Session }

func (c oneCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	for _, s := range c.s {
		if s.Day == day {
			s.CalendarID = id
			return s, nil
		}
	}
	return clock.Session{CalendarID: id, Day: day}, nil
}

func (c oneCal) NextSession(context.Context, string, contracts.Day) (clock.Session, error) {
	return clock.Session{}, nil
}

// TestSchedWakeInstantEqualsAt: the At of a Start of sched.NextStarts is shared.At of the same slot
// and the session of the day.
func TestSchedWakeInstantEqualsAt(t *testing.T) {
	t.Parallel()
	sess := clock.Session{Day: "2026-11-05", IsSession: true, OpenAt: utc("2026-11-05T14:30:00Z"), CloseAt: utc("2026-11-05T21:00:00Z")}
	for _, slot := range []runner.Slot{
		{JobID: "o", CalendarID: "XNYS", Anchor: shared.AnchorOpen, Offset: -45 * time.Minute},
		{JobID: "c", CalendarID: "XNYS", Anchor: shared.AnchorClose, Offset: 5 * time.Minute},
		{JobID: "t", CalendarID: "XNYS", Anchor: shared.AnchorTime, Zone: "America/New_York", Offset: 7 * time.Hour},
	} {
		starts, err := sched.NextStarts(context.Background(), oneCal{[]clock.Session{sess}}, []runner.Slot{slot}, utc("2026-11-05T00:00:00Z"), 1)
		if err != nil || len(starts) != 1 {
			t.Fatalf("%s: starts = %v, err = %v", slot.JobID, starts, err)
		}
		want, err := shared.At(slot, sess)
		if err != nil || !starts[0].At.Equal(want) {
			t.Fatalf("%s: NextStarts at %v, shared.At %v (%v)", slot.JobID, starts[0].At, want, err)
		}
	}
}
