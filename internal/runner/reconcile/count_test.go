package reconcile

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

const countDay = contracts.Day("2026-11-04")

// outcome is what one call of a countJob does.
type outcome int

const (
	outFull      outcome = iota // usable rows
	outPartial                  // rows returned, then the run is cut (the Reconcile context is cancelled)
	outPermanent                // a Permanent error
	outNoValue                  // a NoValue error
	outRetryable                // a Retryable error
)

// countJob plans keyA and answers call n with script[n-1] (the last entry repeats).
type countJob struct {
	script []outcome
	mu     sync.Mutex
	n      int
	cancel context.CancelFunc
	// post, when set, makes the job a runner.Postcondition answering its value.
	post *bool
}

func (j *countJob) ID() string { return "count" }
func (j *countJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{keyA}, "plan-1", nil
}

func (j *countJob) Run(_ context.Context, _ contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.n++
	o := j.script[min(j.n, len(j.script))-1]
	switch o {
	case outPartial:
		j.cancel()
		return contracts.JobResult{Returned: 1}
	case outPermanent:
		return contracts.JobResult{Err: kindErr{kind: contracts.Permanent}}
	case outNoValue:
		return contracts.JobResult{Err: kindErr{kind: contracts.NoValue}}
	case outRetryable:
		return retryable
	case outFull:
	}
	return contracts.JobResult{Returned: 1, Usable: 1}
}

func (j *countJob) calls() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.n
}

// postJob is a countJob with a Postcondition.
type postJob struct{ *countJob }

func (j postJob) Postcondition(context.Context, string, string, string) (bool, error) {
	return *j.post, nil
}

var _ runner.Postcondition = postJob{}

// countEnv is one key on 2026-11-04 (window closes 21:00 UTC), the cursor at 2026-11-03.
type countEnv struct {
	t   *testing.T
	clk *clockwork.FakeClock
	led *fakeLedger
	job *countJob
	rec runner.Reconciler
}

func newCountEnv(t *testing.T, now string, job contracts.Job, cj *countJob) *countEnv {
	t.Helper()
	e := &countEnv{t: t, clk: clockwork.NewFakeClockAt(utc(now)), led: newLedger(), job: cj}
	e.led.cursor = store.Cursor{Day: "2026-11-03"}
	e.rec = NewWithRule(e.led, oneDay(), e.clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"})
	return e
}

func newPlainEnv(t *testing.T, now string, script ...outcome) *countEnv {
	cj := &countJob{script: script}
	return newCountEnv(t, now, cj, cj)
}

// tick runs one Reconcile; a partial outcome cancels its context, as a drain does.
func (e *countEnv) tick() {
	e.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.job.mu.Lock()
	e.job.cancel = cancel
	e.job.mu.Unlock()
	if err := e.rec.Reconcile(ctx, countDay); err != nil {
		e.t.Fatal(err)
	}
}

func (e *countEnv) goTo(at string) { e.clk.Advance(utc(at).Sub(e.clk.Now())) }

func (e *countEnv) seed(runs ...store.Run) {
	for i, r := range runs {
		r.Day, r.Key = countDay, keyA
		if r.ID == "" {
			r.ID = "2026-11-04|ds|a|" + string(rune('1'+i))
		}
		e.led.runs[countDay] = append(e.led.runs[countDay], r)
	}
}

func (e *countEnv) runs() []store.Run { return e.led.runs[countDay] }

func (e *countEnv) status() (store.PresentStatus, bool) {
	s, ok := e.led.present[countDay][keyA]
	return s, ok
}

func (e *countEnv) wantCursor(day contracts.Day) {
	e.t.Helper()
	if e.led.cursor.Day != day {
		e.t.Fatalf("cursor = %s, want %s", e.led.cursor.Day, day)
	}
}

func (e *countEnv) wantCalls(n int) {
	e.t.Helper()
	if e.job.calls() != n {
		e.t.Fatalf("calls = %d, want %d", e.job.calls(), n)
	}
}

func (e *countEnv) wantNoValue(reason string) {
	e.t.Helper()
	if s, ok := e.status(); !ok || s != store.StatusNoValue {
		e.t.Fatalf("key = %q (%v), want no_value", s, ok)
	}
	for _, r := range e.runs() {
		if r.ReasonCode == reason {
			return
		}
	}
	e.t.Fatalf("no run has reason %q: %+v", reason, e.runs())
}

func (e *countEnv) wantMissing() {
	e.t.Helper()
	if s, ok := e.status(); ok {
		e.t.Fatalf("key = %q, want missing", s)
	}
}

func crashedRun() store.Run {
	return store.Run{Status: store.RunDone, ErrorCode: string(contracts.Retryable), ReasonCode: reasonCrashed}
}

func abandonedRun() store.Run {
	return store.Run{Status: store.RunAbandoned, ReasonCode: reasonAbandonedStop}
}

func TestAbandonedRunNotCounted(t *testing.T) {
	t.Parallel()
	e := newPlainEnv(t, "2026-11-04T15:00:00Z", outPartial, outFull)
	e.seed(abandonedRun())
	e.tick() // the abandoned run is no counted run: this is the first, and it is cut
	e.wantMissing()
	e.tick() // a partial earns the second run
	e.wantCalls(2)
	if s, _ := e.status(); s != store.StatusPresent {
		t.Fatalf("key = %q, want present", s)
	}
	e.wantCursor(countDay)
}

func TestLapsedLeaseNotCounted(t *testing.T) {
	t.Parallel()
	lapsed := func(e *countEnv) store.Run {
		return store.Run{Status: store.RunRunning, LeaseUntil: e.clk.Now().Add(-time.Minute)}
	}

	t.Run("marked crashed and the key is run again", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T15:00:00Z", outPartial, outFull)
		e.seed(lapsed(e))
		e.tick()
		if r := e.runs()[0]; r.Status != store.RunDone || r.ReasonCode != reasonCrashed {
			t.Fatalf("first run = %+v, want done/crashed", r)
		}
		e.tick() // the crash did not use the allowance: the partial gets its second run
		e.wantCalls(2)
		if s, _ := e.status(); s != store.StatusPresent {
			t.Fatalf("key = %q, want present", s)
		}
	})

	t.Run("a third lapsed run stops the key", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T15:00:00Z", outFull)
		l := lapsed(e)
		e.seed(crashedRun(), crashedRun(), l)
		e.tick()
		e.wantCalls(0)
		if r := e.runs()[2]; r.Status != store.RunDone || r.ReasonCode != reasonCrashed {
			t.Fatalf("lapsed run = %+v, want marked crashed", r)
		}
		e.wantMissing()
		e.wantCursor("2026-11-03")
	})
}

