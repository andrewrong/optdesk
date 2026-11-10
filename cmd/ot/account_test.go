package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	gwTwoRequired = "[[gateway]]\nname = \"g1\"\naccount = \"a\"\nrequired = true\n[[gateway]]\nname = \"g2\"\naccount = \"b\"\nrequired = true\n"
	gwMainB       = "[[gateway]]\nname = \"g1\"\naccount = \"a\"\nrequired = false\n[[gateway]]\nname = \"g2\"\naccount = \"b\"\nrequired = true\n"
	gwNoRequired  = "[[gateway]]\nname = \"g1\"\naccount = \"a\"\nrequired = false\n"
)

type fakeAcctReader struct{ n *atomic.Int32 }

func (fakeAcctReader) Account(context.Context) (broker.Account, error) { return broker.Account{}, nil }
func (fakeAcctReader) Executions(context.Context) (broker.Executions, error) {
	return broker.Executions{}, nil
}
func (f fakeAcctReader) Close() error { f.n.Add(1); return nil }

type acctJob struct{ fakeJob }

func (acctJob) ID() string { return "account_capture" }

type fakeAcctStore struct {
	store.AccountStore
	n *atomic.Int32
}

func (f fakeAcctStore) Close() error { f.n.Add(1); return nil }

// acctRig is run-once over fakes with the account lines in .env; got holds what the constructor received.
type acctRig struct {
	a              *app
	dialed         []string
	cfg            accountcapture.Config
	built          atomic.Int32
	closed, opened atomic.Int32
}

func newAcctRig(t *testing.T, gateways, sockets string) *acctRig {
	t.Helper()
	env := fullEnv + "OT_PG_DSN_ACCOUNT=acct-dsn\n"
	if gateways != "" {
		p := filepath.Join(t.TempDir(), "gateways.toml")
		if err := os.WriteFile(p, []byte(gateways), 0o600); err != nil {
			t.Fatal(err)
		}
		env += "OT_IBKR_GATEWAYS=" + p + "\n"
	}
	if sockets != "" {
		env += "OT_ACCOUNT_SOCKETS=" + sockets + "\n"
	}
	r := &acctRig{}
	a, _ := testApp(t, env)
	var opened, closed atomic.Int32
	var day contracts.Day
	runOnceFakes(a, &opened, &closed, &day)
	d := &a.deps
	d.dialAccount = func(_ context.Context, sock string, _ broker.Options) (broker.AccountReader, error) {
		r.dialed = append(r.dialed, sock)
		return fakeAcctReader{n: &r.closed}, nil
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
	r := newAcctRig(t, gwMainB, "a=/s/a.sock,b=/s/b.sock")
	var ids []string
	r.a.deps.newRunner = func(_ store.OpsLedger, _ clock.Calendar, _ clock.Clock, js []contracts.Job, _ runner.WindowRule) runner.Reconciler {
		for _, j := range js {
			ids = append(ids, j.ID())
		}
		return recRunner{day: new(contracts.Day)}
	}
	if err := runOnce(r.a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if r.built.Load() != 1 || r.cfg.Main != "b" || len(r.cfg.Readers) != 2 || r.cfg.Writer == nil || r.cfg.Clock == nil {
		t.Fatalf("built %d cfg main %q readers %d", r.built.Load(), r.cfg.Main, len(r.cfg.Readers))
	}
	if strings.Join(r.dialed, ",") != "/s/a.sock,/s/b.sock" || r.opened.Load() != 1 {
		t.Fatalf("dialed %v opened %d", r.dialed, r.opened.Load())
	}
	if r.closed.Load() != 3 {
		t.Fatalf("closed %d, want two readers and the store", r.closed.Load())
	}
	if !slices.Contains(ids, "account_capture") {
		t.Fatalf("jobs %v lack account_capture", ids)
	}
}

func TestAccountLeftOutWithoutSockets(t *testing.T) {
	t.Parallel()
	r := newAcctRig(t, "", "")
	if err := runOnce(r.a, []string{"--day", "2026-03-04"}); err != nil || r.built.Load() != 0 {
		t.Fatalf("err %v built %d", err, r.built.Load())
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

func TestAccountRefusesNoMain(t *testing.T) {
	t.Parallel()
	assertRefused(t, newAcctRig(t, gwNoRequired, "a=/s/a.sock"), "OT_IBKR_GATEWAYS")
	assertRefused(t, newAcctRig(t, "", "a=/s/a.sock"), "OT_IBKR_GATEWAYS")
}

func TestAccountRefusesTwoMains(t *testing.T) {
	t.Parallel()
	assertRefused(t, newAcctRig(t, gwTwoRequired, "a=/s/a.sock,b=/s/b.sock"), "OT_IBKR_GATEWAYS")
}

func TestAccountRefusesMainNotInSockets(t *testing.T) {
	t.Parallel()
	assertRefused(t, newAcctRig(t, gwMainB, "a=/s/a.sock"), "OT_ACCOUNT_SOCKETS")
}

func TestAccountGatewaysKeyReadThroughDeps(t *testing.T) {
	t.Parallel()
	r := newAcctRig(t, "", "")
	p := filepath.Join(t.TempDir(), "gw.toml")
	if err := os.WriteFile(p, []byte(gwMainB), 0o600); err != nil {
		t.Fatal(err)
	}
	var reads atomic.Int32
	r.a.deps.readEnv = func(string) (map[string]string, error) {
		reads.Add(1)
		return map[string]string{
			"OT_ACCOUNT_SOCKETS": "a=/s/a.sock,b=/s/b.sock", "OT_IBKR_GATEWAYS": p, "OT_PG_DSN_ACCOUNT": "x",
		}, nil
	}
	var extra brokerSet
	j, ok, err := r.a.accountCapture(context.Background(), jobFactory{d: r.a.deps}, &extra)
	if err != nil || !ok || j == nil || reads.Load() == 0 || r.cfg.Main != "b" {
		t.Fatalf("ok %v err %v reads %d main %q", ok, err, reads.Load(), r.cfg.Main)
	}
}
