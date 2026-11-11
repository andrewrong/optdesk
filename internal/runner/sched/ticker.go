package sched

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"sync/atomic"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// The errors Run returns instead of exiting; the exit code is cmd/ot's.
var (
	// ErrFenceLost: the instance fence was lost (its connection died). cmd/ot exits non-zero.
	ErrFenceLost = errors.New("sched: instance fence lost")
	// ErrLoopFrozen: the loop did not turn for 60 seconds of the clock. cmd/ot exits non-zero.
	ErrLoopFrozen = errors.New("sched: loop frozen")
	// ErrExecutableChanged: the executable identity changed; the ticker drained. cmd/ot exits 75.
	ErrExecutableChanged = errors.New("sched: executable replaced")
)

const (
	firstTickDelay = 10 * time.Minute // the first Reconcile tick is start + 10 minutes
	tickEvery      = 15 * time.Minute // then every 15 minutes
	ceiling        = 30 * time.Second // the loop recomputes at least this often
	busyRetry      = 5 * time.Second  // a due turn that found the worker busy is retried after this
	drainGrace     = 30 * time.Second // the worker may finish its task this long after a stop
	killGrace      = 5 * time.Second  // after the cancel the worker gets this long to return
	frozenAfter    = 60 * time.Second // a loop that has not turned this long is frozen
	watchdogPoll   = 200 * time.Millisecond
	frozenPolls    = 5 // polls in a row that must read frozen: a jump of the clock must not look like a freeze

	reconcileTimeout = 2 * time.Hour
	probeTimeout     = time.Minute
	syncTimeout      = 2 * time.Minute
	ledgerTimeout    = 30 * time.Second

	scheduleLookback = 24 * time.Hour // starts are computed from now minus this, so a window in progress is seen
	scheduleDays     = 4
	defaultCalendar  = "XNYS" // the calendar the reconciler walks when its rule names none
)

// New builds the scheduler ticker; it has the type runner.NewSchedulerFunc. fence is held already
// (cmd/ot took it): the ticker never locks, it watches Lost and closes the fence on drain. The loop
// goroutine only reads the clock and the published schedule; every ledger, calendar, probe,
// Reconcile and Sync call runs on the one worker goroutine under its own timeout.
func New(fence store.Fence, sched store.SchedulerLedger, ops store.OpsLedger, cal clock.Calendar, clk clockwork.Clock, rec runner.Reconciler, gaps runner.GapSyncer, jobs []contracts.Job, disabled map[string]string, exe runner.ExecIdentityFunc) runner.Scheduler { //nolint:lll // the signature is fixed by runner.NewSchedulerFunc
	t := &ticker{
		fence: fence, sched: sched, ops: ops, cal: cal, clk: clk, rec: rec, gaps: gaps, jobs: jobs, exe: exe,
		disabled: maps.Clone(disabled),
		tasks:    make(chan turn, 1),
		wake:     make(chan struct{}, 1),
	}
	if t.disabled == nil {
		t.disabled = map[string]string{}
	}
	t.log = slog.Default()
	t.known = t.probeKnown()
	t.lastTick.Store(clk.Now().UnixNano())
	t.held.Store(true)
	return t
}

// turn is one worker task. A reconcile turn probes, refreshes and reconciles day; the others refresh only.
type turn struct {
	reconcile bool
	day       contracts.Day
}

type ticker struct {
	fence store.Fence
	sched store.SchedulerLedger
	ops   store.OpsLedger
	cal   clock.Calendar
	clk   clockwork.Clock
	rec   runner.Reconciler
	gaps  runner.GapSyncer
	jobs  []contracts.Job
	exe   runner.ExecIdentityFunc

	known map[string]bool // the calendars of the jobs, other than defaultCalendar, that answered at construction
	log   *slog.Logger

	disabled map[string]string // owned by the worker goroutine once Run has started

	lastTick atomic.Int64 // UnixNano of the last loop turn, or of construction
	ticks    atomic.Uint64
	held     atomic.Bool
	busy     atomic.Bool
	starts   atomic.Pointer[[]Start] // the schedule published by the worker
	tasks    chan turn
	wake     chan struct{} // the worker published a new schedule
}

func (t *ticker) LastTick() time.Time { return time.Unix(0, t.lastTick.Load()).UTC() }
func (t *ticker) FenceHeld() bool     { return t.held.Load() }

