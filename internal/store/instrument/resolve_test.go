package instrument

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeDB is an in-memory master: it answers the lookup and the instrument insert of resolve.go and records every
// statement in the order it arrives.
type fakeDB struct {
	mu     sync.Mutex
	stmts  []string // the first word group of each statement: lock, lookup, insert-instrument, insert-identifier
	ids    map[store.Identifier]int64
	nextID int64
	events []string
	args   [][]any
	rows   [][]any // answers of Select
	selSQL []string
	closed bool
	// failTerms makes the terms insert fail.
	failTerms bool
}

func newFake() *fakeDB { return &fakeDB{ids: map[store.Identifier]int64{}, nextID: 100} }

type fakeTx struct{ f *fakeDB }

func (t fakeTx) Exec(_ context.Context, sql string, args ...any) error {
	t.f.stmts = append(t.f.stmts, sql)
	t.f.args = append(t.f.args, args)
	if sql == insertTermsSQL && t.f.failTerms {
		return fmt.Errorf("permission denied")
	}
	if sql == insertIdentifierSQL {
		t.f.ids[store.Identifier{Scheme: args[1].(string), Value: args[2].(string)}] = args[0].(int64)
	}
	return nil
}

func (t fakeTx) Query(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	t.f.stmts = append(t.f.stmts, sql)
	t.f.args = append(t.f.args, args)
	switch sql {
	case lookupSQL:
		if id, ok := t.f.ids[store.Identifier{Scheme: args[0].(string), Value: args[1].(string)}]; ok {
			return scan(valsScanner{id})
		}
	case insertInstrumentSQL:
		t.f.nextID++
		return scan(valsScanner{t.f.nextID})
	}
	return nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, "begin")
	if err := fn(fakeTx{f}); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

func (f *fakeDB) Select(_ context.Context, sql string, _ []any, scan func(scanner) error) error {
	f.selSQL = append(f.selSQL, sql)
	for _, r := range f.rows {
		if err := scan(valsScanner(r)); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

// valsScanner copies vals into the destinations; a nil value leaves a pointer destination nil.
type valsScanner []any

func (s valsScanner) Scan(dest ...any) error {
	for i, d := range dest {
		dv := reflect.ValueOf(d).Elem()
		if s[i] == nil {
			dv.Set(reflect.Zero(dv.Type()))
			continue
		}
		v := reflect.ValueOf(s[i])
		if dv.Kind() == reflect.Pointer {
			p := reflect.New(dv.Type().Elem())
			p.Elem().Set(v)
			dv.Set(p)
			continue
		}
		dv.Set(v)
	}
	return nil
}

func goodSpec() store.InstrumentSpec {
	return store.InstrumentSpec{Type: "option", Name: "SPY 261120C00450000", Currency: "USD", MIC: "XNYS",
		CalendarID: "XNYS", Status: "", PriceIncrement: "0.01", LotSize: "100"}
}

func assertPermanent(t *testing.T, err error) {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("want a Permanent error, got %v", err)
	}
}

var (
	keyID  = store.Identifier{Scheme: "ibkr_conid", Value: "12345"}
	extraO = store.Identifier{Scheme: "osi", Value: "SPY   261120C00450000"}
)

func TestResolveCreates(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &instrumentStore{db: f}
	id, err := s.ResolveInstrument(context.Background(), keyID, goodSpec(), []store.Identifier{extraO})
	if err != nil {
		t.Fatal(err)
	}
	if id != 101 {
		t.Fatalf("id = %d, want 101", id)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) {
		t.Fatalf("events %v: want one transaction", f.events)
	}
	if f.ids[keyID] != 101 || f.ids[extraO] != 101 {
		t.Fatalf("identifiers stored: %v", f.ids)
	}
	var inserts int
	for _, q := range f.stmts {
		if q == insertIdentifierSQL {
			inserts++
		}
	}
	if inserts != 2 {
		t.Fatalf("%d identifier inserts, want 2", inserts)
	}
	// the status defaults to active
	for i, q := range f.stmts {
		if q == insertInstrumentSQL && f.args[i][5] != "active" {
			t.Fatalf("status argument %v, want active", f.args[i][5])
		}
	}
}

func TestResolveSame(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &instrumentStore{db: f}
	first, err := s.ResolveInstrument(context.Background(), keyID, goodSpec(), []store.Identifier{extraO})
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.stmts)
	second, err := s.ResolveInstrument(context.Background(), keyID, goodSpec(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("ids %d then %d", first, second)
	}
	for _, q := range f.stmts[before:] {
		if q == insertInstrumentSQL || q == insertIdentifierSQL {
			t.Fatalf("second call inserted: %s", q)
		}
	}
}

func TestResolveLock(t *testing.T) {
	t.Parallel()
	f := newFake()
	if _, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, goodSpec(), nil); err != nil {
		t.Fatal(err)
	}
	if len(f.stmts) < 2 || f.stmts[0] != lockSQL || f.stmts[1] != lookupSQL {
		t.Fatalf("statement order: %q", f.stmts)
	}
	if !strings.Contains(lockSQL, "pg_advisory_xact_lock(hashtext(") {
		t.Fatalf("lock statement: %s", lockSQL)
	}
	if !reflect.DeepEqual(f.args[0], []any{"ibkr_conid", "12345"}) {
		t.Fatalf("lock arguments %v: want the key", f.args[0])
	}
}

