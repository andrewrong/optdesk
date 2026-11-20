package main

import (
	"context"
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

	// busyJobRunAge is how young the heartbeat of an open ops.job_run row must be to count as alive. Progress
	// refreshes it per key and a key can take several attempts, so the 90 s capture limit is too short; a row
	// left open by a crashed process stops holding a deploy after this long.
	busyJobRunAge = 10 * time.Minute
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
	if err := busyJobRun(a, now); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(a.stdout, "idle")
	return nil
}

// busyJobRun reads the open ops.job_run rows once (Running) and returns an exitError with code 75 naming the
// first one whose heartbeat is younger than busyJobRunAge, code 1 when the store cannot be opened or read, and
// nil when none is alive. Without an opener it reads nothing. The store is released on every path; the error
// texts hold no DSN.
func busyJobRun(a *app, now time.Time) error {
	if a.deps.openJobRun == nil {
		return nil
	}
	env, err := a.envFile()
	if err != nil {
		return &exitError{code: 1, err: err}
	}
	ctx := context.Background()
	st, err := a.deps.openJobRun(ctx, env[store.DSNEnvRunner])
	if err != nil {
		return &exitError{code: 1, err: fmt.Errorf("open job run store with %s failed", store.DSNEnvRunner)}
	}
	defer func() { _ = st.Release() }()
	rows, err := st.Running(ctx)
	if err != nil {
		return &exitError{code: 1, err: errors.New("read the open job runs failed")}
	}
	for _, r := range rows {
		if !r.HeartbeatAt.IsZero() && now.Sub(r.HeartbeatAt) < busyJobRunAge {
			return &exitError{code: exitBusy, err: fmt.Errorf("busy: job run %s is open", r.ID)}
		}
	}
	return nil
}
