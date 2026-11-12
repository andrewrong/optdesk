package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/jonboulle/clockwork"
)

// jcLog is the ordered record of the calls of the fakes of one host.
type jcLog struct{ items []string }

func (l *jcLog) add(format string, a ...any) { l.items = append(l.items, fmt.Sprintf(format, a...)) }
func (l *jcLog) count(prefix string) (n int) {
	for _, s := range l.items {
		if strings.HasPrefix(s, prefix) {
			n++
		}
	}
	return n
}

type jcFinish struct {
	id     int64
	state  jobrequest.State
	reason string
	lines  []jobrequest.KeyLine
}

type jcQueue struct {
	log         *jcLog
	rows        []jobrequest.Row
	queuedErr   error
	queuedBlock bool
	claimHook   func(id int64)
	failErrs    []error // one per FailRunning call; nil after the list ends
	failCalls   int
	finishes    []jcFinish
	onFinish    func(ctx context.Context)
}

func (q *jcQueue) Queued(ctx context.Context, _ int) ([]jobrequest.Row, error) {
	q.log.add("queued")
	if q.queuedBlock {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	var out []jobrequest.Row
	for _, r := range q.rows {
		if r.State == jobrequest.StateQueued {
			out = append(out, r)
		}
	}
	return out, q.queuedErr
}

func (q *jcQueue) Claim(_ context.Context, id int64, _ time.Time) (bool, error) {
	q.log.add("claim:%d", id)
	if q.claimHook != nil {
		q.claimHook(id)
	}
	return true, nil
}

func (q *jcQueue) Hold(_ context.Context, id int64, job string, at time.Time) error {
	q.log.add("hold:%d:%s:%s", id, job, at.UTC().Format(time.RFC3339))
	return nil
}

func (q *jcQueue) Finish(ctx context.Context, id int64, s jobrequest.State, reason string, lines []jobrequest.KeyLine, _ time.Time) error {
	q.log.add("finish:%d:%s:%s", id, s, reason)
	q.finishes = append(q.finishes, jcFinish{id, s, reason, lines})
	if q.onFinish != nil {
		q.onFinish(ctx)
	}
	return nil
}

func (q *jcQueue) FailRunning(_ context.Context, reason string, now time.Time) error {
	q.log.add("fail_running:%s:%s", reason, now.Format(time.RFC3339))
	if q.failCalls++; q.failCalls <= len(q.failErrs) {
		return q.failErrs[q.failCalls-1]
	}
	return nil
}

func (q *jcQueue) Close() error { return nil }

type jcBackfill struct {
	log      *jcLog
	res      runner.BackfillResult
	err      error
	fn       func(ctx context.Context) (runner.BackfillResult, error)
	days     []contracts.Day
	datasets [][]string
	opts     []runner.BackfillOptions
}

func (b *jcBackfill) BackfillWith(ctx context.Context, day contracts.Day, datasets []string, opt runner.BackfillOptions) (runner.BackfillResult, error) {
	b.log.add("backfill")
	b.days, b.datasets, b.opts = append(b.days, day), append(b.datasets, datasets), append(b.opts, opt)
	if b.fn != nil {
		return b.fn(ctx)
	}
	return b.res, b.err
}

func (l *jcLog) SyncKeys(_ context.Context, day contracts.Day) error {
	l.add("sync:%s", day)
	return nil
}

// jcJob is a fake job; jcBounded adds a RunBound.
type jcJob struct {
	id     string
	keys   []contracts.ExpectedKey
	planFn func() error
}

func (j jcJob) ID() string { return j.id }
func (j jcJob) Plan(context.Context, contracts.Day) (keys []contracts.ExpectedKey, h string, err error) {
	if j.planFn != nil {
		err = j.planFn()
	}
	return j.keys, "h", err
}
func (jcJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) (r contracts.JobResult) {
	return r
}

type jcBounded struct {
	jcJob
	bound time.Duration
}

func (j jcBounded) RunBound() time.Duration { return j.bound }

type jcTyped struct{ kind contracts.ErrorKind }

func (e jcTyped) Error() string             { return "typed" }
func (e jcTyped) Kind() contracts.ErrorKind { return e.kind }

type jcRig struct {
	t    *testing.T
	log  *jcLog
	q    *jcQueue
	bf   *jcBackfill
	fc   *clockwork.FakeClock
	out  *bytes.Buffer
	host *consoleHost
}

var jcKey = []contracts.ExpectedKey{{DatasetID: "ds.a", SubjectKey: "s1"}, {DatasetID: "ds.a", SubjectKey: "s2"}, {DatasetID: "ds.b", SubjectKey: "s3"}}

// newJCRig builds a host over fakes at 2026-11-12 15:00 UTC with one plain job per id given.
func newJCRig(t *testing.T, ids ...string) *jcRig {
	t.Helper()
	l := &jcLog{}
	r := &jcRig{t: t, log: l, q: &jcQueue{log: l}, bf: &jcBackfill{log: l},
		fc: clockwork.NewFakeClockAt(time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)), out: &bytes.Buffer{}}
	r.host = newConsoleHost(r.q, l, r.fc, slog.New(slog.NewTextHandler(r.out, nil)))
	var jobs []contracts.Job
	for _, id := range ids {
		jobs = append(jobs, jcJob{id: id, keys: jcKey})
	}
	r.host.set(jobs, r.bf)
	return r
}