func TestCrashedRepeatedlyStopsKey(t *testing.T) {
	t.Parallel()

	t.Run("two crashed runs are below the brake", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T22:00:00Z", outRetryable)
		e.seed(crashedRun(), crashedRun())
		e.tick()
		e.wantCalls(1)
		runs := e.runs()
		if len(runs) != 3 || runs[2].ReasonCode != reasonLate {
			t.Fatalf("runs = %+v, want a third late run", runs)
		}
		for _, r := range runs {
			if r.ReasonCode == reasonCrashedRepeatedly || r.ReasonCode == reasonExpired {
				t.Fatalf("run %s closed below the brake", r.ID)
			}
		}
	})

	t.Run("three crashed runs hold the cursor until the close", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T15:00:00Z", outFull)
		e.seed(crashedRun(), abandonedRun(), crashedRun())
		e.tick()
		e.wantCalls(0)
		e.wantMissing()
		e.wantCursor("2026-11-03")
		e.goTo("2026-11-04T22:00:00Z")
		e.tick()
		e.wantCalls(0)
		e.wantNoValue(reasonCrashedRepeatedly)
		if r := e.runs()[2]; r.ReasonCode != reasonCrashedRepeatedly {
			t.Fatalf("latest crashed run = %+v, want crashed_repeatedly", r)
		}
		if r := e.runs()[1]; r.ReasonCode != reasonAbandonedStop {
			t.Fatalf("earlier run = %+v, want untouched", r)
		}
		e.wantCursor(countDay)
	})

	t.Run("row 1 wins over row 2", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T22:00:00Z", outFull)
		e.seed(store.Run{Status: store.RunPartial, ReturnedN: 1}, crashedRun(), crashedRun(), crashedRun())
		e.tick()
		e.wantCalls(0)
		runs := e.runs()
		if runs[3].ReasonCode != reasonCrashedRepeatedly {
			t.Fatalf("latest crashed run = %+v, want crashed_repeatedly", runs[3])
		}
		if runs[0].Status != store.RunPartial || runs[0].ReasonCode != "" {
			t.Fatalf("partial run = %+v, want untouched", runs[0])
		}
		e.wantNoValue(reasonCrashedRepeatedly)
		e.wantCursor(countDay)
	})
}

