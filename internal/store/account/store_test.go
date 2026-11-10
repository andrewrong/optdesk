package account

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type call struct {
	sql  string
	args []any
}

// fakeDB records the statements; the first statement of a transaction reports captureRows rows added.
type fakeDB struct {
	events      []string
	calls       []call
	captureRows int64
	execErr     error
	selSQL      string
	selArgs     []any
	rows        [][]any
	selErr      error
	closed      bool
}

type fakeTx struct{ f *fakeDB }

func (t fakeTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	if t.f.execErr != nil {
		return 0, t.f.execErr
	}
	t.f.calls = append(t.f.calls, call{sql, args})
	if sql == insertCaptureSQL {
		return t.f.captureRows, nil
	}
	return 1, nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.events = append(f.events, "begin")
	if err := fn(fakeTx{f}); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

type rowScanner struct{ vals []any }

func (s rowScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(s.vals[i]))
	}
	return nil
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	f.selSQL, f.selArgs = sql, args
	if f.selErr != nil {
		return f.selErr
	}
	for _, r := range f.rows {
		if err := scan(rowScanner{r}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

func capture() store.Capture {
	return store.Capture{
		ID: "a:2026-11-06:run1", Alias: "a", At: time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC),
		SessionDay: "2026-11-06", Status: "ok",
		Values: []store.AccountValue{{Tag: "NetLiquidation", Value: "100000.50", Currency: "USD"}, {Tag: "Cushion", Value: "0.8", Currency: "USD"}},
		Positions: []store.Position{{
			Symbol: "XYZ", SecType: "option", Currency: "USD", Expiry: "2026-12-18", Strike: "50", Multiplier: "100",
			Right: "put", ConID: 7, Quantity: "-2", AvgCost: "123.45",
		}},
	}
}

func kind(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not a TypedError: %v", err)
	}
	return te.Kind()
}

func TestWriteCaptureNewWritesChildrenInOneTransaction(t *testing.T) {
	t.Parallel()
	f := &fakeDB{captureRows: 1}
	if err := (&accountStore{db: f}).WriteCapture(context.Background(), capture()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) || len(f.calls) != 4 {
		t.Fatalf("events %v, calls %d", f.events, len(f.calls))
	}
	if f.calls[0].sql != insertCaptureSQL || f.calls[1].sql != insertValueSQL || f.calls[3].sql != insertPositionSQL {
		t.Fatal("capture first, then values, then positions")
	}
	if f.calls[3].args[0] != "a:2026-11-06:run1" || f.calls[3].args[1] != "a" || f.calls[3].args[3] != "2026-11-06" {
		t.Fatalf("child must carry the capture's id, alias and day: %v", f.calls[3].args)
	}
	for _, c := range f.calls {
		if n := strings.Count(c.sql, "$"); n != len(c.args) {
			t.Fatalf("%d placeholders, %d args: %s", n, len(c.args), c.sql)
		}
	}
}

func TestWriteCaptureExistingIDWritesNoChild(t *testing.T) {
	t.Parallel()
	f := &fakeDB{captureRows: 0}
	if err := (&accountStore{db: f}).WriteCapture(context.Background(), capture()); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0].sql != insertCaptureSQL {
		t.Fatalf("calls %v", f.calls)
	}
	if !strings.HasSuffix(insertCaptureSQL, "ON CONFLICT DO NOTHING") {
		t.Fatal("capture insert must be ON CONFLICT DO NOTHING")
	}
}

func TestWriteCaptureErrorRollsBackAndIsRetryable(t *testing.T) {
	t.Parallel()
	f := &fakeDB{captureRows: 1, execErr: errors.New("boom")}
	err := (&accountStore{db: f}).WriteCapture(context.Background(), capture())
	if err == nil || kind(t, err) != contracts.Retryable || !reflect.DeepEqual(f.events, []string{"begin", "rollback"}) {
		t.Fatalf("err %v, events %v", err, f.events)
	}
}

func TestWriteCaptureRejectsBadInputBeforeTheDatabase(t *testing.T) {
	t.Parallel()
	for name, mod := range map[string]func(*store.Capture){
		"no id":    func(c *store.Capture) { c.ID = "" },
		"no alias": func(c *store.Capture) { c.Alias = "" },
		"bad day":  func(c *store.Capture) { c.SessionDay = "06/11/2026" },
	} {
		c := capture()
		mod(&c)
		f := &fakeDB{}
		err := (&accountStore{db: f}).WriteCapture(context.Background(), c)
		if err == nil || kind(t, err) != contracts.Permanent || len(f.events) != 0 {
			t.Errorf("%s: err %v, events %v", name, err, f.events)
		}
	}
}

func TestWriteExecutionsInsertOnlyUnderAlias(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	rows := []store.Execution{
		{ExecID: "e1", SessionDay: "2026-11-06", Alias: "ignored", Quantity: "1", Price: "1.5", Commission: "0.65"},
		{ExecID: "e2", SessionDay: "2026-11-06", Quantity: "2", Price: "1.6", Commission: "1.3"},
	}
	if err := (&accountStore{db: f}).WriteExecutions(context.Background(), "b", rows); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) || len(f.calls) != 2 {
		t.Fatalf("events %v, calls %d", f.events, len(f.calls))
	}
	if f.calls[0].args[0] != "b" || f.calls[1].args[1] != "e2" {
		t.Fatalf("alias comes from the argument: %v", f.calls[0].args)
	}
	if !strings.HasSuffix(insertExecutionSQL, "ON CONFLICT DO NOTHING") || strings.Count(insertExecutionSQL, "$") != len(f.calls[0].args) {
		t.Fatal(insertExecutionSQL)
	}
}

