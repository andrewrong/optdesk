package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
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

const (
	// reasonUpstreamMissing marks a run of a dependent whose upstream was not ok for the day when it started
	// (Decision 6 of docs/design/console-run-lock.md). A run that ended ok may carry it.
	reasonUpstreamMissing = "upstream_missing"
	// reasonJobRunning is the ReasonCode of the lease_held line of a job that another run holds.
	reasonJobRunning = "job_running"
	// maxSessionLookback is how many days back the session of an upstream in another calendar is looked for.
	maxSessionLookback = 10
)

// errJobBusy is returned by runMissingKey when the job's try-lock is held.
var errJobBusy = errors.New("job running")

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

// dayCtx is the state of one day of one call that starts dependents: the plan and sessions of the day,
// the jobs due in the call that have not ended (Reconcile only), and the dependents started by a trigger.
type dayCtx struct {
	day  contracts.Day
	p    plan
	sess daySessions
	// console is set for a call of BackfillWith: a triggered run is bounded by the job's own run bound.
	console bool

	wg sync.WaitGroup // the goroutines of the dependents started by a trigger

	mu      sync.Mutex
	pending map[string]bool // jobs due in this call whose turn has not ended
	busy    map[string]bool // jobs whose turn found them locked by another run
	recheck map[string]recheck
	err     error
}

// recheck is a trigger of a dependent that found the dependent's lock held: the end of upstream up (ok
// tells whether it was ok). The turn that holds the lock honours it when it unlocks.
type recheck struct {
	up string
	ok bool
}

func newDayCtx(day contracts.Day, p plan, sess daySessions, console bool) *dayCtx {
	return &dayCtx{day: day, p: p, sess: sess, console: console, pending: map[string]bool{}, busy: map[string]bool{}, recheck: map[string]recheck{}}
}

func (dc *dayCtx) setPending(id string) {
	dc.mu.Lock()
	dc.pending[id] = true
	dc.mu.Unlock()
}

// finish records that the turn of job id ended; busy tells that it did not run because the job was locked.
func (dc *dayCtx) finish(id string, busy bool) {
	dc.mu.Lock()
	delete(dc.pending, id)
	if busy {
		dc.busy[id] = true
	}
	dc.mu.Unlock()
}

// want records a trigger of job id before it asks for the lock, so a turn that unlocks later sees it.
func (dc *dayCtx) want(id string, c recheck) {
	dc.mu.Lock()
	dc.recheck[id] = c
	dc.mu.Unlock()
}

// take removes and returns the trigger recorded for job id.
func (dc *dayCtx) take(id string) (recheck, bool) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	c, ok := dc.recheck[id]
	delete(dc.recheck, id)
	return c, ok
}

func (dc *dayCtx) running(id string) bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.pending[id] || dc.busy[id]
}

func (dc *dayCtx) fail(err error) {
	dc.mu.Lock()
	if dc.err == nil {
		dc.err = err
	}
	dc.mu.Unlock()
}

func (dc *dayCtx) firstErr() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.err
}

// keysOf are the keys of the plan of the day that job id plans.
func (dc *dayCtx) keysOf(id string) []contracts.ExpectedKey {
	var out []contracts.ExpectedKey
	for _, k := range dc.p.keys {
		if j := dc.p.job[k]; j != nil && j.ID() == id {
			out = append(out, k)
		}
	}
	return out
}

// jobEnd is how the runs of one job in one call ended.
type jobEnd struct{ ran, cut, failed bool }

func (e *jobEnd) add(rec store.Run) {
	e.ran = true
	switch {
	case rec.Status != store.RunDone:
		e.cut = true
	case rec.ErrorCode != "":
		e.failed = true
	}
}

// merge folds the end of another run of the same job into e.
func (e *jobEnd) merge(o jobEnd) {
	e.ran = e.ran || o.ran
	e.cut = e.cut || o.cut
	e.failed = e.failed || o.failed
}

// ended reports whether the job's runs all came to an end, ok or failed (a cut run is no end).
func (e jobEnd) ended() bool { return e.ran && !e.cut }

type upState int

const (
	upOK      upState = iota // every key of the upstream is marked for the day
	upMissing                // failed, ended without ok, or not run
	upRunning                // a run of the upstream is going on
)

func (r *reconciler) jobByID(id string) contracts.Job {
	for _, j := range r.jobs {
		if j.ID() == id {
			return j
		}
	}
	return nil
}

// upstreamDay is the day of job in its own calendar that a dependent of day reads: day itself when the
// calendar has a session in it, else the latest session day before it. "" when there is none.
func (r *reconciler) upstreamDay(ctx context.Context, dc *dayCtx, job contracts.Job) contracts.Day {
	cal := r.jobCalendar(job)
	if _, ok := dc.sess[cal]; ok {
		return dc.day
	}
	t, err := time.Parse("2006-01-02", string(dc.day))
	if err != nil || r.cal == nil {
		return ""
	}
	for i := 1; i <= maxSessionLookback; i++ {
		d := contracts.Day(t.AddDate(0, 0, -i).Format("2006-01-02"))
		if s, err := r.cal.Session(ctx, cal, d); err == nil && s.IsSession {
			return d
		}
	}
	return ""
}

