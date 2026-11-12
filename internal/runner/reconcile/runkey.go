package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	// beat is the period of the heartbeat of a run and of the renewal of the run lease.
	beat = 30 * time.Second
	// reasonAbandonedStop is the reason of a run cut with no rows returned (Decision 10).
	reasonAbandonedStop = "abandoned_stop"
	// reasonConsoleStop is the reason of a key cut by a console stop (a deploy cancel or the key's own
	// console deadline): no crash strike, no attempt, no gap, which no reader counts.
	reasonConsoleStop = "console_stop"
	// defaultRunBound is the run bound of a job that has no Bounded method.
	defaultRunBound = 30 * time.Minute
)

// consoleKey marks a key context made by BackfillWith; RunKey only reads it.
type consoleKey struct{}

var (
	// errConsoleStop is the cause of the key context cancelled by the turn's own context (a deploy).
	errConsoleStop = errors.New("console stop")
	// errConsoleBound is the cause of a marked key context cut at the key's own deadline.
	errConsoleBound = errors.New("console key bound")
)

// consoleCut reports whether cause is one of the two console stops.
func consoleCut(cause error) bool {
	return errors.Is(cause, errConsoleStop) || errors.Is(cause, errConsoleBound)
}

type keyRunner struct {
	ops store.OpsLedger
	clk clockwork.Clock
	log *slog.Logger
}

// NewKeyRunner builds the one path that runs a job for one key. It takes no lease of its
// own in stage 1 (the caller holds ops.run_lease), so the SchedulerLedger of the
// constructor type is not used.
func NewKeyRunner(ops store.OpsLedger, _ store.SchedulerLedger, clk clockwork.Clock) runner.KeyRunner {
	return &keyRunner{ops: ops, clk: clk, log: slog.Default()}
}

// RunKey runs key through job under a context that ends at deadline, and records the run
// row with a heartbeat every 30 s of the clock. A run cut with rows is partial, a run cut
// without rows is abandoned (abandoned_stop); neither marks the key present. A job with a
// Postcondition marks the key present only when it answers true with a nil error.
func (k *keyRunner) RunKey(
	ctx context.Context, job contracts.Job, day contracts.Day, key contracts.ExpectedKey, attempt int, closeAt, deadline time.Time, reason string,
) (store.Run, error) {
	// The row is written after a cut (a drain cancels ctx), so the writes outlive ctx.
	wctx := context.WithoutCancel(ctx)
	now := k.clk.Now()
	rec := store.Run{
		ID:          fmt.Sprintf("%s|%s|%s|%d", day, key.DatasetID, key.SubjectKey, attempt),
		Day:         day,
		Key:         key,
		Status:      store.RunRunning,
		LeaseUntil:  now.Add(leaseTTL),
		HeartbeatAt: now,
		ReasonCode:  reason,
	}
	if err := k.ops.RecordRun(wctx, rec); err != nil {
		return rec, err
	}

	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	// A deadline that is not after the start (a run begun after the window close and its
	// bound) cuts nothing: that case is as unbounded as it was before the key runner, and
	// the follow-up of the design settles its bound.
	if deadline.After(now) {
		// A console key is cut for the console's own bound; any other cut is a plain cancel (a strike).
		var cause error
		if ctx.Value(consoleKey{}) != nil {
			cause = errConsoleBound
		}
		cut := k.clk.AfterFunc(deadline.Sub(now), func() { cancel(cause) })
		defer cut.Stop()
	}
	stop := k.heartbeat(wctx, runCtx, rec)

	res := job.Run(runCtx, day, []contracts.ExpectedKey{key})
	cutNow := runCtx.Err() != nil
	console := cutNow && consoleCut(context.Cause(runCtx))
	stop()

	rec.FinishedAt = k.clk.Now()
	rec.ReturnedN, rec.UsableN = res.Returned, res.Usable
	var status store.PresentStatus
	switch {
	case cutNow && res.Returned > 0:
		rec.Status = store.RunPartial
		if console {
			rec.ReasonCode = reasonConsoleStop
		}
	case cutNow:
		rec.Status = store.RunAbandoned
		rec.ReasonCode = reasonAbandonedStop
		if console {
			rec.ReasonCode = reasonConsoleStop
		}
	default:
		rec.Status = store.RunDone
		var err error
		if status, err = k.verdict(ctx, job, day, key, closeAt, res, &rec); err != nil {
			return rec, err
		}
	}
	if err := k.ops.RecordRun(wctx, rec); err != nil {
		return rec, err
	}
	k.logFinished(wctx, job, key, attempt, now, rec, res.Err)
	if status == "" {
		return rec, nil
	}
	return rec, k.ops.MarkPresent(wctx, day, key, status)
}

var (
	reDSN          = regexp.MustCompile(`(?i)\bpostgres(?:ql)?://\S*`)
	reUserinfoURL  = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s/@]*@\S*`)
	reSecretPair   = regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|secret|apikey|api_key|key)=[^\s&;]*`)
	reAuthzHeader  = regexp.MustCompile(`(?i)(authorization:\s*)(?:bearer|basic)\s+[^\s;]+`)
	reControlChars = regexp.MustCompile(`[\p{Cc}]+`)
)

const (
	redacted   = "[redacted]"
	maxErrText = 300
)