// Run loops until ctx ends (nil after the drain), the fence is lost (ErrFenceLost), the loop freezes
// (ErrLoopFrozen) or the executable is replaced (ErrExecutableChanged).
func (t *ticker) Run(ctx context.Context) error {
	first, err := t.exe()
	if err != nil {
		return err
	}
	start := t.clk.Now()
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	workCtx, cancelWork := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelWork()

	fail := make(chan error, 4)
	go t.watchFence(runCtx, stop, fail)
	go t.watchdog(runCtx, fail)
	workerDone := make(chan struct{})
	go t.worker(runCtx, workCtx, workerDone)
	loopDone := make(chan error, 1)
	go func() { loopDone <- t.loop(runCtx, start, first) }()

	var runErr error
	select {
	case runErr = <-loopDone:
		select {
		case e := <-fail:
			runErr = e
		default:
		}
	case runErr = <-fail:
	}
	stop()
	if errors.Is(runErr, ErrLoopFrozen) {
		cancelWork()
		return t.finish(runErr)
	}
	// Drain: the worker finishes its task within the grace, then it is cancelled.
	if !t.waitDone(workerDone, drainGrace) {
		cancelWork()
		t.waitDone(workerDone, killGrace)
	}
	return t.finish(runErr)
}

// finish closes the fence and returns runErr, or the close error when there is none.
func (t *ticker) finish(runErr error) error {
	if err := t.fence.Close(); runErr == nil {
		return err
	}
	return runErr
}

// waitDone waits on the clock until done closes (true) or d passes (false).
func (t *ticker) waitDone(done <-chan struct{}, d time.Duration) bool {
	tm := t.clk.NewTimer(d)
	defer tm.Stop()
	select {
	case <-done:
		return true
	case <-tm.Chan():
		return false
	}
}

func (t *ticker) watchFence(ctx context.Context, stop context.CancelFunc, fail chan<- error) {
	select {
	case <-t.fence.Lost():
		t.held.Store(false)
		fail <- ErrFenceLost
		stop()
	case <-ctx.Done():
	}
}

// watchdog reports ErrLoopFrozen when the turn counter has not moved between polls and the last
// turn is frozenAfter or more behind the clock. It polls on real time, so it holds no waiter of the
// injected clock; frozenPolls polls in a row keep a jump of the clock, or a starved loop, from reading as a freeze.
func (t *ticker) watchdog(ctx context.Context, fail chan<- error) {
	poll := time.NewTicker(watchdogPoll)
	defer poll.Stop()
	seen, strikes := t.ticks.Load(), 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
		}
		n := t.ticks.Load()
		if n != seen || t.clk.Now().Sub(t.LastTick()) < frozenAfter {
			seen, strikes = n, 0
			continue
		}
		if strikes++; strikes >= frozenPolls {
			fail <- ErrLoopFrozen
			return
		}
	}
}

// loop is the one waiting goroutine. It calls no ledger, calendar, broker or job.
func (t *ticker) loop(ctx context.Context, start time.Time, first string) error {
	nextTick, mark, lastTurn, skipped := start.Add(firstTickDelay), start, start, false
	// The first turn reconciles the start day: a process that lives under firstTickDelay still plans
	// a key that is inside its window and not done, and a start after the close only expires it.
	t.submit(turn{reconcile: true, day: dayOf(start)})
	for {
		now := t.clk.Now()
		fire, stopTimer := t.timer(now, t.waitFor(now, lastTurn, nextTick, mark, skipped))
		select {
		case <-ctx.Done():
			stopTimer()
			return nil
		case <-t.wake:
			stopTimer()
			continue
		case <-fire:
		}
		stopTimer()
		now = t.clk.Now()
		lastTurn = now
		t.lastTick.Store(now.UnixNano())
		t.ticks.Add(1)
		tickDue := !now.Before(nextTick)
		slotDue := t.slotDue(mark, now)
		if tickDue {
			if id, err := t.exe(); err == nil && id != first {
				return ErrExecutableChanged
			}
		}
		skipped = false
		if !t.submit(turn{reconcile: tickDue || slotDue, day: dayOf(now)}) {
			skipped = tickDue || slotDue
			continue
		}
		if tickDue {
			for !nextTick.After(now) {
				nextTick = nextTick.Add(tickEvery)
			}
		}
		if tickDue || slotDue {
			mark = now // this Reconcile covers every start up to now
		}
	}
}

// waitFor is the time to the next of the next tick, the next slot start and the ceiling after the last
// turn, from now; a wake for a new schedule does not postpone the ceiling.
func (t *ticker) waitFor(now, lastTurn, nextTick, mark time.Time, skipped bool) time.Duration {
	if skipped {
		return busyRetry
	}
	wait := min(lastTurn.Add(ceiling).Sub(now), nextTick.Sub(now))
	if at, ok := t.nextSlot(mark); ok {
		wait = min(wait, at.Sub(now))
	}
	return max(wait, 0)
}

