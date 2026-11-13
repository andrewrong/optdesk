package main

import (
	"context"
	"github.com/jonboulle/clockwork"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

type marksJob struct{ fakeJob }

func (marksJob) ID() string { return "position_marks" }

type fakeMarkStore struct {
	mark.Store
	n *atomic.Int32
}

func (f fakeMarkStore) Close() error { f.n.Add(1); return nil }

type markRig struct {
	*acctRig
	markDSN string
	market  broker.Broker
	aliases []string
	built   atomic.Int32
	closed  atomic.Int32
	acct    store.AccountReader
	ids     []string
}

func newMarkRig(t *testing.T, withDSN bool) *markRig {
	t.Helper()
	r := &markRig{acctRig: newAcctRig(t, gwMainB, "c=/s/c.sock,a=/s/a.sock,b=/s/b.sock")}
	d := &r.a.deps
	read := d.readEnv
	if read == nil {
		read = defaultReadEnv
	}
	d.readEnv = func(path string) (map[string]string, error) {
		m, err := read(path)
		if err == nil && withDSN {
			m[mark.DSNEnvPositionMark] = "mark-dsn"
		}
		return m, err
	}
	d.openMark = func(_ context.Context, dsn string) (mark.Store, error) {
		r.markDSN = dsn
		return fakeMarkStore{n: &r.closed}, nil
	}
	d.newPositionMarks = func(m broker.Broker, al []string, acct store.AccountReader, _ mark.Writer, clk clock.Clock) (contracts.Job, error) {
		r.market, r.aliases, r.acct = m, al, acct
		r.built.Add(1)
		if clk == nil {
			t.Error("clock is nil")
		}
		return marksJob{}, nil
	}
	d.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return &fakeSchedLedger{}, nil }
	d.newKeyRunner = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyRunner { return nil }
	d.newReconciler = func(
		_ store.OpsLedger, _ store.SchedulerLedger, _ clock.Calendar, _ clockwork.Clock, js []contracts.Job, _ runner.WindowRule, _ runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		for _, j := range js {
			r.ids = append(r.ids, j.ID())
		}
		return recRunner{day: new(contracts.Day)}, nil
	}
	return r
}

func TestPositionMarksWiredInRunOnce(t *testing.T) {
	t.Parallel()
	r := newMarkRig(t, true)
	if err := runWired(r.a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if r.built.Load() != 1 || r.markDSN != "mark-dsn" || r.market == nil || r.acct == nil {
		t.Fatalf("built %d dsn %q", r.built.Load(), r.markDSN)
	}
	if want := []string{"b", "a", "c"}; !slices.Equal(r.aliases, want) {
		t.Fatalf("aliases %v, want %v (main first, then sorted)", r.aliases, want)
	}
	if !slices.Contains(r.ids, "position_marks") {
		t.Fatalf("jobs %v lack position_marks", r.ids)
	}
	if r.closed.Load() != 1 {
		t.Fatalf("mark store closed %d times", r.closed.Load())
	}
}

func TestMissingPosmarkLineLeavesOtherJobs(t *testing.T) {
	t.Parallel()
	r := newMarkRig(t, false)
	if err := runWired(r.a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatalf("run-once failed for all jobs: %v", err)
	}
	if slices.Contains(r.ids, "position_marks") || !slices.Contains(r.ids, "account_capture") {
		t.Fatalf("jobs %v", r.ids)
	}
	if r.built.Load() != 0 || r.markDSN != "" {
		t.Fatal("job built or store opened without its line")
	}
}
