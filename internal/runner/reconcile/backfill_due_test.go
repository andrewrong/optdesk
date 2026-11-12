package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

func dueJob(keys ...contracts.ExpectedKey) scheduledJob {
	return scheduledJob{fakeJob: &fakeJob{id: "open", keys: keys}, calendarID: "XNYS", anchor: "session_open", offset: -45 * time.Minute}
}

func dueRun(t *testing.T, now string, opt runner.BackfillOptions, l *spyLedger, job contracts.Job) runner.BackfillResult {
	t.Helper()
	res, err := forceRec(l, clockwork.NewFakeClockAt(utc(now)), job).BackfillWith(context.Background(), bfDay, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestBeforeSlotKeyRuns(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	job := dueJob(keyA)
	res := dueRun(t, "2026-11-04T13:00:00Z", runner.BackfillOptions{}, l, job)
	if line := lineOf(t, res, keyA); line.Group != runner.GroupRan || len(job.calls) != 1 {
		t.Fatalf("line %+v calls %v", line, job.calls)
	}
	l = bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	job = dueJob(keyA)
	res = dueRun(t, "2026-11-04T13:00:00Z", runner.BackfillOptions{Force: true}, l, job)
	if line := lineOf(t, res, keyA); line.Group != runner.GroupRerun || len(job.calls) != 1 {
		t.Fatalf("forced line %+v calls %v", line, job.calls)
	}
}

func TestTodayAtSlotRuns(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	job := dueJob(keyA)
	res := dueRun(t, "2026-11-04T13:45:00Z", runner.BackfillOptions{}, l, job) // inside the window, from the slot on
	if line := lineOf(t, res, keyA); line.Group != runner.GroupRan || len(job.calls) != 1 {
		t.Fatalf("line %+v calls %v", line, job.calls)
	}
}

func TestPastDayNeverHeld(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	job := dueJob(keyA)
	res := dueRun(t, "2026-11-06T00:00:00Z", runner.BackfillOptions{}, l, job)
	if line := lineOf(t, res, keyA); line.Group != runner.GroupRan || len(job.calls) != 1 {
		t.Fatalf("line %+v", line)
	}
}

func TestReconcileHoldsBeforeSlot(t *testing.T) {
	t.Parallel()
	for _, now := range []string{"2026-11-04T00:00:00Z", "2026-11-04T13:44:59Z", "2026-11-04T13:45:00Z", "2026-11-04T20:00:00Z", "2026-11-06T00:00:00Z"} {
		t.Run(now, func(t *testing.T) {
			t.Parallel()
			l := bfSetup(bfDay, keyA, keyC)
			held, plain := dueJob(keyA), &fakeJob{id: "plain", keys: []contracts.ExpectedKey{keyC}}
			clk := clockwork.NewFakeClockAt(utc(now))
			rec := forceRec(l, clk, held, plain)
			sess, err := rec.sessionsOf(context.Background(), bfDay)
			if err != nil {
				t.Fatal(err)
			}
			p, err := rec.plan(context.Background(), bfDay, sess)
			if err != nil {
				t.Fatal(err)
			}
			res, err := rec.BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, k := range []contracts.ExpectedKey{keyA, keyC} {
				if g := lineOf(t, res, k).Group; g != runner.GroupRan {
					t.Fatalf("%v: BackfillWith line %v, want ran", k, g)
				}
				if want := now == "2026-11-04T00:00:00Z" || now == "2026-11-04T13:44:59Z"; k == keyA && p.held[k] != want {
					t.Fatalf("%v: Reconcile holds %v, want %v", k, p.held[k], want)
				}
			}
		})
	}
}

func TestJobWithoutScheduleIsNeverHeld(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res := dueRun(t, "2026-11-04T00:00:00Z", runner.BackfillOptions{}, l, job)
	if line := lineOf(t, res, keyA); line.Group != runner.GroupRan || !line.DueAt.IsZero() {
		t.Fatalf("line %+v", line)
	}
}
