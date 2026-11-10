package main

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

// callsRig is a run-once rig with the account, candidate and events lines whose option_eod and option_candidates
// constructors record what they were given.
type callsRig struct {
	*rig
	heldAcct   store.AccountReader
	calls      atomic.Int32
	puts       atomic.Int32
	callsDiv   dividend.Reader
	acctOpened atomic.Int32
	closed     atomic.Int32
}

func newCallsRig(t *testing.T, env string) *callsRig {
	t.Helper()
	c := &callsRig{rig: newRig(t, env+candidateEnv+"OT_PG_DSN_ACCOUNT=acct-dsn\n")}
	d := &c.a.deps
	d.openAccount = func(context.Context, string) (store.AccountStore, error) {
		c.acctOpened.Add(1)
		return fakeAcctStore{n: &c.closed}, nil
	}
	d.openCandidateRW = func(context.Context, string) (candidateRW, error) { return &fakeCandidateRW{closed: &c.closed}, nil }
	d.newCandidateRule = func() compute.OptionCandidates { return fakeRule{} }
	d.newOptionEodHeld = func(
		_ broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter, _ []jobs.WatchEntry, acct store.AccountReader, _ clock.Clock,
	) (contracts.Job, error) {
		c.heldAcct = acct
		return namedJob{id: jobOptionEod}, nil
	}
	d.newOptionCandidates = func(jobs.CandidateStores, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error) {
		c.puts.Add(1)
		return candJobStub{id: jobOptionCandidates}, nil
	}
	d.newOptionCandidatesCalls = func(
		_ jobs.CandidateStores, div dividend.Reader, _ compute.OptionCandidates, _ []jobs.WatchEntry, _ clock.Clock,
	) (contracts.Job, error) {
		c.calls.Add(1)
		c.callsDiv = div
		return candJobStub{id: jobOptionCandidates}, nil
	}
	d.newReference = func(context.Context, string, broker.Options) (broker.ReferenceClient, error) { return &divRef{}, nil }
	d.openDividend = func(context.Context, string) (dividend.Store, error) { return &divStore{}, nil }
	d.newDividendCapture = func(
		broker.ReferenceClient, store.InstrumentResolver, dividend.Writer, dividend.Reader, []jobs.WatchEntry, clock.Clock,
	) (contracts.Job, error) {
		return namedJob{id: "dividend_capture"}, nil
	}
	return c
}

func TestOptionEodUsesHeldReader(t *testing.T) {
	t.Parallel()
	c := newCallsRig(t, fullEnv)
	if err := runOnce(c.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatalf("%v\n%s", err, c.log)
	}
	if c.heldAcct == nil {
		t.Fatal("option_eod was not built with the account reader")
	}
	if c.acctOpened.Load() != 1 {
		t.Fatalf("account store opened %d times, want once for option_eod and option_candidates", c.acctOpened.Load())
	}
	// Without the account line option_eod is the job of before.
	n := newCallsRig(t, fullEnv)
	n.a.deps.openAccount = nil
	n.a.deps.newOptionEodWatch = namedEodWatch(jobOptionEod)
	if err := runOnce(n.a, []string{"--day", "2026-11-04"}); err != nil || n.heldAcct != nil {
		t.Fatalf("err %v held %v", err, n.heldAcct)
	}
}

func TestCallsJobUsesDividendReader(t *testing.T) {
	t.Parallel()
	c := newCallsRig(t, fullEnv)
	if err := runOnce(c.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatalf("%v\n%s", err, c.log)
	}
	if c.calls.Load() != 1 || c.puts.Load() != 0 || c.callsDiv == nil {
		t.Fatalf("calls %d puts %d reader %v", c.calls.Load(), c.puts.Load(), c.callsDiv)
	}
	if !slices.Contains(jobIDs(c.jobs), jobOptionCandidates) {
		t.Fatalf("jobs %v", jobIDs(c.jobs))
	}
}

func TestMissingEventsDSNKeepsPutsJob(t *testing.T) {
	t.Parallel()
	c := newCallsRig(t, strings.Replace(fullEnv, "OT_PG_DSN_EVENTS=events-dsn\n", "", 1))
	if err := runOnce(c.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatalf("%v\n%s", err, c.log)
	}
	if c.puts.Load() != 1 || c.calls.Load() != 0 {
		t.Fatalf("puts %d calls %d", c.puts.Load(), c.calls.Load())
	}
	if !slices.Contains(jobIDs(c.jobs), jobOptionCandidates) {
		t.Fatalf("jobs %v", jobIDs(c.jobs))
	}
	if !strings.Contains(c.log.String(), reasonNoEventsDSN) || !strings.Contains(c.log.String(), jobOptionCandidates) {
		t.Fatalf("no named line: %s", c.log)
	}
}
