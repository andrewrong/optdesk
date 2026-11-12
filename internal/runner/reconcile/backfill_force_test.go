package reconcile

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

var (
	keyD = contracts.ExpectedKey{DatasetID: "ds3", SubjectKey: "d"}
)

// forceRec is the reconciler over one calendar with the clock and the jobs given.
func forceRec(l store.OpsLedger, clk clockwork.Clock, jobs ...contracts.Job) *reconciler {
	return newReconciler(l, nil, oneDay(), clk, jobs, runner.WindowRule{CalendarID: "XNYS"}, nil)
}

func lineOf(t *testing.T, res runner.BackfillResult, k contracts.ExpectedKey) runner.KeyRun {
	t.Helper()
	for _, kr := range res.Keys {
		if kr.Key == k {
			return kr
		}
	}
	t.Fatalf("no line for %v in %+v", k, res.Keys)
	return runner.KeyRun{}
}

func TestForcedPresentKeyWritesNoRunRowAndNoMark(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	l.runs[bfDay] = []store.Run{{ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunDone}}
	runsBefore, _ := l.Runs(context.Background(), bfDay)
	markBefore, _ := l.Present(context.Background(), bfDay)
	recordsBefore, marksBefore := l.records.Load(), l.marks.Load()
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Returned: 4, Usable: 3}}}

	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	runsAfter, _ := l.Runs(context.Background(), bfDay)
	markAfter, _ := l.Present(context.Background(), bfDay)
	if !reflect.DeepEqual(runsBefore, runsAfter) || !reflect.DeepEqual(markBefore, markAfter) {
		t.Fatalf("rows or marks changed: %+v -> %+v, %v -> %v", runsBefore, runsAfter, markBefore, markAfter)
	}
	if l.records.Load() != recordsBefore || l.marks.Load() != marksBefore {
		t.Fatalf("writes: records %d marks %d", l.records.Load(), l.marks.Load())
	}
	line := lineOf(t, res, keyA)
	if line.Group != runner.GroupRerun || line.Returned != 4 || line.Usable != 3 || line.ErrorCode != "" || line.Mark != "present" {
		t.Fatalf("line %+v", line)
	}
	if len(job.calls) != 1 {
		t.Fatalf("job ran %d times", len(job.calls))
	}
}

func TestForcedPresentKeyDoesNotCountAsAttempt(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	l.runs[bfDay] = []store.Run{{ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunPartial}}
	rec := forceRec(l, afterClose(), &fakeJob{keys: []contracts.ExpectedKey{keyA}})
	runs, _ := l.Runs(context.Background(), bfDay)
	_, startBefore, _ := rec.nextAttempt(runs, time.Time{})
	if _, err := rec.BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	runs, _ = l.Runs(context.Background(), bfDay)
	_, startAfter, _ := rec.nextAttempt(runs, time.Time{})
	if len(runs) != 1 || startBefore != startAfter || !startAfter {
		t.Fatalf("runs %+v, start %v -> %v", runs, startBefore, startAfter)
	}
}

func TestBareRunNoValueReadsReasonNoValue(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: noValue{}}}}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	line := lineOf(t, res, keyA)
	if line.Group != runner.GroupRerun || line.ReasonCode != "no_value" || line.ErrorCode != "" {
		t.Fatalf("line %+v", line)
	}
}

func TestPresentKeyWithoutForceIsAlreadyPresent(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if lineOf(t, res, keyA).Group != runner.GroupAlreadyPresent || len(job.calls) != 0 || len(res.AlreadyPresent) != 1 {
		t.Fatalf("res %+v calls %v", res, job.calls)
	}
}

func TestMissingKeyTakesOrdinaryRunAndMark(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyC)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}, results: map[contracts.ExpectedKey]contracts.JobResult{
		keyC: {Err: noValue{}},
	}}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if l.present[bfDay][keyA] != store.StatusPresent || l.present[bfDay][keyC] != store.StatusNoValue {
		t.Fatalf("marks %v", l.present[bfDay])
	}
	for _, r := range l.runs[bfDay] {
		if r.ReasonCode != "backfill" || r.ErrorCode != "" {
			t.Fatalf("run %+v", r)
		}
	}
	a, c := lineOf(t, res, keyA), lineOf(t, res, keyC)
	if a.Group != runner.GroupRan || a.Mark != "present" || c.Mark != "no_value" || c.ReasonCode != "backfill" {
		t.Fatalf("lines %+v %+v", a, c)
	}
}

