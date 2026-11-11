package optionopen

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func planKeys(t *testing.T, build func(ResolveDeps, ResolveOptions) (contracts.Job, error)) []contracts.ExpectedKey {
	t.Helper()
	r := newRig()
	j, err := build(r.deps("AAPL", "MSFT"), ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %v", keys)
	}
	return keys
}

func TestGapAndResolvePlanKeysDoNotCollide(t *testing.T) {
	t.Parallel()
	resolve := planKeys(t, NewResolve)
	gap := planKeys(t, NewGap)
	for _, a := range resolve {
		for _, b := range gap {
			if a == b {
				t.Fatalf("key %v is planned by both jobs", a)
			}
		}
	}
}

func TestGapKeysSurviveTheReconcilerMerge(t *testing.T) {
	t.Parallel()
	merged := map[contracts.ExpectedKey]bool{}
	for _, build := range []func(ResolveDeps, ResolveOptions) (contracts.Job, error){NewResolve, NewGap} {
		for _, k := range planKeys(t, build) {
			if _, dup := merged[k]; !dup {
				merged[k] = true
			}
		}
	}
	if len(merged) != 4 {
		t.Fatalf("want 4 distinct keys, got %v", merged)
	}
	n := map[string]int{}
	for k := range merged {
		n[k.DatasetID]++
	}
	if n["master.option_contract.gap"] != 2 || n["master.option_contract"] != 2 {
		t.Fatalf("keys by dataset: %v", n)
	}
}

func TestGapRunFinishesItsOwnKeys(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	res := r.run(t, NewGap, ResolveOptions{}, "AAPL")
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(r.logs) != 1 || !strings.HasPrefix(r.logs[0], "option_open_resolve_gap day=") {
		t.Fatalf("logs: %q", r.logs)
	}
}
