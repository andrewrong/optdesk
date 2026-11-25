package reconcile

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

func TestBackfillDropsStaleStoredKey(t *testing.T) {
	t.Parallel()
	stale := contracts.ExpectedKey{DatasetID: "ds", SubjectKey: "stale"}
	l := bfSetup(bfDay, keyA, keyB, stale)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyB}}
	rec := forceRec(l, afterClose(), job)
	lg, buf := captureLog()
	rec.log = lg
	res, err := rec.BackfillWith(context.Background(), bfDay, []string{"ds"}, runner.BackfillOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 2 || job.calls[0] != keyA || job.calls[1] != keyB {
		t.Fatalf("calls = %v, want [%v %v]", job.calls, keyA, keyB)
	}
	if len(res.Filled) != 2 {
		t.Fatalf("res %+v", res)
	}
	if got := l.expected[bfDay]; len(got) != 3 || got[2] != stale {
		t.Fatalf("stored keys changed: %v", got)
	}
	out := buf.String()
	if !strings.Contains(out, string(bfDay)) || !strings.Contains(out, "dataset=ds") || !strings.Contains(out, "keys=1") {
		t.Fatalf("log = %q, want day, dataset and count", out)
	}
}

func TestBackfillStaleKeyOnlyJobFails(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	bad, _ := badAndGoodJobs()
	rec := forceRec(l, afterClose(), bad)
	_, err := rec.BackfillWith(context.Background(), bfDay, []string{"ds"}, runner.BackfillOptions{Force: true})
	if err == nil || !strings.Contains(err.Error(), "no job returns a key of dataset ds") {
		t.Fatalf("err = %v", err)
	}
}