func TestMissingKeyBeforeCloseKeepsEmptyBeforeCloseRetry(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z")) // inside the window, which closes at 21:00
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {}}}
	rec := forceRec(l, clk, job)
	res, err := rec.BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	line := lineOf(t, res, keyA)
	if line.Group != runner.GroupRan || line.ReasonCode != "empty_before_close" || line.Mark != "" {
		t.Fatalf("line %+v", line)
	}
	if _, start, _ := rec.nextAttempt(l.runs[bfDay], utc("2026-11-04T21:00:00Z")); !start {
		t.Fatal("the scheduler does not retry the key")
	}
}

func TestCrashDuringMissingKeyRunIsAnOrdinaryCrash(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	clk := afterClose()
	l.runs[bfDay] = []store.Run{{
		ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunRunning, LeaseUntil: clk.Now().Add(-time.Minute),
	}}
	res, err := forceRec(l, clk, &fakeJob{keys: []contracts.ExpectedKey{keyA}}).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	runs := l.runs[bfDay]
	if len(runs) != 2 || runs[0].ReasonCode != "crashed" || runs[1].ReasonCode != "backfill" {
		t.Fatalf("runs %+v", runs)
	}
	if line := lineOf(t, res, keyA); line.Group != runner.GroupRan || line.Attempt != 2 {
		t.Fatalf("line %+v", line)
	}
}

func TestKeysCarryRunRowFactsAndMark(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyC)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}, results: map[contracts.ExpectedKey]contracts.JobResult{
		keyA: {Returned: 4, Usable: 3},
		keyC: {Err: kindErr{kind: contracts.Retryable}},
	}}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[contracts.ExpectedKey]runner.KeyRun{
		keyA: {Key: keyA, Group: "ran", Status: "done", ReasonCode: "backfill", Returned: 4, Usable: 3, Attempt: 1, Mark: "present"},
		keyC: {Key: keyC, Group: "ran", Status: "done", ErrorCode: "retryable", ReasonCode: "backfill", Attempt: 1},
	}
	for k, w := range want {
		if got := lineOf(t, res, k); got != w {
			t.Fatalf("line %+v, want %+v", got, w)
		}
	}
}

func TestLeaseHeldGroup(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	clk := afterClose()
	l.runs[bfDay] = []store.Run{{
		ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunRunning, LeaseUntil: clk.Now().Add(time.Minute),
	}}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err := forceRec(l, clk, job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if lineOf(t, res, keyA).Group != runner.GroupLeaseHeld || len(job.calls) != 0 || len(l.runs[bfDay]) != 1 {
		t.Fatalf("res %+v calls %v", res, job.calls)
	}
}

func TestZeroOptionsKeepSixLists(t *testing.T) {
	t.Parallel()
	keys := []contracts.ExpectedKey{keyA, keyB, keyC, keyD}
	l := bfSetup(bfDay, keys...)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	job := &fakeJob{keys: keys, results: map[contracts.ExpectedKey]contracts.JobResult{
		keyC: {Err: noValue{}},
		keyD: {Err: kindErr{kind: contracts.Retryable}},
	}}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	one := func(ks []contracts.ExpectedKey, k contracts.ExpectedKey) bool { return len(ks) == 1 && ks[0] == k }
	if !one(res.AlreadyPresent, keyA) || !one(res.Filled, keyB) || !one(res.Absent, keyC) || !one(res.Missing, keyD) || len(res.InWindow) != 0 {
		t.Fatalf("lists %+v", res)
	}
	if len(res.Keys) != 4 {
		t.Fatalf("lines %+v", res.Keys)
	}
}

func TestNoKeyDatasetIsRefusedNoKey(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	jobA := windowedJob{fakeJob: &fakeJob{id: "a", keys: []contracts.ExpectedKey{keyA}}, calendarID: "XNYS"}
	jobC := windowedJob{fakeJob: &fakeJob{id: "c", keys: []contracts.ExpectedKey{keyC}}, calendarID: "OTHER"} // a holiday there
	cal := onlyCal{id: "XNYS", fakeCalendar: oneDay()}
	rec := newReconciler(l, nil, cal, afterClose(), []contracts.Job{jobA, jobC}, runner.WindowRule{CalendarID: "XNYS"}, nil)
	res, err := rec.BackfillWith(context.Background(), bfDay, []string{"ds2"}, runner.BackfillOptions{})
	if err != nil || res.Refused != runner.RefusedNoKey || len(res.Keys) != 0 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if l.writes() != 0 || len(jobC.calls) != 0 {
		t.Fatalf("writes %d", l.writes())
	}
}