func TestWriteExecutionsEmptyAndBadInput(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	s := &accountStore{db: f}
	if err := s.WriteExecutions(context.Background(), "a", nil); err != nil || len(f.events) != 0 {
		t.Fatalf("empty: %v %v", err, f.events)
	}
	if err := s.WriteExecutions(context.Background(), "", []store.Execution{{SessionDay: "2026-11-06"}}); err == nil || kind(t, err) != contracts.Permanent {
		t.Fatalf("no alias: %v", err)
	}
	if err := s.WriteExecutions(context.Background(), "a", []store.Execution{{SessionDay: "x"}}); err == nil || kind(t, err) != contracts.Permanent {
		t.Fatalf("bad day: %v", err)
	}
	if len(f.events) != 0 {
		t.Fatal("database touched")
	}
}

func TestReaderUsesViewsSortedAndReturnsText(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	f := &fakeDB{rows: [][]any{{"c1", "a", at, "2026-11-06", "XYZ", "option", "SMART", "USD", "2026-12-18", "50", "100", "put", int64(7), "-2", "123.45"}}}
	s := &accountStore{db: f}
	got, err := s.Positions(context.Background(), "a", "2026-11-06")
	if err != nil || len(got) != 1 || got[0].Quantity != "-2" || got[0].AvgCost != "123.45" || got[0].ConID != 7 {
		t.Fatalf("%v %v", got, err)
	}
	if !strings.Contains(f.selSQL, "FROM rd.account_position_r1") || !strings.Contains(f.selSQL, "ORDER BY capture_id, con_id") {
		t.Fatal(f.selSQL)
	}
	f.rows = [][]any{{"c1", "a", at, "2026-11-06", "Cushion", "0.8", "USD"}}
	vals, err := s.Values(context.Background(), "a", "2026-11-06")
	if err != nil || len(vals) != 1 || vals[0].Value != "0.8" || !strings.Contains(f.selSQL, "FROM rd.account_value_r1") {
		t.Fatalf("%v %v", vals, err)
	}
	f.rows = [][]any{{"a", "e1", int64(9), at, "2026-11-06", "XYZ", "option", "2026-12-18", "50", "put", "100", "USD", "SLD", "2", "1.5", "0.65", "USD"}}
	ex, err := s.Executions(context.Background(), "a", "2026-11-01", "2026-11-06")
	if err != nil || len(ex) != 1 || ex[0].Price != "1.5" || ex[0].PermID != 9 ||
		!strings.Contains(f.selSQL, "FROM rd.account_execution_r1") || !strings.Contains(f.selSQL, "ORDER BY exec_id") {
		t.Fatalf("%v %v", ex, err)
	}
	if !reflect.DeepEqual(f.selArgs, []any{"a", "2026-11-01", "2026-11-06"}) {
		t.Fatal(f.selArgs)
	}
}

func TestReaderEmptyIsNotNilAndErrorsAreTyped(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	s := &accountStore{db: f}
	p, err := s.Positions(context.Background(), "a", "2026-11-06")
	v, err2 := s.Values(context.Background(), "a", "2026-11-06")
	e, err3 := s.Executions(context.Background(), "a", "2026-11-06", "2026-11-06")
	if err != nil || err2 != nil || err3 != nil || p == nil || v == nil || e == nil {
		t.Fatalf("empty must be non-nil: %v %v %v", p, v, e)
	}
	f.selErr = errors.New("down")
	if _, err := s.Positions(context.Background(), "a", "2026-11-06"); err == nil || kind(t, err) != contracts.Retryable {
		t.Fatal(err)
	}
	if _, err := s.Executions(context.Background(), "a", "bad", "2026-11-06"); err == nil || kind(t, err) != contracts.Permanent {
		t.Fatal(err)
	}
}

func TestCloseClosesDatabase(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	if err := (&accountStore{db: f}).Close(); err != nil || !f.closed {
		t.Fatal("not closed")
	}
}

func TestWriteConstraintViolationIsPermanent(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		err  error
		want contracts.ErrorKind
	}{
		"not null":      {&pgconn.PgError{Code: "23502"}, contracts.Permanent},
		"foreign key":   {&pgconn.PgError{Code: "23503"}, contracts.Permanent},
		"unique":        {&pgconn.PgError{Code: "23505"}, contracts.Permanent},
		"check":         {&pgconn.PgError{Code: "23514"}, contracts.Permanent},
		"check wrapped": {fmt.Errorf("x: %w", &pgconn.PgError{Code: "23514"}), contracts.Permanent},
		"serialization": {&pgconn.PgError{Code: "40001"}, contracts.Retryable},
		"deadlock":      {&pgconn.PgError{Code: "40P01"}, contracts.Retryable},
		"connection":    {&pgconn.PgError{Code: "08006"}, contracts.Retryable},
		"plain":         {errors.New("boom"), contracts.Retryable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := &accountStore{db: &fakeDB{captureRows: 1, execErr: tc.err}}
			err := s.WriteCapture(context.Background(), capture())
			if err == nil || kind(t, err) != tc.want {
				t.Fatalf("WriteCapture: %v", err)
			}
			err = s.WriteExecutions(context.Background(), "a", []store.Execution{{ExecID: "e1", SessionDay: "2026-11-06"}})
			if err == nil || kind(t, err) != tc.want {
				t.Fatalf("WriteExecutions: %v", err)
			}
		})
	}
}