// timer returns a channel that fires after d of the clock from now, and the function that stops it. It
// fires at once, without a waiter on the clock, when d is zero or when the clock moved d or more between
// reading now and creating the timer (a timer made after that jump would sleep another d).
func (t *ticker) timer(now time.Time, d time.Duration) (<-chan time.Time, func()) {
	if d > 0 {
		tm := t.clk.NewTimer(d)
		if t.clk.Now().Sub(now) < d {
			return tm.Chan(), func() { tm.Stop() }
		}
		tm.Stop()
	}
	c := make(chan time.Time, 1)
	c <- t.clk.Now()
	return c, func() {}
}

// nextSlot is the first published start after mark.
func (t *ticker) nextSlot(mark time.Time) (time.Time, bool) {
	p := t.starts.Load()
	if p == nil {
		return time.Time{}, false
	}
	for _, s := range *p {
		if s.At.After(mark) {
			return s.At, true
		}
	}
	return time.Time{}, false
}

func (t *ticker) slotDue(mark, now time.Time) bool {
	at, ok := t.nextSlot(mark)
	return ok && !at.After(now)
}

// submit hands tn to the worker unless it is busy; it never blocks.
func (t *ticker) submit(tn turn) bool {
	if !t.busy.CompareAndSwap(false, true) {
		return false
	}
	t.tasks <- tn
	return true
}

func (t *ticker) worker(runCtx, workCtx context.Context, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-runCtx.Done():
			return
		case tn := <-t.tasks:
			t.runTurn(workCtx, tn)
			t.busy.Store(false)
		}
	}
}

// runTurn is the worker task. Errors are not fatal: the next turn tries again, and the gap syncer
// and ops.scheduler_job show what is missing.
func (t *ticker) runTurn(ctx context.Context, tn turn) {
	if tn.reconcile {
		t.probe(ctx)
	}
	t.refresh(ctx)
	if tn.reconcile {
		t.reconcileAndSync(ctx, tn.day)
	}
}

// probe re-probes every left-out job when the reconciler can; a passing job leaves the map.
func (t *ticker) probe(ctx context.Context) {
	p, ok := t.rec.(runner.Prober)
	if !ok {
		return
	}
	for _, id := range slices.Sorted(maps.Keys(t.disabled)) {
		pctx, cancel := context.WithTimeout(ctx, probeTimeout)
		reason := p.Probe(pctx, id)
		cancel()
		if reason == "" {
			delete(t.disabled, id)
		} else {
			t.disabled[id] = reason
		}
	}
}

// refresh recomputes the schedule of the enabled jobs, publishes it and writes ops.scheduler_job.
func (t *ticker) refresh(ctx context.Context) {
	now := t.clk.Now()
	slots := t.slots()
	cctx, cancel := context.WithTimeout(ctx, ledgerTimeout)
	defer cancel()
	starts, err := nextStarts(cctx, t.cal, slots, now.Add(-scheduleLookback), scheduleDays, t.known, t.log)
	if err != nil {
		if p := t.starts.Load(); p != nil {
			starts = *p
		}
	}
	t.starts.Store(&starts)
	select {
	case t.wake <- struct{}{}:
	default:
	}
	// The write is best effort: the next turn replaces the rows.
	_ = t.sched.PutJobStates(cctx, t.states(cctx, now, slots, starts))
}

// slots are the plain schedules of the jobs that are not left out.
func (t *ticker) slots() []runner.Slot {
	var out []runner.Slot
	for _, j := range t.jobs {
		s, ok := j.(runner.Scheduled)
		if !ok {
			continue
		}
		if _, off := t.disabled[j.ID()]; off {
			continue
		}
		cal, anchor, zone, offset := s.Schedule()
		out = append(out, runner.Slot{JobID: j.ID(), CalendarID: cal, Anchor: anchor, Zone: zone, Offset: offset})
	}
	return out
}

// states is one row per job: its next start, the end of the window it is in or next opens, whether a
// restart must be avoided, and why it is left out.
func (t *ticker) states(ctx context.Context, now time.Time, slots []runner.Slot, starts []Start) []store.JobState {
	slotOf := map[string]runner.Slot{}
	for _, s := range slots {
		slotOf[s.JobID] = s
	}
	out := make([]store.JobState, 0, len(t.jobs))
	for _, j := range t.jobs {
		st := store.JobState{JobID: j.ID(), DisabledReason: t.disabled[j.ID()]}
		if r, ok := j.(runner.RestartSensitive); ok {
			st.RestartSensitive = r.RestartSensitive()
		}
		for _, s := range starts {
			if s.JobID != j.ID() {
				continue
			}
			if st.NextStart.IsZero() && s.At.After(now) {
				st.NextStart = s.At
			}
			if end := t.windowEnd(ctx, j, slotOf[j.ID()].CalendarID, s.At); st.WindowEnd.IsZero() && end.After(now) {
				st.WindowEnd = end
			}
		}
		out = append(out, st)
	}
	return out
}