func TestResolveExtraTaken(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.ids[extraO] = 7 // another instrument already has this osi symbol
	_, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, goodSpec(), []store.Identifier{extraO})
	assertPermanent(t, err)
	for _, q := range f.stmts {
		if q == insertInstrumentSQL || q == insertIdentifierSQL {
			t.Fatalf("insert after a taken extra: %s", q)
		}
	}
	if _, ok := f.ids[keyID]; ok {
		t.Fatal("key was written")
	}
}

func TestResolveChecks(t *testing.T) {
	t.Parallel()
	mod := func(f func(*store.Identifier, *store.InstrumentSpec, *[]store.Identifier)) func() (store.Identifier, store.InstrumentSpec, []store.Identifier) {
		return func() (store.Identifier, store.InstrumentSpec, []store.Identifier) {
			k, sp, ex := keyID, goodSpec(), []store.Identifier(nil)
			f(&k, &sp, &ex)
			return k, sp, ex
		}
	}
	cases := map[string]func() (store.Identifier, store.InstrumentSpec, []store.Identifier){
		"unknown scheme": mod(func(k *store.Identifier, _ *store.InstrumentSpec, _ *[]store.Identifier) { k.Scheme = "cusip" }),
		"empty value":    mod(func(k *store.Identifier, _ *store.InstrumentSpec, _ *[]store.Identifier) { k.Value = "" }),
		"extra unknown scheme": mod(func(_ *store.Identifier, _ *store.InstrumentSpec, e *[]store.Identifier) {
			*e = []store.Identifier{{Scheme: "x", Value: "1"}}
		}),
		"extra empty value": mod(func(_ *store.Identifier, _ *store.InstrumentSpec, e *[]store.Identifier) {
			*e = []store.Identifier{{Scheme: "isin", Value: ""}}
		}),
		"type":                  mod(func(_ *store.Identifier, s *store.InstrumentSpec, _ *[]store.Identifier) { s.Type = "future" }),
		"currency lower":        mod(func(_ *store.Identifier, s *store.InstrumentSpec, _ *[]store.Identifier) { s.Currency = "usd" }),
		"currency long":         mod(func(_ *store.Identifier, s *store.InstrumentSpec, _ *[]store.Identifier) { s.Currency = "USDX" }),
		"currency empty":        mod(func(_ *store.Identifier, s *store.InstrumentSpec, _ *[]store.Identifier) { s.Currency = "" }),
		"price increment":       mod(func(_ *store.Identifier, s *store.InstrumentSpec, _ *[]store.Identifier) { s.PriceIncrement = "1e-2" }),
		"price increment empty": mod(func(_ *store.Identifier, s *store.InstrumentSpec, _ *[]store.Identifier) { s.PriceIncrement = "" }),
		"lot size":              mod(func(_ *store.Identifier, s *store.InstrumentSpec, _ *[]store.Identifier) { s.LotSize = "ten" }),
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			k, sp, ex := mk()
			_, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), k, sp, ex)
			assertPermanent(t, err)
			if len(f.events)+len(f.stmts) != 0 {
				t.Fatalf("seam reached: %v %v", f.events, f.stmts)
			}
		})
	}
}

func TestResolveDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	s := &instrumentStore{db: failDB{}}
	_, err := s.ResolveInstrument(context.Background(), keyID, goodSpec(), nil)
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("want Retryable, got %v", err)
	}
}