func (r *jcRig) add(id int64, job, day string, force bool) *jobrequest.Row {
	r.q.rows = append(r.q.rows, jobrequest.Row{ID: id, State: jobrequest.StateQueued, Request: jobrequest.Request{JobID: job, Day: day, Force: force}})
	return &r.q.rows[len(r.q.rows)-1]
}

func (r *jcRig) turn(hold runner.SlotHold) bool {
	r.t.Helper()
	took, err := r.host.ConsoleTurn(r.t.Context(), hold)
	if err != nil {
		r.t.Fatalf("ConsoleTurn error %v", err)
	}
	return took
}

func (r *jcRig) wantEnd(s jobrequest.State, reason string) jcFinish {
	r.t.Helper()
	if len(r.q.finishes) == 0 {
		r.t.Fatalf("no Finish; calls %v", r.log.items)
	}
	f := r.q.finishes[len(r.q.finishes)-1]
	if f.state != s || f.reason != reason {
		r.t.Fatalf("ended %s/%s, want %s/%s", f.state, f.reason, s, reason)
	}
	return f
}

const jcDay, jcStart = "2026-11-12", "fail_running:restart:2026-11-12T15:00:00Z"

func jcNever(time.Duration) (string, time.Time, bool) { return "", time.Time{}, false }

func jcOne(t *testing.T, job string) *jcRig {
	r := newJCRig(t, job)
	r.add(1, job, jcDay, false)
	return r
}

func jcResult(t *testing.T, res runner.BackfillResult, err error, s jobrequest.State, reason string) jcFinish {
	t.Helper()
	r := jcOne(t, "fred_series")
	r.bf.res, r.bf.err = res, err
	r.turn(jcNever)
	return r.wantEnd(s, reason)
}

func jcLines(g ...string) (out runner.BackfillResult) {
	for i, group := range g {
		out.Keys = append(out.Keys, runner.KeyRun{Key: jcKey[i], Group: group})
	}
	return out
}