// upstreamState reads the state of upstream id for the day of dc. ends holds the upstreams whose run just
// ended in the triggering call, with their outcome, which wins over the ledger.
func (r *reconciler) upstreamState(ctx context.Context, dc *dayCtx, id string, ends map[string]bool) (upState, error) {
	if ok, has := ends[id]; has {
		if ok {
			return upOK, nil
		}
		return upMissing, nil
	}
	if dc.running(id) {
		return upRunning, nil
	}
	job := r.jobByID(id)
	if job == nil {
		return upOK, nil // an id no job answers to is not an order
	}
	d := r.upstreamDay(ctx, dc, job)
	if d == "" {
		return upMissing, nil
	}
	var keys []contracts.ExpectedKey
	if d == dc.day {
		keys = dc.keysOf(id)
	} else if ks, _, err := job.Plan(ctx, d); err == nil {
		keys = ks
	}
	if len(keys) == 0 {
		return upMissing, nil
	}
	present, byKey, err := r.dayState(ctx, d)
	if err != nil {
		return upMissing, err
	}
	now := r.clk.Now()
	all, live := true, false
	for _, k := range keys {
		if _, ok := present[k]; !ok {
			all = false
		}
		for _, run := range byKey[k] {
			if run.Status == store.RunRunning && !run.LeaseUntil.Before(now) {
				live = true
			}
		}
	}
	switch {
	case all:
		return upOK, nil
	case live:
		return upRunning, nil
	}
	return upMissing, nil
}

// gate decides how job starts on the day of dc: leave is true while an upstream is running or due in the
// call and not ended (no wait: the end of that upstream starts the job); mark is reasonUpstreamMissing when
// an upstream is not ok. A job with no upstream has neither.
func (r *reconciler) gate(ctx context.Context, dc *dayCtx, job contracts.Job, ends map[string]bool) (mark string, leave bool, err error) {
	dep, ok := job.(runner.Dependent)
	if !ok {
		return "", false, nil
	}
	for _, id := range dep.After() {
		st, err := r.upstreamState(ctx, dc, id, ends)
		if err != nil {
			return "", false, err
		}
		switch st {
		case upOK:
		case upRunning:
			leave = true
		case upMissing:
			mark = reasonUpstreamMissing
		}
	}
	return mark, leave, nil
}

// consoleMark is the mark of a console run of job: it is never held by the order and never starts an
// upstream, so an upstream that is not ok (running included) only marks the run.
func (r *reconciler) consoleMark(ctx context.Context, dc *dayCtx, job contracts.Job) (string, error) {
	mark, leave, err := r.gate(ctx, dc, job, nil)
	if leave {
		mark = reasonUpstreamMissing
	}
	return mark, err
}

// trigger starts, in goroutines of dc.wg under ctx, each dependent of the job up that has a session on
// the day, whose slot has passed and that has not run for the day. The caller waits for dc.wg.
func (r *reconciler) trigger(ctx context.Context, dc *dayCtx, up string, upEndedOK bool) {
	for _, d := range r.jobs {
		dep, ok := d.(runner.Dependent)
		if !ok || ctx.Err() != nil || !slices.Contains(dep.After(), up) {
			continue
		}
		if _, has := dc.sess[r.jobCalendar(d)]; !has || len(dc.keysOf(d.ID())) == 0 {
			continue
		}
		if at, sched, err := slotAt(d, dc.sess, r.rule); err != nil || (sched && r.clk.Now().Before(at)) {
			continue // not yet its slot: left to its slot turn
		}
		dc.wg.Add(1)
		go func() {
			defer dc.wg.Done()
			r.runDependent(ctx, dc, d, up, upEndedOK)
		}()
	}
}

// runDependent runs the keys of d after the end of its upstream up, under d's try-lock, unless d is
// running, already ran for the day, or another upstream is still running. Its own dependents follow.
func (r *reconciler) runDependent(ctx context.Context, dc *dayCtx, d contracts.Job, up string, upEndedOK bool) {
	// Recorded before the lock is asked: a turn of d that holds it and leaves d to its upstream sees the
	// trigger when it unlocks and runs d itself.
	dc.want(d.ID(), recheck{up: up, ok: upEndedOK})
	unlock, ok := r.locks.TryLock(d.ID())
	if !ok {
		return
	}
	dc.take(d.ID())
	end, ok := r.dependentRun(ctx, dc, d, up, upEndedOK, unlock)
	if ok && end.ended() && ctx.Err() == nil {
		r.trigger(ctx, dc, d.ID(), !end.failed)
	}
}

