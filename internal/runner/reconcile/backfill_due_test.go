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

// slotDue is the slot of dueJob on bfDay: the session open 14:30Z less 45 minutes.
var slotDue = utc("2026-11-04T13:45:00Z")

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

func TestTodayBeforeSlotIsNotDueWithDueAt(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	job := dueJob(keyA)
	res := dueRun(t, "2026-11-04T13:00:00Z", runner.BackfillOptions{}, l, job)
	line := lineOf(t, res, keyA)
	if line.Group != runner.GroupNotDue || !line.DueAt.Equal(slotDue) || len(job.calls) != 0 || len(l.runs[bfDay]) != 0 {
		t.Fatalf("line %+v calls %v", line, job.calls)
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

func TestNotDueAgreesWithReconcileHeldSet(t *testing.T) {
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
				if got := lineOf(t, res, k).Group == runner.GroupNotDue; got != p.held[k] {
					t.Fatalf("%v: BackfillWith holds %v, Reconcile holds %v", k, got, p.held[k])
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

func TestForcedPresentKeyBeforeSlotIsNotDueToo(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	job := dueJob(keyA)
	res := dueRun(t, "2026-11-04T13:00:00Z", runner.BackfillOptions{Force: true}, l, job)
	if line := lineOf(t, res, keyA); line.Group != runner.GroupNotDue || !line.DueAt.Equal(slotDue) || len(job.calls) != 0 {
		t.Fatalf("line %+v calls %v", line, job.calls)
	}
}

func TestQuoteSnapshotThreeKeysShareOneInstant(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyC, keyD)
	job := dueJob(keyA, keyC, keyD)
	res := dueRun(t, "2026-11-04T13:00:00Z", runner.BackfillOptions{}, l, job)
	if len(res.Keys) != 3 {
		t.Fatalf("lines %+v", res.Keys)
	}
	for _, line := range res.Keys {
		if line.Group != runner.GroupNotDue || !line.DueAt.Equal(slotDue) {
			t.Fatalf("line %+v", line)
		}
	}
}
