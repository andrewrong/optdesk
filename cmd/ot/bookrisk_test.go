package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/bookrisk"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

const bookRiskEnv = "OT_PG_DSN_CANDIDATE=cand-secret\nOT_PG_DSN_OPTION_LEDGER=led-secret\nOT_PG_DSN_ACCOUNT=acct-secret\n"

// bookRiskResults is the strategy.result store of the job; it records its close.
type bookRiskResults struct {
	strategyresult.Store
	closed *atomic.Int32
}

func (f bookRiskResults) Close() error { f.closed.Add(1); return nil }

// bookRiskRig is a run-once rig whose stores and whose constructor of book_risk are fakes.
type bookRiskRig struct {
	*rig
	stores    bookrisk.Stores
	built     atomic.Int32
	closed    atomic.Int32
	resClosed atomic.Int32
	dsns      []string
}

func newBookRiskRig(t *testing.T, env string) *bookRiskRig {
	t.Helper()
	b := &bookRiskRig{rig: newRig(t, env)}
	d := &b.a.deps
	d.openStrategyResults = func(_ context.Context, dsn string) (resultStore, error) {
		b.dsns = append(b.dsns, dsn)
		return bookRiskResults{closed: &b.resClosed}, nil
	}
	d.openOptionLedgerStore = func(_ context.Context, dsn string) (store.OptionLedgerStore, error) {
		b.dsns = append(b.dsns, dsn)
		return &ledgerFakeStore{}, nil
	}
	d.openAccount = func(_ context.Context, dsn string) (store.AccountStore, error) {
		b.dsns = append(b.dsns, dsn)
		return fakeAcctStore{n: &b.closed}, nil
	}
	d.newOptionLedger = func() compute.OptionLedger { return nil }
	d.newOptionStructures = func() compute.OptionStructures { return nil }
	d.ext = map[string]any{extBookRisk: bookRiskDeps{newJob: func(st bookrisk.Stores, _ clock.Clock) (contracts.Job, error) {
		b.stores = st
		b.built.Add(1)
		return plannedJob{namedJob{id: jobBookRisk}}, nil
	}}}
	return b
}

func (b *bookRiskRig) ids() []string {
	var ids []string
	for _, j := range b.jobs {
		ids = append(ids, j.ID())
	}
	return ids
}

func TestBookRiskNewIsRegisteredByRunOnce(t *testing.T) {
	t.Parallel()
	b := newBookRiskRig(t, fullEnv+bookRiskEnv)
	if err := runWired(b.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(b.ids(), jobBookRisk) {
		t.Fatalf("jobs %v hold no book_risk", b.ids())
	}
	st := b.stores
	if st.Stresser == nil || st.Fitter == nil || st.Results == nil || st.Ledger == nil || st.Account == nil || len(st.Aliases) == 0 {
		t.Errorf("stores %+v", st)
	}
	for _, want := range []string{"cand-secret", "led-secret", "acct-secret"} {
		if !slices.Contains(b.dsns, want) {
			t.Errorf("no store opened with %q: %v", want, b.dsns)
		}
	}
	if b.resClosed.Load() != 1 {
		t.Errorf("results store closed %d times, want 1", b.resClosed.Load())
	}
}

func TestBookRiskNewIsRegisteredForBackfill(t *testing.T) {
	t.Parallel()
	b := newBookRiskRig(t, fullEnv+bookRiskEnv)
	if err := backfillWired(b.a, []string{"--day", "2026-11-04", "--job", jobBookRisk}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(b.ids(), jobBookRisk) {
		t.Fatalf("jobs %v hold no book_risk", b.ids())
	}
}

func TestBookRiskNewDemoProfileRefused(t *testing.T) {
	t.Parallel()
	b := newBookRiskRig(t, fullEnv+bookRiskEnv)
	b.a.profile.Name = "demo"
	for name, err := range map[string]error{
		"run-once": runWired(b.a, []string{"--day", "2026-11-04"}),
		"backfill": backfillWired(b.a, []string{"--day", "2026-11-04", "--job", jobBookRisk}),
	} {
		if err == nil || !strings.Contains(err.Error(), "demo profile") {
			t.Errorf("%s: err %v, want the demo refusal", name, err)
		}
	}
	if b.built.Load() != 0 || len(b.dsns) != 0 {
		t.Errorf("the demo profile built the job (%d) or opened %v", b.built.Load(), b.dsns)
	}
}

func TestBookRiskNewMissingDSNNamesTheKeyOnly(t *testing.T) {
	t.Parallel()
	b := newBookRiskRig(t, fullEnv+"OT_PG_DSN_CANDIDATE=cand-secret\nOT_PG_DSN_ACCOUNT=acct-secret\n")
	if err := runWired(b.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(b.ids(), jobBookRisk) || b.built.Load() != 0 {
		t.Errorf("book_risk is wired without its ledger line: %v", b.ids())
	}
	log := b.log.String()
	if !strings.Contains(log, "OT_PG_DSN_OPTION_LEDGER is not set") {
		t.Errorf("no reason naming the key:\n%s", log)
	}
	if strings.Contains(log, "cand-secret") || strings.Contains(log, "acct-secret") {
		t.Errorf("a value is logged:\n%s", log)
	}

	// a failed open names the key, never the value
	b = newBookRiskRig(t, fullEnv+bookRiskEnv)
	b.a.deps.openStrategyResults = func(context.Context, string) (resultStore, error) {
		return nil, errors.New("boom cand-secret")
	}
	err := runWired(b.a, []string{"--day", "2026-11-04"})
	if err == nil || !strings.Contains(err.Error(), "OT_PG_DSN_CANDIDATE") || strings.Contains(err.Error(), "secret") {
		t.Errorf("open error %v", err)
	}
}

// plannedJob plans one key, so that backfill --job finds the job on the day.
type plannedJob struct{ namedJob }

func (plannedJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "strategy.result", SubjectKey: "book_risk"}}, "h", nil
}