// dependentRun runs the keys of d under its lock, which it releases on every return.
func (r *reconciler) dependentRun(
	ctx context.Context, dc *dayCtx, d contracts.Job, up string, upEndedOK bool, unlock func(),
) (jobEnd, bool) {
	defer unlock()
	present, byKey, err := r.dayState(ctx, dc.day)
	if err != nil {
		dc.fail(err)
		return jobEnd{}, false
	}
	keys := dc.keysOf(d.ID())
	for _, k := range keys {
		if _, ok := present[k]; ok || len(byKey[k]) > 0 {
			return jobEnd{}, false // it ran for the day
		}
	}
	mark, leave, err := r.gate(ctx, dc, d, map[string]bool{up: upEndedOK})
	if err != nil {
		dc.fail(err)
		return jobEnd{}, false
	}
	if leave {
		return jobEnd{}, false
	}
	var end jobEnd
	for _, k := range keys {
		closeAt, err := r.closeOf(ctx, k.DatasetID, dc.day)
		if err != nil {
			dc.fail(err)
			return jobEnd{}, false
		}
		now := r.clk.Now()
		reason, deadline := mark, deadlineFor(d, closeAt, now)
		if dc.console {
			deadline = now.Add(runBound(d))
		}
		if reason == "" {
			reason = r.plainReason(dc, closeAt, now)
		}
		rec, err := r.kr.RunKey(ctx, d, dc.day, k, len(byKey[k])+1, closeAt, deadline, reason)
		if err != nil {
			dc.fail(err)
			return jobEnd{}, false
		}
		end.add(rec)
	}
	return end, true
}

// plainReason is the reason of a run with no mark: backfill for a console call, late after the close.
func (r *reconciler) plainReason(dc *dayCtx, closeAt, now time.Time) string {
	switch {
	case dc.console:
		return reasonBackfill
	case !closeAt.IsZero() && now.After(closeAt):
		return reasonLate
	}
	return ""
}

// jobTurn is the result of the keys of one job in one day. skipped: the job was locked or left to the
// end of its upstream, so a key of it may still have been run by a trigger.
type jobTurn struct {
	complete, skipped bool
	err               error
}

// reconcileJob runs the keys of job under its try-lock: a busy job is skipped and its keys stay missing
// for the next tick. Before the first key that starts it asks the order of its upstreams. When the turn
// ends the job's dependents are triggered.
func (r *reconciler) reconcileJob(ctx context.Context, dc *dayCtx, job contracts.Job, keys []contracts.ExpectedKey, closes map[string]time.Time) jobTurn {
	id := job.ID()
	unlock, ok := r.locks.TryLock(id)
	if !ok {
		dc.finish(id, true)
		return jobTurn{skipped: true}
	}
	t, end := r.lockedTurn(ctx, dc, job, keys, closes, unlock)
	dc.finish(id, false)
	// A turn that ended with no run (a key the brake stops, a key already counted) is an end without ok too:
	// the dependents left to it start marked. A turn left to its own upstream, or cut, is no end.
	if t.err == nil && !t.skipped && !end.cut && ctx.Err() == nil {
		r.trigger(ctx, dc, id, !end.failed && t.complete)
	}
	// The upstream of a job this turn left may have ended while the turn held the lock: its trigger found
	// the job locked and recorded itself; the job starts now.
	if t.skipped && t.err == nil && ctx.Err() == nil {
		if c, ok := dc.take(id); ok {
			r.runDependent(ctx, dc, job, c.up, c.ok)
		}
	}
	return t
}

// lockedTurn is reconcileKeys under the job's lock, which it releases on every return.
func (r *reconciler) lockedTurn(
	ctx context.Context, dc *dayCtx, job contracts.Job, keys []contracts.ExpectedKey, closes map[string]time.Time, unlock func(),
) (jobTurn, jobEnd) {
	defer unlock()
	return r.reconcileKeys(ctx, dc, job, keys, closes)
}

func (r *reconciler) reconcileKeys(
	ctx context.Context, dc *dayCtx, job contracts.Job, keys []contracts.ExpectedKey, closes map[string]time.Time,
) (jobTurn, jobEnd) {
	t := jobTurn{complete: true}
	var end jobEnd
	fail := func(err error) (jobTurn, jobEnd) { return jobTurn{err: err}, end }
	present, byKey, err := r.dayState(ctx, dc.day)
	if err != nil {
		return fail(err)
	}
	var (
		gated, leave bool
		mark         string
	)
	for _, k := range keys {
		if _, ok := present[k]; ok {
			continue
		}
		closeAt := closes[k.DatasetID]
		if expired, err := r.expireIfClosed(ctx, byKey[k], closeAt, dc.day, k); err != nil {
			return fail(err)
		} else if expired {
			continue
		}
		crashed, start, reason := r.nextAttempt(byKey[k], closeAt)
		// A lapsed run is closed as crashed even when the brake stops the key.
		if crashed != nil {
			if err := r.markCrashed(ctx, *crashed); err != nil {
				return fail(err)
			}
		}
		if !start {
			t.complete = false
			continue
		}
		if !gated {
			gated = true
			if mark, leave, err = r.gate(ctx, dc, job, nil); err != nil {
				return fail(err)
			}
		}
		if leave {
			t.complete, t.skipped = false, true
			continue
		}
		if mark != "" {
			reason = mark
		}
		rec, ok, err := r.run(ctx, job, dc.day, k, len(byKey[k])+1, closeAt, reason)
		if err != nil {
			return fail(err)
		}
		end.add(rec)
		t.complete = t.complete && ok
	}
	return t, end
}
