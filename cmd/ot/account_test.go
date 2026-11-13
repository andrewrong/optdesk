package main

import (
	"context"
	"errors"
	"github.com/jonboulle/clockwork"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts"
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// bookMainB is an accounts.toml of the aliases a and b whose main is b.
var bookMainB = bookText("b", "a", "b")

// fakeAcctReader answers with the id of the account behind its socket and counts its closes.
type fakeAcctReader struct {
	n  *atomic.Int32
	id string
}

func (f fakeAcctReader) Account(context.Context) (broker.Account, error) {
	return broker.Account{Alias: f.id}, nil
}
func (f fakeAcctReader) Executions(context.Context) (broker.Executions, error) {
	return broker.Executions{Alias: f.id}, nil
}
func (f fakeAcctReader) Close() error { f.n.Add(1); return nil }

type acctJob struct{ fakeJob }

func (acctJob) ID() string { return "account_capture" }

type fakeAcctStore struct {
	store.AccountStore
	n *atomic.Int32
}

func (f fakeAcctStore) Close() error { f.n.Add(1); return nil }

// Positions answers none: the held-name reader of the job list reads it.
func (fakeAcctStore) Positions(context.Context, string, string) ([]store.Position, error) {
	return nil, nil
}

// acctRig is run-once over fakes with the account lines in .env; got holds what the constructor received.
type acctRig struct {
	a              *app
	dialed         []string
	cfg            accountcapture.Config
	built          atomic.Int32
	closed, opened atomic.Int32
}

// newAcctRig is run-once over fakes; book is the body of accounts.toml ("" writes none), sockets the plain list of
// OT_ACCOUNT_SOCKETS ("" sets none). The socket /s/<x>.sock reports the id of the alias x.
func newAcctRig(t *testing.T, book, sockets string) *acctRig {
	t.Helper()
	env := fullEnv + "OT_PG_DSN_ACCOUNT=acct-dsn\n"
	if sockets != "" {
		env += "OT_ACCOUNT_SOCKETS=" + sockets + "\n"
	}
	r := &acctRig{}
	a, _ := testApp(t, env)
	if book != "" {
		writeBook(t, a, book)
	}
	var opened, closed atomic.Int32
	var day contracts.Day
	runOnceFakes(a, &opened, &closed, &day)
	d := &a.deps
	d.dialAccount = func(_ context.Context, sock string, _ broker.Options) (broker.AccountReader, error) {
		r.dialed = append(r.dialed, sock)
		alias := strings.TrimSuffix(strings.TrimPrefix(sock, "/s/"), ".sock")
		return fakeAcctReader{n: &r.closed, id: aliasID(alias)}, nil
	}
	d.openAccount = func(context.Context, string) (store.AccountStore, error) {
		r.opened.Add(1)
		return fakeAcctStore{n: &r.closed}, nil
	}
	d.newAccountCapture = func(c accountcapture.Config) (contracts.Job, error) {
		r.cfg = c
		r.built.Add(1)
		return acctJob{}, nil
	}
	r.a = a
	return r
}

func TestAccountCaptureDryRun(t *testing.T) {
	t.Parallel()
	r := newAcctRig(t, bookMainB, "/s/a.sock,/s/b.sock")
	var ids []string
	r.a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return &fakeSchedLedger{}, nil }
	r.a.deps.newKeyRunner = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyRunner { return nil }
	r.a.deps.newReconciler = func(
		_ store.OpsLedger, _ store.SchedulerLedger, _ clock.Calendar, _ clockwork.Clock, js []contracts.Job, _ runner.WindowRule, _ runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		for _, j := range js {
			ids = append(ids, j.ID())
		}
		return recRunner{day: new(contracts.Day)}, nil
	}
	if err := runOnce(r.a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if r.built.Load() != 1 || r.cfg.Main != "b" || len(r.cfg.Readers) != 2 || r.cfg.Writer == nil || r.cfg.Clock == nil {
		t.Fatalf("built %d cfg main %q readers %d", r.built.Load(), r.cfg.Main, len(r.cfg.Readers))
	}
	if strings.Join(r.dialed, ",") != "/s/a.sock,/s/b.sock" || r.opened.Load() != 2 {
		t.Fatalf("dialed %v opened %d", r.dialed, r.opened.Load())
	}
	// The plan printed by run-once reads the held names of the job list through a second account store.
	if r.closed.Load() != 4 {
		t.Fatalf("closed %d, want two readers and the two stores", r.closed.Load())
	}
	if !slices.Contains(ids, "account_capture") {
		t.Fatalf("jobs %v lack account_capture", ids)
	}
}

func assertRefused(t *testing.T, r *acctRig, key string) {
	t.Helper()
	err := runOnce(r.a, []string{"--day", "2026-03-04"})
	if err == nil || !strings.Contains(err.Error(), key) {
		t.Fatalf("err %v, want one naming %s", err, key)
	}
	if strings.Contains(err.Error(), "/") || r.built.Load() != 0 || len(r.dialed) != 0 || r.opened.Load() != 0 {
		t.Fatalf("err %q built %d dialed %v opened %d", err, r.built.Load(), r.dialed, r.opened.Load())
	}
}

func TestAccountRefusesMissingOrBadBook(t *testing.T) {
	t.Parallel()
	assertRefused(t, newAcctRig(t, "", "/s/a.sock"), accountsFileName)
	assertRefused(t, newAcctRig(t, "a = \"x\"\n", "/s/a.sock"), accountsFileName) // no main, bad id
}

func TestAccountRefusesLabelForm(t *testing.T) {
	t.Parallel()
	assertRefused(t, newAcctRig(t, bookMainB, "a=/s/a.sock,b=/s/b.sock"), "OT_ACCOUNT_SOCKETS")
}

func TestAccountBookReadThroughDeps(t *testing.T) {
	t.Parallel()
	r := newAcctRig(t, bookMainB, "")
	var reads atomic.Int32
	r.a.deps.readEnv = func(string) (map[string]string, error) {
		reads.Add(1)
		return map[string]string{"OT_ACCOUNT_SOCKETS": "/s/a.sock,/s/b.sock", "OT_PG_DSN_ACCOUNT": "x"}, nil
	}
	var extra closerSet
	j, err := r.a.accountCapture(context.Background(), jobFactory{d: r.a.deps}, &extra)
	if err != nil || j == nil || reads.Load() == 0 || r.cfg.Main != "b" {
		t.Fatalf("err %v reads %d main %q", err, reads.Load(), r.cfg.Main)
	}
}

func TestAccountRefusesUnsetSockets(t *testing.T) {
	t.Parallel()
	r := newAcctRig(t, bookMainB, "") // book present, OT_ACCOUNT_SOCKETS unset
	err := runOnce(r.a, []string{"--day", "2026-03-04"})
	if err == nil || !strings.Contains(err.Error(), "OT_ACCOUNT_SOCKETS") {
		t.Fatalf("err %v, want a refusal naming OT_ACCOUNT_SOCKETS", err)
	}
}

// The resolver tests use fake readers: socket 0 reports the id of a, socket 1 the id of b.

func TestResolverMatchGivesReaderOfEachAlias(t *testing.T) {
	t.Parallel()
	rs := resolverOf(t, bookMainB, idReader{id: aliasID("a")}, idReader{id: aliasID("b")})
	if len(rs) != 2 {
		t.Fatalf("readers %d, want one per alias", len(rs))
	}
	// Asked in the reverse order, so that "b" probes socket 0 first, names it a and moves on.
	for _, al := range []string{"b", "a", "b"} {
		acc, err := rs[al].Account(context.Background())
		if err != nil || acc.Alias != al {
			t.Fatalf("%s: answer alias %q err %v", al, acc.Alias, err)
		}
		ex, err := rs[al].Executions(context.Background())
		if err != nil || ex.Alias != al {
			t.Fatalf("%s: executions alias %q err %v", al, ex.Alias, err)
		}
	}
}

// countingReader counts the calls of Account.
type countingReader struct {
	idReader
	n *atomic.Int32
}

func (c countingReader) Account(ctx context.Context) (broker.Account, error) {
	c.n.Add(1)
	return c.idReader.Account(ctx)
}

func TestResolverCachesSocketPerAlias(t *testing.T) {
	t.Parallel()
	var n0, n1 atomic.Int32
	rs := resolverOf(t, bookMainB,
		countingReader{idReader{id: aliasID("a")}, &n0}, countingReader{idReader{id: aliasID("b")}, &n1})
	for i := 0; i < 3; i++ {
		if _, err := rs["b"].Account(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	// The first call probes socket 0 (names it a) and socket 1; the next two ask socket 1 alone.
	if n0.Load() != 1 || n1.Load() != 3 {
		t.Fatalf("socket 0 asked %d times, socket 1 %d times", n0.Load(), n1.Load())
	}
}

func TestResolverUnknownIDIsPermanentUnknownAccount(t *testing.T) {
	t.Parallel()
	stranger := inventedID(9)
	rs := resolverOf(t, bookMainB, idReader{id: aliasID("a")}, idReader{id: stranger})
	if _, err := rs["a"].Account(context.Background()); err != nil {
		t.Fatalf("a: %v", err)
	}
	_, err := rs["b"].Account(context.Background())
	var te contracts.TypedError
	if !errors.Is(err, accounts.ErrUnknownAccount) || !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("err %v, want a permanent unknown account", err)
	}
	if strings.Contains(err.Error(), stranger) || !strings.Contains(err.Error(), "socket 1") {
		t.Fatalf("text %q must name the socket index and no id", err)
	}
}

func TestResolverSocketDownIsFailedReadOfItsAlias(t *testing.T) {
	t.Parallel()
	down := errors.New("socket down")
	rs := resolverOf(t, bookMainB, idReader{id: aliasID("a")}, downReader{err: down})
	if acc, err := rs["a"].Account(context.Background()); err != nil || acc.Alias != "a" {
		t.Fatalf("a: %+v %v", acc, err)
	}
	if _, err := rs["b"].Account(context.Background()); !errors.Is(err, down) {
		t.Fatalf("b: err %v, want the dial error", err)
	}
}

func TestResolverAliasWithoutSocketIsRefused(t *testing.T) {
	t.Parallel()
	rs := resolverOf(t, bookMainB, idReader{id: aliasID("a")})
	if _, err := rs["b"].Account(context.Background()); err == nil {
		t.Fatal("an alias with no socket was read")
	}
}
