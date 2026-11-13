package main

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

// init adds ot strategy-compare to the exclusions of the leak table (env_test.go) from this file.
func init() {
	leakExcluded["strategy-compare"] = "the leak deps hold no candidate or results opener; " +
		"its open errors are fixed texts that name the key and TestStrategyCompareOpenErrorWithheld covers them"
}

// compareRig is an app whose two readers hold the given rows for 2026-11-05.
func compareRig(t *testing.T, old []store.Candidate, res *fakeResultStore) (*app, *bytes.Buffer, *atomic.Int32) {
	t.Helper()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	var out bytes.Buffer
	a.stdout = &out
	var closed atomic.Int32
	res.closed = &closed
	a.deps.openCandidate = func(_ context.Context, dsn string) (candidateStore, error) {
		if dsn != "pages-dsn" {
			t.Errorf("dsn %q", dsn)
		}
		return &fakeCandidateRW{closed: &closed, rows: old}, nil
	}
	a.deps.openStrategyResults = func(_ context.Context, dsn string) (resultStore, error) {
		if dsn != "pages-dsn" {
			t.Errorf("dsn %q", dsn)
		}
		return res, nil
	}
	return a, &out, &closed
}

// oldRowAndResult is one short put row and the result row that rebuilds it.
func oldRowAndResult() (store.Candidate, strategyresult.Row) {
	yield := "0.12"
	old := store.Candidate{SessionDay: "2026-11-05", RunID: "r1", Kind: "short_put", Issuer: "cik:0000320193", InstrumentID: 11,
		Expiry: "2026-12-18", Strike: "95", Status: "ok", Marks: "[]", Gates: "[]", AnnualizedYield: &yield}
	row := strategyresult.Row{SessionDay: "2026-11-05", RunID: "r1", Strategy: "short_put", Key: "11", UnderlyingID: 1, Status: "ok",
		Legs: "[]", Checks: "[]", Facts: "{}"}
	return old, row
}

func TestStrategyCompareEqualPrintsEqual(t *testing.T) {
	t.Parallel()
	a, out, closed := compareRig(t, nil, &fakeResultStore{})
	if err := strategyCompareCmd(a, []string{"--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "equal rows=0" {
		t.Fatalf("output %q", got)
	}
	if closed.Load() != 2 {
		t.Fatalf("closed %d, want both stores", closed.Load())
	}
}

func TestStrategyCompareDifferenceExitsNonZero(t *testing.T) {
	t.Parallel()
	old, _ := oldRowAndResult()
	a, out, _ := compareRig(t, []store.Candidate{old}, &fakeResultStore{})
	err := strategyCompareCmd(a, []string{"--day", "2026-11-05"})
	if err == nil {
		t.Fatal("a difference must be an error")
	}
	if !strings.Contains(out.String(), "only in the old rows") || strings.Contains(out.String(), "equal") {
		t.Fatalf("output %q", out.String())
	}
}

func TestStrategyCompareDemoRefused(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	err := strategyCompareCmd(a, nil)
	if err == nil || !strings.Contains(err.Error(), "demo profile") {
		t.Fatalf("error %v", err)
	}
}

func TestStrategyCompareOpenErrorWithheld(t *testing.T) {
	t.Parallel()
	a, _, _ := compareRig(t, nil, &fakeResultStore{})
	a.deps.openStrategyResults = func(context.Context, string) (resultStore, error) {
		return nil, context.DeadlineExceeded
	}
	err := strategyCompareCmd(a, nil)
	if err == nil || strings.Contains(err.Error(), "pages-dsn") || !strings.Contains(err.Error(), pagesDSNName) {
		t.Fatalf("error %v", err)
	}
}
