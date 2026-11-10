package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/sched"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
	"github.com/jonboulle/clockwork"
)

const (
	jpDay      = "2026-11-09"
	jpLastDay  = "2026-11-12"
	noXTKSLine = "calendar XTKS is not loaded"
)

// kindErr is a typed calendar error.
type kindErr struct{ kind contracts.ErrorKind }

func (e kindErr) Error() string             { return "calendar: " + string(e.kind) }
func (e kindErr) Kind() contracts.ErrorKind { return e.kind }

// jpCal is a fake clock.Calendar. XTKS has a session on every weekday through last and answers
// UnknownCalendar after it; with fail set it answers a plain error. Every other calendar has a session
// on every weekday, closing at 21:00 UTC.
type jpCal struct {
	last contracts.Day
	fail atomic.Bool
}

func (c *jpCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	tm, err := time.Parse("2006-01-02", string(day))
	if err != nil {
		return clock.Session{}, err
	}
	if id == "XTKS" {
		if c.fail.Load() {
			return clock.Session{}, errors.New("session read failed")
		}
		if day > c.last {
			return clock.Session{}, kindErr{contracts.UnknownCalendar}
		}
	}
	wd := tm.Weekday()
	open, closeAt := tm.Add(14*time.Hour), tm.Add(21*time.Hour)
	if id == "XTKS" {
		open, closeAt = tm, tm.Add(6*time.Hour+30*time.Minute)
	}
	return clock.Session{CalendarID: id, Day: day, IsSession: wd != time.Saturday && wd != time.Sunday, OpenAt: open, CloseAt: closeAt}, nil
}

func (c *jpCal) NextSession(context.Context, string, contracts.Day) (clock.Session, error) {
	return clock.Session{}, errors.New("not used")
}

// fwClock reads a fake clockwork clock as a clock.Clock.
type fwClock struct{ fw clockwork.Clock }

func (c fwClock) Now() time.Time { return c.fw.Now() }

func withCal(r *rig, c clock.Calendar) {
	r.a.deps.newCalendar = func(clock.SessionSource) clock.Calendar { return c }
}

func TestJPJobWiredAfterUS(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	withCal(r, &jpCal{last: jpLastDay})
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"fred_series", "option_eod", "option_eod_jp", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}
	if got := jobIDs(r.jobs); !slices.Equal(got, want) {
		t.Fatalf("jobs %v, want %v", got, want)
	}
}

func TestJPJobLeftOutWithoutXTKS(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	c := &jpCal{last: jpLastDay}
	c.fail.Store(true)
	withCal(r, c)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}
	if got := jobIDs(r.jobs); !slices.Equal(got, want) {
		t.Fatalf("jobs %v, want %v", got, want)
	}
	log := r.log.String()
	if n := strings.Count(log, noXTKSLine); n != 1 {
		t.Fatalf("%d lines say %q, want 1: %s", n, noXTKSLine, log)
	}
	if strings.Contains(log, r.a.getenv("OT_HOME")) {
		t.Fatalf("log names a path: %s", log)
	}
}

func TestJPJobDisabledWhenBrokerDown(t *testing.T) {
	t.Parallel()
	if got := startDisabled(runOnceConfig{fredKey: "k"}, false)[jobOptionEodJP]; got != reasonBrokerDown {
		t.Fatalf("startDisabled with the broker down lists option_eod_jp as %q", got)
	}
	if got, ok := startDisabled(runOnceConfig{fredKey: "k"}, true)[jobOptionEodJP]; ok {
		t.Fatalf("startDisabled with the broker up lists option_eod_jp as %q", got)
	}
	// The prober brings the job back once the broker answers and XTKS is loaded.
	r := newSchedRig(t, fullEnv)
	r.a.deps.newCalendar = func(clock.SessionSource) clock.Calendar { return &jpCal{last: jpLastDay} }
	r.a.deps.newClock = func() clock.Clock {
		return fwClock{clockwork.NewFakeClockAt(time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC))}
	}
	r.dialErrs = func(n int32) error {
		if n == 1 {
			return errors.New("down")
		}
		return nil
	}
	done := r.start()
	r.waitStarted(t, done)
	if got := r.sched.disabled[jobOptionEodJP]; got != reasonBrokerDown {
		t.Fatalf("disabled %v", r.sched.disabled)
	}
	r.tick()
	if _, off := r.sched.disabled[jobOptionEodJP]; off || !slices.Contains(r.lastBuild(), jobOptionEodJP) {
		t.Fatalf("disabled %v, last build %v", r.sched.disabled, r.lastBuild())
	}
	_ = r.stop(t, done)
}

