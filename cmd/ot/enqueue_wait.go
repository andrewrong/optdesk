package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/jonboulle/clockwork"
)

// Design: docs/design/single-runner.md Decisions 1 and 2. run-once and backfill run no job: they post console
// requests through jobrequest.Poster and may wait for the outcome, which the scheduler's console produces.

const (
	// defaultWaitTimeout is how long --wait waits, on the injected clock, before it gives up.
	defaultWaitTimeout = 120 * time.Second
	// waitPoll is the gap between two reads of a request.
	waitPoll = time.Second
)

// enqueueFlags are the flags run-once and backfill share.
type enqueueFlags struct {
	wait    bool
	timeout time.Duration
}

func (f *enqueueFlags) register(fs *flag.FlagSet) {
	fs.BoolVar(&f.wait, "wait", false, "wait for the console to finish the requests and print their key lines")
	fs.DurationVar(&f.timeout, "timeout", defaultWaitTimeout, "with --wait: give up after this long")
}

// buildRequests is the requests of a command: one per name of jobs when it has any (the page's own refusal is
// the answer), else one per job of the console list that the console can run (any_day and today_only; never
// listed_only). skipNotToday drops a today_only job when day is not today in its zone (run-once; backfill keeps
// it, the console host runs it for any day). force goes to the jobs that offer it when none is named.
func buildRequests(jobs []string, day contracts.Day, now time.Time, force, skipNotToday bool) []jobrequest.Request {
	var out []jobrequest.Request
	for _, id := range jobs {
		out = append(out, jobrequest.Request{JobID: id, Day: string(day), Force: force})
	}
	if len(jobs) > 0 {
		return out
	}
	for _, row := range jobTable {
		if !row.runnable() || (skipNotToday && row.Kind == kindTodayOnly && !isToday(row, day, now)) {
			continue
		}
		out = append(out, jobrequest.Request{JobID: row.ID, Day: string(day), Force: force && row.Force})
	}
	return out
}

// isToday tells whether day is the date of now in the zone of row (the market zone when it names none).
func isToday(row jobRow, day contracts.Day, now time.Time) bool {
	zone := row.Zone
	if zone == "" {
		zone = marketZone
	}
	loc, err := time.LoadLocation(zone)
	return err == nil && string(day) == now.In(loc).Format(dayLayout)
}

// enqueueNames is what Enqueue answered, by outcome.
var enqueueNames = map[jobrequest.EnqueueOutcome]string{
	jobrequest.EnqueueCreated: "created", jobrequest.EnqueueExisting: "existing",
	jobrequest.EnqueueWithdrawing: "withdrawing", jobrequest.EnqueueBusy: "busy",
}

// enqueue posts reqs for command cmd, prints the id and the outcome of each and, with --wait, waits for each
// request and prints its key lines. The error is non-zero for a request that is being withdrawn, was busy, or
// ended failed, refused or interrupted, and for a request still open at the timeout.
func enqueue(ctx context.Context, a *app, cmd string, reqs []jobrequest.Request, f enqueueFlags) error {
	o, ok := jobRequestOpenersOf(a.deps)
	_, dsn, reason, err := a.ownerEditSecrets()
	switch {
	case err != nil:
		return fmt.Errorf("%s: cannot read the env file", cmd)
	case reason != "":
		return fmt.Errorf("%s: %s", cmd, reason)
	case !ok || o.Poster == nil:
		return fmt.Errorf("%s: no request store in this build", cmd)
	}
	st, err := o.Poster(ctx, dsn)
	if err != nil {
		return fmt.Errorf("%s: cannot open the request store", cmd)
	}
	defer func() { _ = st.Close() }()
	var open []jobrequest.Row
	var failed []string
	for _, r := range reqs {
		res, err := st.Enqueue(ctx, r)
		if err != nil {
			return fmt.Errorf("%s: posting %s failed", cmd, r.JobID)
		}
		_, _ = fmt.Fprintf(a.stdout, "request %d job=%s day=%s outcome=%s\n", res.Row.ID, r.JobID, r.Day, enqueueNames[res.Outcome])
		switch res.Outcome { //nolint:exhaustive // created and existing are open; every other outcome stored nothing
		case jobrequest.EnqueueCreated, jobrequest.EnqueueExisting:
			open = append(open, res.Row)
		default:
			failed = append(failed, r.JobID)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%s: %d request(s) not stored (withdrawing or busy), retry: %v", cmd, len(failed), failed)
	}
	if !f.wait {
		return nil
	}
	return waitAll(ctx, a.deps.newFwClock(), a.stdout, st, open, f.timeout)
}

// waitAll polls each request on clk until it is neither queued nor running or until timeout has passed since the
// first poll, and prints its end. A request still open at the timeout names the console as the likely cause.
func waitAll(ctx context.Context, clk clockwork.Clock, w io.Writer, rd jobrequest.Reader, rows []jobrequest.Row, timeout time.Duration) error {
	deadline := clk.Now().Add(timeout)
	var bad []int64
	for _, r := range rows {
		row, err := waitOne(ctx, clk, rd, r.ID, deadline)
		if err != nil {
			return err
		}
		switch row.State { //nolint:exhaustive // only the two open states stay as the timeout leaves them
		case jobrequest.StateQueued:
			_, _ = fmt.Fprintf(w, "request %d is still queued: the console may be off (no consumer is running)\n", row.ID)
			bad = append(bad, row.ID)
			continue
		case jobrequest.StateRunning:
			_, _ = fmt.Fprintf(w, "request %d is still running: the timeout passed\n", row.ID)
			bad = append(bad, row.ID)
			continue
		}
		for _, l := range row.Lines {
			_, _ = fmt.Fprintf(w, "key dataset=%s subject=%s group=%s status=%s", l.Dataset, l.Subject, l.Group, l.Status)
			for _, kv := range [][2]string{{"error_code", l.ErrorCode}, {"reason", l.ReasonCode}, {"mark", l.Mark}} {
				if kv[1] != "" {
					_, _ = fmt.Fprintf(w, " %s=%s", kv[0], kv[1])
				}
			}
			_, _ = fmt.Fprintf(w, " returned=%d usable=%d attempt=%d\n", l.Returned, l.Usable, l.Attempt)
		}
		_, _ = fmt.Fprintf(w, "request %d job=%s day=%s state=%s reason=%s\n", row.ID, row.Request.JobID, row.Request.Day, row.State, row.Reason)
		switch row.State { //nolint:exhaustive // only these three end states are a bad exit
		case jobrequest.StateFailed, jobrequest.StateRefused, jobrequest.StateInterrupted:
			bad = append(bad, row.ID)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%d request(s) did not finish done: %v", len(bad), bad)
	}
	return nil
}

// waitOne reads request id every waitPoll of clk until it is closed or the clock reaches deadline; it returns
// the last row read.
func waitOne(ctx context.Context, clk clockwork.Clock, rd jobrequest.Reader, id int64, deadline time.Time) (jobrequest.Row, error) {
	for {
		row, found, err := rd.Get(ctx, id)
		switch {
		case err != nil:
			return row, fmt.Errorf("read request %d failed", id)
		case !found:
			return row, fmt.Errorf("request %d is gone", id)
		case row.State != jobrequest.StateQueued && row.State != jobrequest.StateRunning:
			return row, nil
		}
		left := deadline.Sub(clk.Now())
		if left <= 0 {
			return row, nil
		}
		t := clk.NewTimer(min(waitPoll, left))
		select {
		case <-ctx.Done():
			t.Stop()
			return row, errors.New("stopped while waiting")
		case <-t.Chan():
		}
	}
}