func TestOrderOfTurn(t *testing.T) {
	t.Parallel()
	r := newJCRig(t)
	r.host.set([]contracts.Job{
		jcBounded{jcJob{id: "quote_snapshot", keys: jcKey}, 16 * time.Minute},
		jcBounded{jcJob{id: "fred_series", keys: jcKey}, 5 * time.Minute},
	}, r.bf)
	r.add(1, "fred_series", jcDay, false).WithdrawnAt = r.fc.Now()
	r.add(2, "quote_snapshot", jcDay, false)
	r.add(3, "fred_series", jcDay, false)
	r.bf.res = jcLines(runner.GroupRan)
	hold := func(b time.Duration) (string, time.Time, bool) {
		return "slot_job", r.fc.Now().Add(time.Hour), b == 19*time.Minute+30*time.Second
	}
	if !r.turn(hold) {
		t.Fatal("took false")
	}
	want := []string{jcStart, "queued", "finish:1:withdrawn:withdrawn",
		"hold:2:slot_job:2026-11-12T16:00:00Z", "claim:3", "backfill", "finish:3:done:completed", "sync:" + jcDay}
	if got := r.log.items; !slices.Equal(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
}

func TestHeldRequestGetsHoldWrittenAndNextIsTaken(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series", "fundamentals")
	r.add(1, "fred_series", jcDay, false)
	r.add(2, "fundamentals", jcDay, false)
	at, calls := r.fc.Now().Add(20*time.Minute), 0
	r.turn(func(time.Duration) (string, time.Time, bool) { calls++; return "slot_job", at, calls == 1 })
	want := []string{jcStart, "queued", "hold:1:slot_job:2026-11-12T15:20:00Z", "claim:2", "backfill", "finish:2:done:completed"}
	if got := r.log.items; !slices.Equal(got, want) {
		t.Fatalf("calls %v, want %v", got, want)
	}
}

func TestHoldBoundIsTurnMax(t *testing.T) {
	t.Parallel()
	r := newJCRig(t)
	r.host.set([]contracts.Job{
		jcBounded{jcJob{id: "quote_snapshot", keys: jcKey}, 16 * time.Minute},
		jcJob{id: "fundamentals", keys: jcKey},
		jcBounded{jcJob{id: "fred_series", keys: jcKey}, 5 * time.Minute},
	}, r.bf)
	for i, id := range []string{"quote_snapshot", "fundamentals", "fred_series"} {
		r.add(int64(i+1), id, jcDay, false)
	}
	var bounds []time.Duration
	r.turn(func(b time.Duration) (string, time.Time, bool) {
		bounds = append(bounds, b)
		return "x", r.fc.Now(), true
	})
	if want := []time.Duration{19*time.Minute + 30*time.Second, 33*time.Minute + 30*time.Second, 8*time.Minute + 30*time.Second}; !slices.Equal(bounds, want) {
		t.Fatalf("bounds %v, want %v", bounds, want)
	}
}

func TestStartPhaseTimesOutAfterAMinute(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	r.q.queuedBlock = true
	done := make(chan bool, 1)
	go func() { took, err := r.host.ConsoleTurn(t.Context(), jcNever); done <- took || err != nil }()
	if err := r.fc.BlockUntilContext(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	r.fc.Advance(59 * time.Second)
	if len(done) != 0 {
		t.Fatal("the turn ended before a minute")
	}
	if r.fc.Advance(time.Second); <-done {
		t.Fatal("took a request or failed")
	}
}

func TestHoldPassedToBackfillWith(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	at := r.fc.Now().Add(time.Hour)
	r.turn(func(time.Duration) (string, time.Time, bool) { return "sj", at, false })
	if opt := r.bf.opts[0]; opt.Hold == nil {
		t.Fatal("no Hold given")
	} else if job, got, held := opt.Hold(time.Minute); job != "sj" || !got.Equal(at) || held {
		t.Fatalf("Hold answered %q %v %v", job, got, held)
	}
}

func TestSlotNearEndsFailedSlotNear(t *testing.T) {
	t.Parallel()
	res := jcLines(runner.GroupRan, runner.GroupNotReached)
	res.Keys[1].ReasonCode = "slot_near"
	if f := jcResult(t, res, nil, jobrequest.StateFailed, "slot_near"); len(f.lines) != 2 {
		t.Fatalf("lines %v", f.lines)
	}
}

func TestRefusedNoKeyEndsRefusedNoKey(t *testing.T) {
	t.Parallel()
	jcResult(t, runner.BackfillResult{Refused: runner.RefusedNoKey}, nil, jobrequest.StateRefused, "no_key")
}

func TestWithdrawalAfterQueueReadBeforeClaimStillRuns(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	r.q.claimHook = func(int64) { r.q.rows[0].WithdrawnAt = r.fc.Now() } // lands after the read
	r.turn(jcNever)
	r.wantEnd(jobrequest.StateDone, "completed")
}

func TestForceForForceNoJobRefusedNotRunnable(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series")
	r.add(1, "fred_series", jcDay, true)
	r.turn(jcNever)
	r.wantEnd(jobrequest.StateRefused, "not_runnable")
}

func TestStartRetriedOnEveryTurn(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	r.q.failErrs = []error{errors.New("not migrated")}
	if r.turn(jcNever) || r.log.count("queued") != 0 {
		t.Fatalf("first turn took a request: %v", r.log.items)
	}
	if !r.turn(jcNever) || r.log.count("fail_running") != 2 {
		t.Fatalf("second turn: %v", r.log.items)
	}
	r.wantEnd(jobrequest.StateDone, "completed")
}

func TestStartErrorWarnsOncePerMinute(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series")
	r.q.failErrs = []error{errors.New("e"), errors.New("e"), errors.New("e")}
	for i, step := range []time.Duration{0, 30 * time.Second, 31 * time.Second} {
		r.fc.Advance(step)
		r.turn(jcNever)
		if n, want := strings.Count(r.out.String(), "level=WARN"), []int{1, 1, 2}[i]; n != want {
			t.Fatalf("turn %d: %d warn lines, want %d", i, n, want)
		}
	}
}

func TestHoldClearedWhenTaken(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series")
	row := r.add(1, "fred_series", jcDay, false)
	row.HoldJob, row.HoldAt = "slot_job", r.fc.Now()
	r.turn(jcNever)
	want := []string{jcStart, "queued", "hold:1::0001-01-01T00:00:00Z", "claim:1", "backfill", "finish:1:done:completed"}
	if got := r.log.items; !slices.Equal(got, want) {
		t.Fatalf("calls %v, want %v", got, want)
	}
}

func TestWithdrawalFinishesWithdrawn(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series")
	r.add(7, "fred_series", jcDay, false).WithdrawnAt = r.fc.Now()
	if took, f := r.turn(jcNever), r.wantEnd(jobrequest.StateWithdrawn, "withdrawn"); !took || f.id != 7 || f.lines != nil {
		t.Fatalf("took %v, finished %+v", took, f)
	}
}

func TestWithdrawalOfRunningIgnored(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series")
	row := r.add(1, "fred_series", jcDay, false)
	row.State, row.WithdrawnAt = jobrequest.StateRunning, r.fc.Now()
	if r.turn(jcNever) || len(r.q.finishes) != 0 {
		t.Fatalf("a running request was touched: %+v", r.q.finishes)
	}
}

func TestLeftOutAndNotRunnable(t *testing.T) {
	t.Parallel()
	for job, reason := range map[string]string{"calendar_refresh": "left_out", "no_such_job": "not_runnable", "option_ledger": "not_runnable"} {
		r := newJCRig(t, "fred_series", "option_ledger") // calendar_refresh is in the table and not in the set
		r.add(1, job, jcDay, false)
		r.turn(jcNever)
		if r.wantEnd(jobrequest.StateRefused, reason); r.log.count("backfill") != 0 {
			t.Fatalf("%s: the backfiller ran", job)
		}
	}
}

func TestNotTodayRefusedAtClaim(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "calendar_refresh")
	r.add(1, "calendar_refresh", jcDay, false)
	r.fc.Advance(15 * time.Hour) // past midnight in New York
	r.turn(jcNever)
	r.wantEnd(jobrequest.StateRefused, "not_today")
}

func TestTodayOnlyUsesTheJobsZone(t *testing.T) {
	t.Parallel()
	// At 20:00 UTC it is 15:00 on the 12th in New York and 05:00 on the 13th in Tokyo.
	for _, c := range [][3]string{{"option_eod_jp", "2026-11-13", "ran"}, {"option_eod_jp", "2026-11-12", "not_today"},
		{"option_eod", "2026-11-13", "not_today"}, {"option_eod", "2026-11-12", "ran"}} {
		r := newJCRig(t, c[0])
		r.fc.Advance(5 * time.Hour)
		r.add(1, c[0], c[1], false)
		r.turn(jcNever)
		if c[2] == "ran" && r.log.count("backfill") != 1 || c[2] != "ran" && r.wantEnd(jobrequest.StateRefused, c[2]).id != 1 {
			t.Errorf("%s %s: calls %v", c[0], c[1], r.log.items)
		}
	}
}

func TestNoKeyRefused(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	r.host.set([]contracts.Job{jcJob{id: "fred_series"}}, r.bf)
	r.turn(jcNever)
	r.wantEnd(jobrequest.StateRefused, "no_key")
}

func TestPlanFailed(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	r.host.set([]contracts.Job{jcJob{id: "fred_series", planFn: func() error { return errors.New("db down") }}}, r.bf)
	r.turn(jcNever)
	r.wantEnd(jobrequest.StateFailed, "plan_failed")
}

func TestNoPlanRefused(t *testing.T) {
	t.Parallel()
	jcResult(t, runner.BackfillResult{Refused: runner.RefusedNoPlan}, nil, jobrequest.StateRefused, "no_plan")
}

func TestCancelEndsInterruptedWithLinesSoFar(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	ctx, cancel := context.WithCancel(t.Context())
	r.bf.fn = func(context.Context) (runner.BackfillResult, error) {
		cancel()
		return jcLines(runner.GroupRan, runner.GroupNotReached), errors.New("context canceled")
	}
	_, _ = r.host.ConsoleTurn(ctx, jcNever)
	if f := r.wantEnd(jobrequest.StateInterrupted, "stopped"); len(f.lines) != 2 || r.log.count("sync") != 0 {
		t.Fatalf("lines %v calls %v", f.lines, r.log.items)
	}
}

func TestCancelDuringPlanIsInterrupted(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	ctx, cancel := context.WithCancel(t.Context())
	r.host.set([]contracts.Job{jcJob{id: "fred_series", planFn: func() error { cancel(); return context.Canceled }}}, r.bf)
	_, _ = r.host.ConsoleTurn(ctx, jcNever)
	r.wantEnd(jobrequest.StateInterrupted, "stopped")
}

func TestLeaseHeldEndsFailed(t *testing.T) {
	t.Parallel()
	jcResult(t, runner.BackfillResult{}, fmt.Errorf("lease: %w", jcTyped{contracts.Retryable}), jobrequest.StateFailed, "lease_held")
	jcResult(t, runner.BackfillResult{}, errors.New("other"), jobrequest.StateFailed, "error")
}

func TestFinishUsesUncancelledContextWith30sTimeout(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	ctx, cancel := context.WithCancel(t.Context())
	r.bf.fn = func(context.Context) (runner.BackfillResult, error) {
		cancel()
		return runner.BackfillResult{}, context.Canceled
	}
	var live bool
	r.q.onFinish = func(c context.Context) {
		r.fc.Advance(29 * time.Second)
		live = c.Err() == nil
		r.fc.Advance(time.Second)
		<-c.Done() // cut at 30 s of the fake clock
	}
	_, _ = r.host.ConsoleTurn(ctx, jcNever)
	if r.wantEnd(jobrequest.StateInterrupted, "stopped"); !live {
		t.Fatal("the Finish context was cut before 30 s")
	}
}

func TestSyncRunsOnlyAfterARanLine(t *testing.T) {
	t.Parallel()
	for sync, groups := range map[int][]string{
		0: {runner.GroupRerun, runner.GroupNotDue, runner.GroupLeaseHeld},
		1: {runner.GroupRerun, runner.GroupRan},
	} {
		r := jcOne(t, "fred_series")
		r.bf.res = jcLines(groups...)
		r.turn(jcNever)
		if r.log.count("sync:"+jcDay) != sync {
			t.Errorf("%d SyncKeys calls after %v, want %d", r.log.count("sync"), groups, sync)
		}
	}
}

func TestAllNotDueEndsRefusedNotDue(t *testing.T) {
	t.Parallel()
	res := jcLines(runner.GroupNotDue, runner.GroupAlreadyPresent)
	due := time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)
	res.Keys[0].DueAt = due
	f := jcResult(t, res, nil, jobrequest.StateRefused, "not_due")
	if l := f.lines; len(l) != 2 || !l[0].DueAt.Equal(due) || l[0].Dataset != "ds.a" || l[0].Subject != "s1" {
		t.Fatalf("lines %+v", l)
	}
}