func TestPartialRerunUntilClose(t *testing.T) {
	t.Parallel()

	t.Run("partial then full then no third run", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T15:00:00Z", outPartial, outFull)
		e.tick()
		e.wantMissing()
		e.wantCursor("2026-11-03")
		if r := e.runs()[0]; r.Status != store.RunPartial {
			t.Fatalf("first run = %+v, want partial", r)
		}
		e.tick()
		if s, _ := e.status(); s != store.StatusPresent {
			t.Fatalf("key = %q, want present", s)
		}
		e.tick()
		e.wantCalls(2)
		e.wantCursor(countDay)
	})

	for _, tc := range []struct {
		name string
		out  outcome
	}{{"permanent", outPermanent}, {"no value", outNoValue}} {
		t.Run(tc.name+" gets no second run", func(t *testing.T) {
			t.Parallel()
			e := newPlainEnv(t, "2026-11-04T15:00:00Z", tc.out)
			e.tick()
			e.tick()
			e.wantCalls(1)
		})
	}

	t.Run("every run cut: two partials, then the close", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T15:00:00Z", outPartial)
		e.tick()
		e.tick()
		e.tick()
		e.wantCalls(2)
		e.wantMissing()
		e.wantCursor("2026-11-03")
		e.goTo("2026-11-04T21:30:00Z")
		e.tick()
		e.wantCalls(2)
		e.wantNoValue(reasonExpired)
		if r := e.runs()[1]; r.Status != store.RunPartial || r.ReasonCode != reasonExpired {
			t.Fatalf("latest partial run = %+v, want partial with reason expired", r)
		}
		e.wantCursor(countDay)
	})
}

func TestCloseTableRows(t *testing.T) {
	t.Parallel()
	no := false

	t.Run("false postcondition is row 4", func(t *testing.T) {
		t.Parallel()
		cj := &countJob{script: []outcome{outFull}, post: &no}
		e := newCountEnv(t, "2026-11-04T15:00:00Z", postJob{cj}, cj)
		e.tick()
		e.tick()
		e.wantCalls(1)
		e.wantMissing()
		e.goTo("2026-11-04T22:00:00Z")
		e.tick()
		e.wantCalls(1)
		e.wantNoValue(reasonExpired)
		e.wantCursor(countDay)
	})

	t.Run("permanent error is row 4", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T15:00:00Z", outPermanent)
		e.tick()
		e.tick()
		e.wantCalls(1)
		e.wantMissing()
		e.goTo("2026-11-04T22:00:00Z")
		e.tick()
		e.wantCalls(1)
		e.wantNoValue(reasonExpired)
		e.wantCursor(countDay)
	})

	t.Run("no run yet gets a late run", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T22:00:00Z", outRetryable)
		e.tick()
		e.wantCalls(1)
		if runs := e.runs(); len(runs) != 1 || runs[0].ReasonCode != reasonLate {
			t.Fatalf("runs = %+v, want one late run", runs)
		}
		e.wantMissing()
	})

	t.Run("only empty_before_close runs get a late run", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T22:00:00Z", outRetryable)
		empty := store.Run{Status: store.RunDone, ReasonCode: reasonEmptyBeforeClose}
		e.seed(empty, empty)
		e.tick()
		e.wantCalls(1)
		runs := e.runs()
		if len(runs) != 3 || runs[2].ReasonCode != reasonLate || runs[0].ReasonCode != reasonEmptyBeforeClose {
			t.Fatalf("runs = %+v, want a third late run and the empty ones untouched", runs)
		}
		e.wantMissing()
	})

	t.Run("live running run is left alone", func(t *testing.T) {
		t.Parallel()
		e := newPlainEnv(t, "2026-11-04T22:00:00Z", outFull)
		e.seed(store.Run{Status: store.RunRunning, LeaseUntil: e.clk.Now().Add(time.Minute)})
		e.tick()
		e.wantCalls(0)
		if r := e.runs()[0]; r.Status != store.RunRunning || r.ReasonCode != "" {
			t.Fatalf("run = %+v, want untouched", r)
		}
		e.wantMissing()
		e.wantCursor("2026-11-03")
	})
}