func TestJPJobSlotAtTokyoAfterClose(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	cal := &jpCal{last: jpLastDay}
	withCal(r, cal)
	r.a.deps.newOptionEodJP = optioneod.NewJP
	r.a.deps.newOptionEodJPWatch = nil
	r.a.deps.newClock = func() clock.Clock { return fakeClock{time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)} }
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	var slots []runner.Slot
	for _, j := range r.jobs {
		if j.ID() != jobOptionEodJP {
			continue
		}
		cid, anchor, zone, off := j.(runner.Scheduled).Schedule()
		slots = append(slots, runner.Slot{JobID: j.ID(), CalendarID: cid, Anchor: anchor, Zone: zone, Offset: off})
	}
	if len(slots) != 1 {
		t.Fatalf("jobs %v", jobIDs(r.jobs))
	}
	from := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	starts, err := sched.NextStarts(t.Context(), cal, slots, from, 2)
	if err != nil || len(starts) == 0 {
		t.Fatalf("starts %v, err %v", starts, err)
	}
	if want := time.Date(2026, 11, 9, 6, 50, 0, 0, time.UTC); !starts[0].At.Equal(want) || starts[0].JobID != jobOptionEodJP {
		t.Fatalf("first start %+v, want %s", starts[0], want)
	}
}