// safeErrText makes the text of a job error fit for the log: credentials are replaced by
// [redacted], control characters become one space, and the result is at most 300 bytes.
func safeErrText(s string) string {
	s = reDSN.ReplaceAllString(s, redacted)                // rule 1: a postgres DSN
	s = reUserinfoURL.ReplaceAllString(s, redacted)        // rule 1: a URL with userinfo
	s = reSecretPair.ReplaceAllString(s, "${1}="+redacted) // rule 2: key=value secrets
	s = reAuthzHeader.ReplaceAllString(s, "${1}"+redacted) // rule 3: Authorization credential
	s = reControlChars.ReplaceAllString(s, " ")            // rule 4: control characters
	if len(s) > maxErrText {                               // rule 5: cut on a rune boundary
		n := maxErrText
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		s = s[:n]
	}
	return s
}

// logFinished writes the one Info line of a run whose final row is recorded. It carries
// counts, codes and the duration; the message of a failed attempt's job error is logged
// after redaction (safeErrText), and no row content is.
func (k *keyRunner) logFinished(ctx context.Context, job contracts.Job, key contracts.ExpectedKey, attempt int, start time.Time, rec store.Run, jobErr error) {
	outcome := "degraded"
	if rec.Status == store.RunDone {
		outcome = "ok"
		if rec.ErrorCode != "" {
			outcome = "failed"
		}
	}
	attrs := []slog.Attr{
		slog.String("job", job.ID()),
		slog.String("day", string(rec.Day)),
		slog.String("dataset", key.DatasetID),
		slog.String("subject", key.SubjectKey),
		slog.Int("attempt", attempt),
		slog.String("outcome", outcome),
		slog.Int("returned", rec.ReturnedN),
		slog.Int("usable", rec.UsableN),
		slog.Duration("duration", rec.FinishedAt.Sub(start)),
	}
	if rec.ErrorCode != "" {
		attrs = append(attrs, slog.String("error_code", rec.ErrorCode))
		if jobErr != nil {
			attrs = append(attrs, slog.String("error", safeErrText(jobErr.Error())))
		}
	}
	if rec.ReasonCode != "" {
		attrs = append(attrs, slog.String("reason", rec.ReasonCode))
	}
	k.log.LogAttrs(ctx, slog.LevelInfo, "job run finished", attrs...)
}

// verdict decides the mark of a run that was not cut and sets ErrorCode or ReasonCode on rec.
func (k *keyRunner) verdict(
	ctx context.Context, job contracts.Job, day contracts.Day, key contracts.ExpectedKey, closeAt time.Time, res contracts.JobResult, rec *store.Run,
) (store.PresentStatus, error) {
	switch {
	case res.Err != nil && res.Err.Kind() == contracts.NoValue:
		return store.StatusNoValue, nil
	case res.Err != nil:
		rec.ErrorCode = string(res.Err.Kind())
		return "", nil
	}
	if pc, ok := job.(runner.Postcondition); ok {
		// A false answer or an error is a done run without a mark: the key stays missing.
		if exists, err := pc.Postcondition(ctx, string(day), key.DatasetID, key.SubjectKey); err == nil && exists {
			return store.StatusPresent, nil
		}
		return "", nil
	}
	switch {
	case res.Usable > 0:
		return store.StatusPresent, nil
	case !closeAt.IsZero() && k.clk.Now().Before(closeAt):
		// An empty answer is no explanation: retry until the window closes.
		rec.ReasonCode = reasonEmptyBeforeClose
		return "", nil
	}
	return store.StatusNoValue, nil
}

// heartbeat renews the run row every 30 s until the returned stop is called, which waits
// for the last write, so the final row is never overwritten by a beat.
func (k *keyRunner) heartbeat(wctx, runCtx context.Context, rec store.Run) (stop func()) {
	return every(runCtx, k.clk, func() bool {
		now := k.clk.Now()
		rec.HeartbeatAt, rec.LeaseUntil = now, now.Add(leaseTTL)
		// A failed beat is not fatal: the next one, or the final write, follows.
		_ = k.ops.RecordRun(wctx, rec)
		return true
	})
}

// every calls fn on each 30 s tick of clk until ctx ends, fn returns false, or stop is
// called; stop waits for a running fn to return.
func every(ctx context.Context, clk clockwork.Clock, fn func() bool) (stop func()) {
	t := clk.NewTicker(beat)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.Chan():
				if !fn() {
					return
				}
			}
		}
	}()
	return func() { close(done); wg.Wait() }
}

// runBound is the longest time one run of job may take.
func runBound(job contracts.Job) time.Duration {
	if b, ok := job.(runner.Bounded); ok {
		if d := b.RunBound(); d > 0 {
			return d
		}
	}
	return defaultRunBound
}

// holdBound is how far ahead the console looks for a slot start before it runs a key of job: the run
// bound of the job plus the timeouts of what follows the request (holdMargin).
func holdBound(job contracts.Job) time.Duration { return runBound(job) + holdMargin }

// deadlineFor is the end of a run: max(CloseAt, start) + the run bound of the job (Decision 8
// of docs/design/scheduler.md), where start is now, the instant the run begins. A run that
// begins before the close is cut a bound after the close; a late run or a Backfill of a closed
// day is cut a bound after its own start. A dataset with no window has no CloseAt, so its
// deadline is now plus the bound.
func deadlineFor(job contracts.Job, closeAt, now time.Time) time.Time {
	from := now // max(closeAt, now): a zero closeAt is never after now
	if closeAt.After(now) {
		from = closeAt
	}
	return from.Add(runBound(job))
}

// withClockwork returns clk as a clockwork.Clock: itself when it is one, else a clock whose
// Now is clk and whose timers are the real ones.
func withClockwork(clk clock.Clock) clockwork.Clock {
	if c, ok := clk.(clockwork.Clock); ok {
		return c
	}
	return nowClock{Clock: clockwork.NewRealClock(), now: clk}
}

type nowClock struct {
	clockwork.Clock
	now clock.Clock
}

func (c nowClock) Now() time.Time { return c.now.Now() }