// windowEnd is the close of the window of the day of at: the session close of the calendar of the job
// plus the close offset of its window. It is zero when the day is no session.
func (t *ticker) windowEnd(ctx context.Context, j contracts.Job, calendarID string, at time.Time) time.Time {
	var closeOffset time.Duration
	if w, ok := j.(runner.Windowed); ok {
		id, _, off := w.Window()
		if id != "" {
			calendarID = id
		}
		closeOffset = off
	}
	if calendarID == "" {
		calendarID = defaultCalendar
	}
	s, err := t.cal.Session(ctx, calendarID, dayOf(at))
	if err != nil || !s.IsSession {
		return time.Time{}
	}
	return s.CloseAt.Add(closeOffset)
}

// reconcileAndSync runs Reconcile for day, then Sync for each day the call walked.
func (t *ticker) reconcileAndSync(ctx context.Context, day contracts.Day) {
	var cursor contracts.Day
	haveCursor := false
	if t.gaps != nil {
		cctx, cancel := context.WithTimeout(ctx, ledgerTimeout)
		cur, err := t.ops.Cursor(cctx)
		cancel()
		cursor, haveCursor = cur.Day, err == nil
	}
	rctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	err := t.rec.Reconcile(rctx, day)
	cancel()
	if err != nil {
		kind := "error"
		var te contracts.TypedError
		if errors.As(err, &te) {
			kind = string(te.Kind())
		}
		t.log.Error("reconcile failed", slog.String("day", string(day)), slog.String("kind", kind))
	}
	if t.gaps == nil || !haveCursor {
		return
	}
	for _, d := range t.walked(ctx, cursor, day) {
		sctx, cancel := context.WithTimeout(ctx, syncTimeout)
		_ = t.gaps.Sync(sctx, d, t.disabledCopy())
		cancel()
	}
}

func (t *ticker) disabledCopy() map[string]string {
	c := make(map[string]string, len(t.disabled))
	maps.Copy(c, t.disabled)
	return c
}

// jobCalendars lists the distinct calendars of the jobs, in order: that of the window of each job
// (defaultCalendar when it names none); defaultCalendar alone when there is no job.
func (t *ticker) jobCalendars() []string {
	seen := map[string]bool{}
	var out []string
	for _, j := range t.jobs {
		id := defaultCalendar
		if w, ok := j.(runner.Windowed); ok {
			if c, _, _ := w.Window(); c != "" {
				id = c
			}
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		out = append(out, defaultCalendar)
	}
	sort.Strings(out)
	return out
}

// probeKnown asks every calendar of the jobs, the window and the slot ones, but defaultCalendar for
// the session of today; the ones that answer are the known set.
func (t *ticker) probeKnown() map[string]bool {
	known := map[string]bool{}
	if t.cal == nil {
		return known
	}
	ids := t.jobCalendars()
	for _, s := range t.slots() {
		ids = append(ids, s.CalendarID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), ledgerTimeout)
	defer cancel()
	today := dayOf(t.clk.Now())
	for _, id := range ids {
		if id == defaultCalendar || id == "" || known[id] {
			continue
		}
		if _, err := t.cal.Session(ctx, id, today); err == nil {
			known[id] = true
		}
	}
	return known
}

// walked lists the session days a Reconcile of today walks: the days after the cursor through today
// that are a session of at least one calendar of the jobs, in order; with no cursor, today alone when
// it is a session of one. A calendar that fails ends its own list.
func (t *ticker) walked(ctx context.Context, cursor, today contracts.Day) []contracts.Day {
	cctx, cancel := context.WithTimeout(ctx, ledgerTimeout)
	defer cancel()
	set := map[contracts.Day]bool{}
	for _, id := range t.jobCalendars() {
		if cursor == "" {
			if s, err := t.cal.Session(cctx, id, today); err == nil && s.IsSession {
				set[today] = true
			}
			continue
		}
		from := cursor
		for from < today {
			s, err := t.cal.NextSession(cctx, id, from)
			if err != nil || s.Day > today {
				break
			}
			set[s.Day] = true
			from = s.Day
		}
	}
	var out []contracts.Day
	for d := range set {
		out = append(out, d)
	}
	slices.Sort(out)
	return out
}

func dayOf(at time.Time) contracts.Day { return contracts.Day(at.UTC().Format(dayLayout)) }
