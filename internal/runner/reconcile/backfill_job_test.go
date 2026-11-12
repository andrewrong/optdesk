package reconcile

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/shared"
	"github.com/andrewrong/optdesk/internal/store"
)

// siblings are two jobs whose keys share the dataset "ds".
func siblings() (a, b *fakeJob) {
	return &fakeJob{id: "a", keys: []contracts.ExpectedKey{keyA}}, &fakeJob{id: "b", keys: []contracts.ExpectedKey{keyB}}
}

func TestJobFilterSkipsKeysOfSiblingOnSharedDataset(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyB)
	a, b := siblings()
	res, err := forceRec(l, afterClose(), a, b).BackfillWith(context.Background(), bfDay, []string{"ds"}, runner.BackfillOptions{Job: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.calls) != 1 || len(b.calls) != 0 || len(res.Keys) != 1 || res.Keys[0].Key != keyA {
		t.Fatalf("calls a %v b %v lines %+v", a.calls, b.calls, res.Keys)
	}
	if _, marked := l.present[bfDay][keyB]; marked {
		t.Fatal("the sibling's key was marked")
	}
}

func TestJobFilterEmptyKeepsEveryJob(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyB)
	a, b := siblings()
	res, err := forceRec(l, afterClose(), a, b).BackfillWith(context.Background(), bfDay, []string{"ds"}, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.calls) != 1 || len(b.calls) != 1 || len(res.Keys) != 2 {
		t.Fatalf("calls a %v b %v lines %+v", a.calls, b.calls, res.Keys)
	}
}

func TestDependentRunsWhenLateUpstreamEnds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup(bfDay, keyA, keyC)
	up, d := upJob(keyA), dep(0, keyC)
	rec := forceRec(l, afterClose(), up, d)
	// The slot has passed and the upstream ends ok later: the dependent runs then, once.
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 1 || len(d.calls) != 1 {
		t.Fatalf("up %d dep %d", len(up.calls), len(d.calls))
	}
	if r := rowOf(t, l.fakeLedger, bfDay, keyC); r.ReasonCode == "upstream_missing" {
		t.Fatalf("row %+v", r)
	}
	// A forced re-run of the upstream does not run the dependent again.
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up", Force: true}); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 2 || len(d.calls) != 1 {
		t.Fatalf("after a forced re-run: up %d dep %d", len(up.calls), len(d.calls))
	}
}

func TestDependentRunsMarkedAfterFailedUpstreamEnd(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyC)
	up := upJob(keyA)
	up.results = map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: kindErr{kind: contracts.Permanent}}}
	d := dep(0, keyC)
	if _, err := forceRec(l, afterClose(), up, d).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 1 {
		t.Fatalf("dep %d", len(d.calls))
	}
	if r := rowOf(t, l.fakeLedger, bfDay, keyC); r.ReasonCode != "upstream_missing" {
		t.Fatalf("row %+v", r)
	}
}

func TestUpstreamTriggersDependents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// A dependent whose slot has not passed is left to its slot turn.
	l := bfSetup(bfDay, keyA, keyC)
	up, d := upJob(keyA), dep(0, keyC)
	early := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	if _, err := forceRec(l, early, up, d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 0 {
		t.Fatalf("dependent started before its slot: %d", len(d.calls))
	}

	// A forced bare re-run of the upstream writes no row, and triggers the dependent when it returns ok.
	l = bfSetup(bfDay, keyA, keyC)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	up, d = upJob(keyA), dep(0, keyC)
	if _, err := forceRec(l, afterClose(), up, d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up", Force: true}); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 1 || len(l.runs[bfDay]) != 1 {
		t.Fatalf("dep %d runs %d", len(d.calls), len(l.runs[bfDay]))
	}

	// A running dependent is skipped, and the call does not wait for it.
	l = bfSetup(bfDay, keyA, keyC)
	up, d = upJob(keyA), dep(0, keyC)
	locks := shared.NewJobLocks()
	rec := forceRec(l, afterClose(), up, d)
	WithJobLocks(rec, locks)
	unlock, _ := locks.TryLock("dep")
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	unlock()
	if len(up.calls) != 1 || len(d.calls) != 0 {
		t.Fatalf("up %d dep %d", len(up.calls), len(d.calls))
	}

	// The chain follows: the dependent's own dependent runs when it ends, before the call returns.
	l = bfSetup(bfDay, keyA, keyC, keyB)
	up, d = upJob(keyA), dep(0, keyC)
	last := depJob{
		scheduledJob: scheduledJob{fakeJob: &fakeJob{id: "last", keys: []contracts.ExpectedKey{keyB}}, calendarID: "XNYS", anchor: "session_close"},
		after:        []string{"dep"},
	}
	if _, err := forceRec(l, afterClose(), up, d, last).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(last.calls) != 1 {
		t.Fatalf("last %d", len(last.calls))
	}
}

func TestUpstreamMissingMark(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, force := range []bool{false, true} {
		l := bfSetup(bfDay, keyA, keyC)
		up, d := upJob(keyA), dep(0, keyC)
		if force {
			l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyC: store.StatusPresent}
		}
		res, err := forceRec(l, afterClose(), up, d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "dep", Force: force})
		if err != nil {
			t.Fatal(err)
		}
		line := lineOf(t, res, keyC)
		if line.ReasonCode != "upstream_missing" || len(d.calls) != 1 || len(up.calls) != 0 {
			t.Fatalf("force %v: line %+v dep %d up %d", force, line, len(d.calls), len(up.calls))
		}
		if !force {
			if line.Group != runner.GroupRan {
				t.Fatalf("line %+v", line)
			}
			if r := rowOf(t, l.fakeLedger, bfDay, keyC); r.ReasonCode != "upstream_missing" {
				t.Fatalf("row %+v: the mark replaces the backfill reason", r)
			}
		}
	}

	// An upstream with an ok run for the day leaves no mark.
	l := bfSetup(bfDay, keyA, keyC)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	d := dep(0, keyC)
	res, err := forceRec(l, afterClose(), upJob(keyA), d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "dep"})
	if err != nil {
		t.Fatal(err)
	}
	if line := lineOf(t, res, keyC); line.ReasonCode == "upstream_missing" {
		t.Fatalf("line %+v", line)
	}
}
