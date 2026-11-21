package optioneod

import (
	"errors"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// WithCalendar sets the calendar of a job built by this package and returns the same job: a job whose slot is the
// session close minus an offset reads the close of the day from it to tell an early run. A nil cal returns the job
// unchanged and any other job is an error. Call it once, before the first run.
func WithCalendar(jb contracts.Job, cal clock.Calendar) (contracts.Job, error) {
	j, ok := jb.(*job)
	if !ok {
		return nil, errors.New("optioneod: WithCalendar takes a job built by this package")
	}
	if cal != nil {
		j.calendar = cal
	}
	return jb, nil
}