func TestInterruptedStoresReasonStopped(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _ = r.host.ConsoleTurn(ctx, jcNever)
	r.wantEnd(jobrequest.StateInterrupted, "stopped")
}

func TestDoneStoresReasonCompleted(t *testing.T) {
	t.Parallel()
	jcResult(t, jcLines(runner.GroupAlreadyPresent), nil, jobrequest.StateDone, "completed")
}

func TestWithdrawnStoresReasonWithdrawn(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series")
	r.add(7, "fred_series", jcDay, false).WithdrawnAt = r.fc.Now()
	r.turn(jcNever)
	r.wantEnd(jobrequest.StateWithdrawn, "withdrawn")
}

func TestStartFailsRunningWithRestart(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "fred_series")
	err := r.host.Start(t.Context())
	if got := r.log.items; err != nil || len(got) != 1 || got[0] != "fail_running:restart:"+r.fc.Now().Format(time.RFC3339) {
		t.Fatalf("FailRunning %v, error %v", got, err)
	}
}

func TestStartErrorLeavesConsoleOff(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	r.q.failErrs = []error{errors.New("x")}
	if r.turn(jcNever) || r.log.count("queued")+r.log.count("claim")+r.log.count("backfill") != 0 {
		t.Fatalf("the console took a turn: %v", r.log.items)
	}
}

func TestQueuedErrorIsNotATurn(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	r.q.queuedErr = errors.New("postgres://user:secret@host/db")
	if r.turn(jcNever) || r.log.count("claim") != 0 || strings.Contains(r.out.String(), "secret") {
		t.Fatalf("calls %v log %s", r.log.items, r.out)
	}
}

func TestJobFilterPassedToBackfillWith(t *testing.T) {
	t.Parallel()
	r := newJCRig(t, "calendar_refresh")
	r.add(1, "calendar_refresh", jcDay, true)
	r.turn(jcNever)
	if len(r.bf.opts) != 1 || r.bf.opts[0].Job != "calendar_refresh" || !r.bf.opts[0].Force {
		t.Fatalf("calls %v options %+v", r.log.items, r.bf.opts)
	}
	if got := r.bf.datasets[0]; !slices.Equal(got, []string{"ds.a", "ds.b"}) || r.bf.days[0] != jcDay {
		t.Fatalf("datasets %v day %s", got, r.bf.days[0])
	}
}
