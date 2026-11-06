// Package realclock is the adapter of clock.Clock that reads the system time.
package realclock

import (
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
)

type real struct{}

// Now returns the system time; this file is the one place the package reads it.
func (real) Now() time.Time { return time.Now() }

// NewReal returns the clock of the running process; tests use a clockwork fake instead.
func NewReal() clock.Clock { return real{} }
