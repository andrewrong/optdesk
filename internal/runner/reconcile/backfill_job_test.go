package reconcile

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
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