type failDB struct{}

func (failDB) InTx(context.Context, func(execer) error) error { return fmt.Errorf("down") }
func (failDB) Select(context.Context, string, []any, func(scanner) error) error {
	return fmt.Errorf("down")
}
func (failDB) Close() error { return nil }

// termsStmts returns the statements of the terms savepoint, in order.
func termsStmts(f *fakeDB) []string {
	var out []string
	for _, q := range f.stmts {
		if q == savepointSQL || q == insertTermsSQL || q == releaseSQL || q == rollbackSQL {
			out = append(out, q)
		}
	}
	return out
}

func TestResolveOptionTerms(t *testing.T) {
	t.Parallel()
	f := newFake()
	if _, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, goodSpec(), []store.Identifier{extraO}); err != nil {
		t.Fatal(err)
	}
	if got := termsStmts(f); !reflect.DeepEqual(got, []string{savepointSQL, insertTermsSQL, releaseSQL}) {
		t.Fatalf("terms statements %q", got)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) {
		t.Fatalf("events %v", f.events)
	}
	// the terms come after the instrument and identifier inserts, in the same transaction
	last := f.stmts[len(f.stmts)-3:]
	if last[0] != savepointSQL {
		t.Fatalf("savepoint not last: %q", f.stmts)
	}
	for i, q := range f.stmts {
		if q == insertTermsSQL {
			want := []any{int64(101), "C", "450", "USD", "100", "2026-11-20"}
			if !reflect.DeepEqual(f.args[i], want) {
				t.Fatalf("terms args %v, want %v", f.args[i], want)
			}
		}
	}
	for _, want := range []string{"'unknown', 'unknown'", "ON CONFLICT DO NOTHING"} {
		if !strings.Contains(insertTermsSQL, want) {
			t.Fatalf("terms insert lacks %q", want)
		}
	}
}

func TestResolveTermsFailureKeepsInstrument(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.failTerms = true
	id, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, goodSpec(), []store.Identifier{extraO})
	if err != nil || id != 101 {
		t.Fatalf("id %d, err %v", id, err)
	}
	if got := termsStmts(f); !reflect.DeepEqual(got, []string{savepointSQL, insertTermsSQL, rollbackSQL}) {
		t.Fatalf("terms statements %q", got)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) || f.ids[keyID] != 101 {
		t.Fatalf("events %v, ids %v", f.events, f.ids)
	}
}

func TestResolveNoTerms(t *testing.T) {
	t.Parallel()
	stock := goodSpec()
	stock.Type = "stock"
	cases := map[string]struct {
		spec  store.InstrumentSpec
		extra []store.Identifier
		seed  bool
	}{
		"existing key":    {goodSpec(), []store.Identifier{extraO}, true},
		"stock":           {stock, []store.Identifier{extraO}, false},
		"no osi":          {goodSpec(), []store.Identifier{{Scheme: "isin", Value: "US0000000001"}}, false},
		"no extra":        {goodSpec(), nil, false},
		"osi unparseable": {goodSpec(), []store.Identifier{{Scheme: "osi", Value: "SPY   261120X00450000"}}, false},
		"osi short":       {goodSpec(), []store.Identifier{{Scheme: "osi", Value: "SPY"}}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			if c.seed {
				f.ids[keyID] = 7
			}
			id, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, c.spec, c.extra)
			if err != nil || id == 0 {
				t.Fatalf("id %d, err %v", id, err)
			}
			if got := termsStmts(f); len(got) != 0 {
				t.Fatalf("terms statements %q", got)
			}
			if !c.seed && f.ids[keyID] != id {
				t.Fatal("instrument was not created")
			}
		})
	}
}

func TestResolveOsiCollision(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.ids[extraO] = 7 // an SPX option already holds this osi; the new key is an SPXW option with the same osi
	_, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), store.Identifier{Scheme: "ibkr_conid", Value: "999"},
		goodSpec(), []store.Identifier{extraO})
	assertPermanent(t, err)
	if got := termsStmts(f); len(got) != 0 {
		t.Fatalf("terms statements %q", got)
	}
	for _, q := range f.stmts {
		if q == insertInstrumentSQL || q == insertIdentifierSQL {
			t.Fatalf("write after a collision: %s", q)
		}
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "rollback"}) {
		t.Fatalf("events %v", f.events)
	}
}
