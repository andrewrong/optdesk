package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

type fakeSectionStore struct {
	flexsection.Store
	n *atomic.Int32
}

func (f fakeSectionStore) Close() error { f.n.Add(1); return nil }

// statementRig is an app whose three stores are fakes that count what is opened and closed.
type statementRig struct {
	a      *app
	log    *bytes.Buffer
	opened atomic.Int32
	closed atomic.Int32
	dsns   map[string]string
}

const statementEnv = "OT_PG_DSN_OPTION_LEDGER=ledger-dsn\nOT_PG_DSN_ACCOUNT=acct-dsn\nOT_PG_DSN_POSMARK=mark-dsn\n"

func newStatementRig(t *testing.T, env, book string) *statementRig {
	t.Helper()
	r := &statementRig{dsns: map[string]string{}}
	a, log := testApp(t, env)
	if book != "" {
		writeBookText(t, a, book)
	}
	r.a, r.log = a, log
	a.deps.ext = map[string]any{extOptionLedger: optionLedgerDeps{
		openFlexSections: func(_ context.Context, dsn string) (flexSectionStore, error) {
			r.opened.Add(1)
			r.dsns["sections"] = dsn
			return fakeSectionStore{n: &r.closed}, nil
		},
	}}
	a.deps.openAccount = func(_ context.Context, dsn string) (store.AccountStore, error) {
		r.opened.Add(1)
		r.dsns["account"] = dsn
		return fakeAcctStore{n: &r.closed}, nil
	}
	a.deps.openMark = func(_ context.Context, dsn string) (mark.Store, error) {
		r.opened.Add(1)
		r.dsns["mark"] = dsn
		return fakeMarkStore{n: &r.closed}, nil
	}
	return r
}

func (r *statementRig) build(t *testing.T) (id string, built bool) {
	t.Helper()
	jc := &jobCtx{f: &jobFactory{d: r.a.deps, clk: fakeClock{}}, cal: openCalendar{}, extra: &closerSet{}}
	j, reason, err := jobFamilies[jobStatementCapture].build(t.Context(), r.a, jc)
	if err != nil || reason != "" {
		t.Fatalf("build: reason %q err %v", reason, err)
	}
	if j == nil {
		return "", false
	}
	return j.ID(), true
}

var statementBook = bookText("a", "a", "b") + "statement = [\"b\"]\n"

func TestFamilyIsRegisteredAtOrder55(t *testing.T) {
	t.Parallel()
	f, ok := jobFamilies["statement_capture"]
	if !ok || f.order != 55 {
		t.Fatalf("family %+v registered %v", f, ok)
	}
}

func TestJobBuiltForStatementAlias(t *testing.T) {
	t.Parallel()
	r := newStatementRig(t, statementEnv, statementBook)
	id, built := r.build(t)
	if !built || id != "statement_capture" {
		t.Fatalf("built %v id %q; log %s", built, id, r.log)
	}
	want := map[string]string{"sections": "ledger-dsn", "account": "acct-dsn", "mark": "mark-dsn"}
	for k, v := range want {
		if r.dsns[k] != v {
			t.Errorf("store %s opened with %q, want %q", k, r.dsns[k], v)
		}
	}
}

func TestNoStatementAliasLeavesJobOut(t *testing.T) {
	t.Parallel()
	r := newStatementRig(t, statementEnv, bookText("a", "a", "b"))
	if _, built := r.build(t); built {
		t.Fatal("job built for a book without statement alias")
	}
	if r.opened.Load() != 0 || !bytes.Contains(r.log.Bytes(), []byte("job left out")) {
		t.Fatalf("opened %d; log %s", r.opened.Load(), r.log)
	}
}

func TestMissingAccountDsnLeavesJobOut(t *testing.T) {
	t.Parallel()
	r := newStatementRig(t, "OT_PG_DSN_OPTION_LEDGER=ledger-dsn\nOT_PG_DSN_POSMARK=mark-dsn\n", statementBook)
	if _, built := r.build(t); built {
		t.Fatal("job built without the account line")
	}
	log := r.log.String()
	if r.opened.Load() != 0 || !bytes.Contains(r.log.Bytes(), []byte(store.DSNEnvAccount+" is not set")) || bytes.Contains(r.log.Bytes(), []byte("ledger-dsn")) {
		t.Fatalf("opened %d; log %s", r.opened.Load(), log)
	}
}

func TestMissingAccountsBookLeavesJobOut(t *testing.T) {
	t.Parallel()
	r := newStatementRig(t, statementEnv, "")
	if err := os.Remove(filepath.Join(r.a.getenv("OT_HOME"), accountsFileName)); err != nil {
		t.Fatal(err)
	}
	if _, built := r.build(t); built {
		t.Fatal("job built without accounts.toml")
	}
	log := r.log.String()
	if r.opened.Load() != 0 || !bytes.Contains(r.log.Bytes(), []byte("job left out")) || !bytes.Contains(r.log.Bytes(), []byte(reasonNoBook)) {
		t.Fatalf("opened %d; log %s", r.opened.Load(), log)
	}
}
