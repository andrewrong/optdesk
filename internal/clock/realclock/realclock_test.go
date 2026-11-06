package realclock

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
)

var _ clock.NewClockFunc = NewReal

func TestRealClockIsSystemTime(t *testing.T) {
	t.Parallel()
	c := NewReal()
	// The system clock cannot be compared to a fixed instant; it must be set and not run backwards.
	first := c.Now()
	if first.Year() < 2026 || c.Now().Before(first) {
		t.Fatalf("Now = %v", first)
	}
}