func TestRuleStaysEmptyWithJPJob(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	withCal(r, &jpCal{last: jpLastDay})
	r.rule = runner.WindowRule{CalendarID: "unset"}
	r.backRule = runner.WindowRule{CalendarID: "unset"}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if err := backfill(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(jobIDs(r.jobs), jobOptionEodJP) {
		t.Fatalf("jobs %v", jobIDs(r.jobs))
	}
	if r.rule != (runner.WindowRule{}) || r.backRule != (runner.WindowRule{}) {
		t.Fatalf("rules %+v %+v", r.rule, r.backRule)
	}
}

// slotJob is a job with a plain schedule.
type slotJob struct {
	contracts.Job
	id, cal, anchor, zone string
	off                   time.Duration
}

func (j slotJob) ID() string { return j.id }
func (j slotJob) Schedule() (string, string, string, time.Duration) {
	return j.cal, j.anchor, j.zone, j.off
}

// cursorLedger is an OpsLedger with an empty cursor.
type cursorLedger struct{ store.OpsLedger }

func (cursorLedger) Cursor(context.Context) (store.Cursor, error) { return store.Cursor{}, nil }
func (cursorLedger) Close() error                                 { return nil }

// putLedger records the job states the ticker writes and signals each write.
type putLedger struct {
	store.SchedulerLedger
	mu     sync.Mutex
	states []store.JobState
	puts   chan struct{}
}

func (l *putLedger) Close() error                           { return nil }
func (l *putLedger) SetState(context.Context, string) error { return nil }
func (l *putLedger) PutJobStates(_ context.Context, s []store.JobState) error {
	l.mu.Lock()
	l.states = append([]store.JobState(nil), s...)
	l.mu.Unlock()
	select {
	case l.puts <- struct{}{}:
	default:
	}
	return nil
}

func (l *putLedger) state(id string) store.JobState {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.states {
		if s.JobID == id {
			return s
		}
	}
	return store.JobState{}
}

// TestJPSlotDroppedWhenXTKSRunsOut runs the real ticker. It is the one test of the package that sets the
// default logger, which the ticker logs through, so it does not run in parallel.
func TestJPSlotDroppedWhenXTKSRunsOut(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := newSchedRig(t, fullEnv)
	fw := clockwork.NewFakeClockAt(time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC))
	cal := &jpCal{last: jpLastDay}
	d := &r.a.deps
	d.newCalendar = func(clock.SessionSource) clock.Calendar { return cal }
	d.newClock = func() clock.Clock { return fwClock{fw} }
	d.newFwClock = func() clockwork.Clock { return fw }
	d.newScheduler = sched.New
	d.openLedger = func(context.Context, string) (store.OpsLedger, error) { return cursorLedger{}, nil }
	pl := &putLedger{puts: make(chan struct{}, 64)}
	d.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return pl, nil }
	us := func(id string) slotJob { return slotJob{id: id, cal: "XNYS", anchor: "session_close"} }
	d.newFredSeries = func(_ sources.Source, _ store.SeriesWriter, _ clock.Clock) (contracts.Job, error) {
		return us(jobFredSeries), nil
	}
	d.newOptionEod = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock, symbol.Translator) (contracts.Job, error) {
		return us(jobOptionEod), nil
	}
	d.newOptionEodJP = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
		return slotJob{id: jobOptionEodJP, cal: "XTKS", anchor: "time", zone: "Asia/Tokyo", off: 15*time.Hour + 50*time.Minute}, nil
	}
	d.newOptionEodJPWatch = nil
	d.newQuoteSnapshot = func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
		return us(jobQuoteSnapshot), nil
	}
	d.newComputeFigures = func(
		computefigures.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
	) (contracts.Job, error) {
		return us(jobComputeFigures), nil
	}
	d.newCalendarRefresh = func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, calendarrefresh.CalendarObserver) (contracts.Job, error) {
		return us(jobCalendarRefresh), nil
	}
	done := r.start()
	// the first refresh, before XTKS runs out: JP is scheduled at 15:50 JST of the 9th
	<-pl.puts
	if jp := pl.state(jobOptionEodJP); !jp.NextStart.Equal(time.Date(2026, 11, 9, 6, 50, 0, 0, time.UTC)) {
		t.Fatalf("first start of option_eod_jp %v", jp.NextStart)
	}
	if strings.Contains(buf.String(), "XTKS") {
		t.Fatalf("XTKS logged before it ran out: %s", buf.String())
	}
	// the clock passes the last XTKS day while the scheduler runs; the next tick walks the slots again
	if err := fw.BlockUntilContext(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	fw.Advance(97 * time.Hour)
	<-pl.puts
	if err := r.stop(t, done); err != nil {
		t.Fatalf("scheduler: %v", err)
	}
	now := fw.Now()
	for _, id := range []string{jobFredSeries, jobOptionEod, jobQuoteSnapshot, jobComputeFigures, jobCalendarRefresh} {
		if st := pl.state(id); !st.NextStart.After(now) {
			t.Errorf("%s has no start after %s: %v", id, now, st.NextStart)
		}
	}
	if st := pl.state(jobOptionEodJP); !st.NextStart.IsZero() {
		t.Errorf("option_eod_jp starts at %v after XTKS ran out", st.NextStart)
	}
	if n := strings.Count(buf.String(), "XTKS"); n != 1 {
		t.Fatalf("%d log lines name XTKS, want 1: %s", n, buf.String())
	}
}

// jpSwapRig is a scheduler rig with a fake XTKS calendar, a switch for the broker and a fake clock before
// 2026-11-12.
type jpSwapRig struct {
	*schedRig
	cal        *jpCal
	brokerDown atomic.Bool
}

