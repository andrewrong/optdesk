package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

const (
	// exitBusy is the exit code of ot scheduler busy while a run is alive: crew holds a deploy on it.
	exitBusy = 75

	// busyHeartbeatAge is how young the heartbeat of a running run must be to count as alive: three times
	// the 30 s beat of internal/runner/reconcile/runkey.go, so a killed process reads idle within 90 s.
	busyHeartbeatAge = 3 * 30 * time.Second
)

// schedBusyCmd is `ot scheduler busy`: nil and `idle` when no run is alive, an exitError with code 75 naming
// the first alive run otherwise, and code 1 when the ledger cannot be opened or read. A run is alive while
// its status is running and its heartbeat is younger than busyHeartbeatAge. It reads through readSchedStatus
// alone: no write, no advisory lock, no fence, no dataset store.
func schedBusyCmd(a *app, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ot scheduler busy")
	}
	st, now, err := readSchedStatus(a)
	if err != nil {
		return &exitError{code: 1, err: err}
	}
	for _, r := range st.Runs {
		if r.Status == store.RunRunning && !r.HeartbeatAt.IsZero() && now.Sub(r.HeartbeatAt) < busyHeartbeatAge {
			return &exitError{code: exitBusy, err: fmt.Errorf("busy: run %s is alive", r.ID)}
		}
	}
	_, _ = fmt.Fprintln(a.stdout, "idle")
	return nil
}
