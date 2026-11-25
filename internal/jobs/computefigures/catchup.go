package computefigures

import (
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// catchupSessions is how many session days before the as-of day a market figure is recomputed for.
const catchupSessions = 5

// catchUpMarket recomputes the figure of t for each of the last catchupSessions session days before the as-of day.
// FRED publishes a day late, so the point an earlier day was computed without may be stored by now; store writes
// a new rev only when the input rows changed. The earlier days add nothing to the counts of the run, and a failure
// of one is the run's retryable error without stopping the as-of day or the other earlier days.
func (r *runState) catchUpMarket(t target) {
	days, err := r.j.stores.SessionDays.SessionDays(r.ctx, r.j.sched.calendar, shiftDay(r.day, -3*catchupSessions), shiftDay(r.day, -1))
	if err != nil {
		r.catchupFail(fmt.Errorf("computefigures: session days: %w", err))
		return
	}
	var sessions []contracts.Day
	for _, d := range days {
		if d.IsSession {
			sessions = append(sessions, dayOf(d.SessionDay))
		}
	}
	if len(sessions) > catchupSessions {
		sessions = sessions[len(sessions)-catchupSessions:]
	}
	for _, d := range sessions {
		if r.ctx.Err() != nil {
			r.catchupFail(r.ctx.Err())
			return
		}
		// A throwaway state: the failed count of an earlier day is dropped on purpose, and its error, whatever its
		// kind, becomes the retryable error of the run (the next run looks back again).
		sub := &runState{j: r.j, ctx: r.ctx, day: d}
		sub.marketDay(t)
		if sub.firstErr != nil {
			r.catchupFail(sub.firstErr)
		}
	}
}

// catchupFail makes err the retryable error of the run without counting a failed key.
func (r *runState) catchupFail(err error) {
	if r.firstErr == nil || r.firstErr.Kind() != contracts.Retryable {
		r.firstErr = &typedError{kind: contracts.Retryable, err: err}
	}
}