func newJPSwapRig(t *testing.T, brokerDownAtStart bool) *jpSwapRig {
	t.Helper()
	r := &jpSwapRig{schedRig: newSchedRig(t, fullEnv), cal: &jpCal{last: jpLastDay}}
	fw := clockwork.NewFakeClockAt(time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC))
	r.a.deps.newCalendar = func(clock.SessionSource) clock.Calendar { return r.cal }
	r.a.deps.newClock = func() clock.Clock { return fwClock{fw} }
	r.brokerDown.Store(brokerDownAtStart)
	r.dialErrs = func(int32) error {
		if r.brokerDown.Load() {
			return errors.New("down")
		}
		return nil
	}
	return r
}

func without(js []string, id string) []string {
	return slices.DeleteFunc(slices.Clone(js), func(s string) bool { return s == id })
}

func TestJPProbeKeepsJobOutWhenXTKSRunsOut(t *testing.T) {
	t.Parallel()
	r := newJPSwapRig(t, true)
	done := r.start()
	r.waitStarted(t, done)
	if got := r.sched.disabled[jobOptionEodJP]; got != reasonBrokerDown {
		t.Fatalf("disabled %v", r.sched.disabled)
	}
	// XTKS runs out and the broker comes back
	r.cal.fail.Store(true)
	r.brokerDown.Store(false)
	r.tick()
	if got := r.sched.disabled[jobOptionEodJP]; got != noXTKSLine {
		t.Fatalf("option_eod_jp is left out with %q, want %q", got, noXTKSLine)
	}
	for i := 0; i < 2; i++ {
		if err := r.sched.rec.Reconcile(t.Context(), "2026-11-09"); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
		r.mu.Lock()
		got := slices.Clone(r.reconned)
		r.mu.Unlock()
		want := []string{jobCalendarRefresh, jobComputeFigures, jobEarningsRefresh, jobFredSeries, jobOptionEod, jobQuoteSnapshot}
		if !slices.Equal(got, want) {
			t.Fatalf("reconcile %d ran %v, want %v", i, got, want)
		}
	}
	_ = r.stop(t, done)
}

func TestJPRebuildDropsJobWhenXTKSRunsOut(t *testing.T) {
	t.Parallel()
	r := newJPSwapRig(t, true)
	// quote_snapshot cannot be built at the first probe, so it stays out until the second
	var quoteBuilds atomic.Int32
	r.a.deps.newQuoteSnapshot = func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
		if n := quoteBuilds.Add(1); n == 2 {
			return nil, errors.New("not yet")
		}
		return idJob{id: jobQuoteSnapshot}, nil
	}
	done := r.start()
	r.waitStarted(t, done)
	r.brokerDown.Store(false)
	r.tick() // option_eod and option_eod_jp come back; quote_snapshot does not
	if got := r.lastBuild(); !slices.Contains(got, jobOptionEodJP) || slices.Contains(got, jobQuoteSnapshot) {
		t.Fatalf("after the first tick: %v", got)
	}
	r.cal.fail.Store(true)
	r.tick() // quote_snapshot comes back, the reconciler is rebuilt
	want := []string{jobCalendarRefresh, jobComputeFigures, jobEarningsRefresh, jobFredSeries, jobOptionEod, jobQuoteSnapshot}
	if got := r.lastBuild(); !slices.Equal(got, want) {
		t.Fatalf("rebuilt with %v, want %v (without option_eod_jp: %v)", got, want, without(r.lastBuild(), jobOptionEodJP))
	}
	for i := 0; i < 2; i++ {
		if err := r.sched.rec.Reconcile(t.Context(), "2026-11-09"); err != nil {
			t.Fatalf("reconcile %d: %v", i, err)
		}
		r.mu.Lock()
		got := slices.Clone(r.reconned)
		r.mu.Unlock()
		if !slices.Equal(got, want) {
			t.Fatalf("reconcile %d ran %v, want %v", i, got, want)
		}
	}
	if n := strings.Count(r.errb.String(), noXTKSLine); n != 1 {
		t.Fatalf("%d log lines name XTKS, want 1: %s", n, r.errb.String())
	}
	_ = r.stop(t, done)
}
