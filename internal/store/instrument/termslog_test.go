package instrument

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

const marker = "PLANTED-MARKER-9137"

// stateErr is an error that carries a SQLSTATE, as pgx's does.
type stateErr struct{ msg, state string }

func (e stateErr) Error() string    { return e.msg }
func (e stateErr) SQLState() string { return e.state }

// logDB wraps fakeDB: the terms insert fails with termsErr, and the rollback to the savepoint fails with
// rollbackErr, so a statement after the terms insert can fail.
type logDB struct {
	*fakeDB
	termsErr    error
	rollbackErr error
}

type logTx struct {
	fakeTx
	d *logDB
}

func (t logTx) Exec(ctx context.Context, sql string, args ...any) error {
	switch {
	case sql == insertTermsSQL && t.d.termsErr != nil:
		t.f.stmts = append(t.f.stmts, sql)
		return t.d.termsErr
	case sql == rollbackSQL && t.d.rollbackErr != nil:
		return t.d.rollbackErr
	}
	return t.fakeTx.Exec(ctx, sql, args...)
}

func (d *logDB) InTx(_ context.Context, fn func(execer) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return fn(logTx{fakeTx{d.fakeDB}, d})
}

func logged(d database) (*instrumentStore, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return &instrumentStore{db: d, log: slog.New(h)}, &buf
}

var (
	optKey  = store.Identifier{Scheme: "ibkr_conid", Value: "7001"}
	optOSI  = []store.Identifier{{Scheme: "osi", Value: "SPY   261218C00500000"}}
	optSpec = store.InstrumentSpec{Type: "option", Currency: "USD", PriceIncrement: "0.01", LotSize: "100"}
)

func lines(buf *bytes.Buffer) []string {
	var out []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "option terms insert rolled back") {
			out = append(out, l)
		}
	}
	return out
}

func TestResolveTermsFailureLogged(t *testing.T) {
	t.Parallel()
	d := &logDB{fakeDB: newFake(), termsErr: stateErr{msg: "permission denied " + marker, state: "42501"}}
	s, buf := logged(d)
	id, err := s.ResolveInstrument(context.Background(), optKey, optSpec, optOSI)
	if err != nil || id != 101 {
		t.Fatalf("id %d err %v", id, err)
	}
	ls := lines(buf)
	if len(ls) != 1 {
		t.Fatalf("lines %d: %q", len(ls), buf.String())
	}
	for _, want := range []string{"instrument_id=101", "err=42501", "component=store", "status=rolled_back", "level=WARN"} {
		if !strings.Contains(ls[0], want) {
			t.Errorf("line lacks %q: %s", want, ls[0])
		}
	}
	if strings.Contains(buf.String(), marker) {
		t.Errorf("error text leaked: %s", buf.String())
	}
}

func TestResolveTermsFailureNoState(t *testing.T) {
	t.Parallel()
	d := &logDB{fakeDB: newFake(), termsErr: errors.New("boom " + marker)}
	s, buf := logged(d)
	if _, err := s.ResolveInstrument(context.Background(), optKey, optSpec, optOSI); err != nil {
		t.Fatal(err)
	}
	ls := lines(buf)
	if len(ls) != 1 || !strings.Contains(ls[0], "err=other") || strings.Contains(buf.String(), marker) {
		t.Fatalf("log %q", buf.String())
	}
}

func TestSQLClass(t *testing.T) {
	t.Parallel()
	inner := stateErr{msg: "x", state: "23505"}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"state", inner, "23505"},
		{"none", errors.New("plain"), "other"},
		{"wrapped twice", fmt.Errorf("a: %w", fmt.Errorf("b: %w", inner)), "23505"},
	}
	for _, c := range cases {
		if got := sqlClass(c.err); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestResolveTermsFailureOuterError(t *testing.T) {
	t.Parallel()
	d := &logDB{fakeDB: newFake(), termsErr: stateErr{msg: "denied", state: "42501"}, rollbackErr: errors.New("rollback failed")}
	s, buf := logged(d)
	if _, err := s.ResolveInstrument(context.Background(), optKey, optSpec, optOSI); err == nil {
		t.Fatal("want an error")
	}
	if ls := lines(buf); len(ls) != 0 {
		t.Fatalf("lines %q", ls)
	}
}

func TestResolveNoTermsNoLog(t *testing.T) {
	t.Parallel()
	failing := stateErr{msg: "denied", state: "42501"}
	run := func(name string, d *logDB, key store.Identifier, spec store.InstrumentSpec, extra []store.Identifier, repeat int) {
		s, buf := logged(d)
		for i := 0; i < repeat; i++ {
			if _, err := s.ResolveInstrument(context.Background(), key, spec, extra); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
		if ls := lines(buf); len(ls) != 0 {
			t.Errorf("%s: lines %q", name, ls)
		}
	}
	// An existing key: the second call finds it and writes nothing, so only the first can log; make the first succeed.
	run("existing key", &logDB{fakeDB: newFake()}, optKey, optSpec, optOSI, 2)
	stock := optSpec
	stock.Type = "stock"
	run("stock", &logDB{fakeDB: newFake(), termsErr: failing}, optKey, stock, optOSI, 1)
	run("no osi", &logDB{fakeDB: newFake(), termsErr: failing}, optKey, optSpec, nil, 1)
	run("bad osi", &logDB{fakeDB: newFake(), termsErr: failing}, optKey, optSpec,
		[]store.Identifier{{Scheme: "osi", Value: "not-an-osi"}}, 1)
	run("terms ok", &logDB{fakeDB: newFake()}, optKey, optSpec, optOSI, 1)
}
