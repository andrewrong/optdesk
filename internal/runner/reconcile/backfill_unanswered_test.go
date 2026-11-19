package reconcile

import (
	"context"
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

func unKeys(ds string, n int) []contracts.ExpectedKey {
	out := make([]contracts.ExpectedKey, n)
	for i := range out {
		out[i] = contracts.ExpectedKey{DatasetID: ds, SubjectKey: fmt.Sprintf("s%d", i)}
	}
	return out
}

var (
	unRetry  = contracts.JobResult{Err: kindErr{kind: contracts.Retryable}}
	unUsable = contracts.JobResult{Returned: 1, Usable: 1}
)

func TestBackfillStopsJobAfterThreeRetryableKeys(t *testing.T) {
	t.Parallel()
	keys := unKeys("ds", 5)
	l := bfSetup(bfDay, keys...)
	results := map[contracts.ExpectedKey]contracts.JobResult{}
	for _, k := range keys {
		results[k] = unRetry
	}
	job := &fakeJob{keys: keys, results: results}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 3 {
		t.Fatalf("calls %v, want 3", job.calls)
	}
	for i, k := range keys {
		line := lineOf(t, res, k)
		if i < 3 && line.Group != runner.GroupRan {
			t.Fatalf("key %d line %+v", i, line)
		}
		if i >= 3 && (line.Group != runner.GroupNotReached || line.ReasonCode != "broker_silent") {
			t.Fatalf("key %d line %+v", i, line)
		}
	}
	if len(res.Missing) != 5 {
		t.Fatalf("missing %v", res.Missing)
	}
	if len(l.runs[bfDay]) != 3 || l.marks.Load() != 0 {
		t.Fatalf("run rows %d marks %d", len(l.runs[bfDay]), l.marks.Load())
	}
}

func TestBackfillRetryableCountResetsOnAnotherEnding(t *testing.T) {
	t.Parallel()
	keys := unKeys("ds", 6)
	l := bfSetup(bfDay, keys...)
	ends := []contracts.JobResult{unRetry, unRetry, unUsable, unRetry, unRetry, unUsable}
	results := map[contracts.ExpectedKey]contracts.JobResult{}
	for i, k := range keys {
		results[k] = ends[i]
	}
	job := &fakeJob{keys: keys, results: results}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 6 {
		t.Fatalf("calls %v, want 6", job.calls)
	}
	for _, line := range res.Keys {
		if line.Group == runner.GroupNotReached {
			t.Fatalf("line %+v", line)
		}
	}
}

func TestBackfillStopOfOneJobKeepsOtherJob(t *testing.T) {
	t.Parallel()
	a, b := unKeys("ds", 5), unKeys("ds2", 4)
	l := bfSetup(bfDay, append(append([]contracts.ExpectedKey{}, a...), b...)...)
	resA := map[contracts.ExpectedKey]contracts.JobResult{}
	for _, k := range a {
		resA[k] = unRetry
	}
	jobA := &fakeJob{id: "silent", keys: a, results: resA}
	jobB := &fakeJob{id: "fine", keys: b}
	res, err := forceRec(l, afterClose(), jobA, jobB).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobA.calls) != 3 || len(jobB.calls) != 4 {
		t.Fatalf("calls a %v b %v", jobA.calls, jobB.calls)
	}
	for i, k := range a {
		if line := lineOf(t, res, k); i >= 3 && (line.Group != runner.GroupNotReached || line.ReasonCode != "broker_silent") {
			t.Fatalf("a %d line %+v", i, line)
		}
	}
	for _, k := range b {
		if line := lineOf(t, res, k); line.Group != runner.GroupRan {
			t.Fatalf("b line %+v", line)
		}
	}
}

func TestBackfillForcedRerunCountsRetryable(t *testing.T) {
	t.Parallel()
	keys := unKeys("ds", 5)
	l := bfSetup(bfDay, keys...)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{}
	results := map[contracts.ExpectedKey]contracts.JobResult{}
	for _, k := range keys {
		l.present[bfDay][k] = store.StatusPresent
		results[k] = unRetry
	}
	job := &fakeJob{keys: keys, results: results}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 3 {
		t.Fatalf("calls %v, want 3", job.calls)
	}
	for i, k := range keys {
		line := lineOf(t, res, k)
		if i < 3 && line.Group != runner.GroupRerun {
			t.Fatalf("key %d line %+v", i, line)
		}
		if i >= 3 && (line.Group != runner.GroupNotReached || line.ReasonCode != "broker_silent" || line.Mark != "present") {
			t.Fatalf("key %d line %+v", i, line)
		}
	}
}
