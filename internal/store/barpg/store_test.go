package barpg

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
	"github.com/andrewrong/optdesk/internal/store/bar"
)

var _ bar.BarStore = (*Store)(nil)

type call struct {
	sql  string
	args []any
}

type fakeDB struct {
	events  []string
	calls   []call
	failAt  int   // 1-based statement that fails, 0 for none
	failErr error // returned by the failing statement instead of a plain error, when set
	selSQL  string
	selArgs []any
	rows    [][]any
	closed  bool
}

type fakeTx struct{ f *fakeDB }

func (t fakeTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	t.f.calls = append(t.f.calls, call{sql, args})
	if t.f.failAt == len(t.f.calls) {
		if t.f.failErr != nil {
			return 0, t.f.failErr
		}
		return 0, errors.New("boom")
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
	for _, r := range f.rows {
		if err := scan(rowScanner{r}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

func day(s string) contracts.Day { return contracts.Day(s) }

func bars(n int) []contracts.PriceBar {
	out := make([]contracts.PriceBar, n)
	for i := range out {
		out[i] = contracts.PriceBar{
			InstrumentID: 7, Basis: "trades", AdjustEpoch: 1, Open: "1", High: "2", Low: "1", Close: "1.5",
			Currency: "USD", Source: "ibkr", ReceivedAt: time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC),
		}
		out[i].SessionDay.Time = time.Date(2026, 11, 2+i, 0, 0, 0, 0, time.UTC)
	}
	return out
}

func TestStoreImplementsBarStore(t *testing.T) {
	t.Parallel()
	var s bar.BarStore = &Store{db: &fakeDB{}}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInsertQueryText(t *testing.T) {
	t.Parallel()
	for name, sql := range map[string]string{"price_bar": insertPriceBarSQL, "fx_rate": insertFxRateSQL} {
		if !strings.Contains(sql, "INSERT INTO obs."+name) || !strings.HasSuffix(sql, "ON CONFLICT DO NOTHING") {
			t.Errorf("%s statement = %q", name, sql)
		}
	}
	for _, sql := range []string{insertPriceBarSQL, insertFxRateSQL, selectPriceBarSQL, selectFxRatesSQL, selectLatestFxSQL} {
		for _, w := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
			if strings.Contains(sql, w) {
				t.Errorf("%q contains %s", sql, w)
			}
		}
	}
}

func TestWinnerQueryReadsView(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{selectPriceBarSQL, selectFxRatesSQL, selectLatestFxSQL} {
		if strings.Contains(sql, "obs.") || !strings.Contains(sql, "rd.") {
			t.Errorf("query reads a table: %q", sql)
		}
	}
	at := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	f := &fakeDB{rows: [][]any{{int64(7), "2026-11-02", "trades", int32(2), "1", "2", "1", "1.5", (*string)(nil), "USD", "ibkr", at}}}
	s := &Store{db: f}
	got, err := s.PriceBars(context.Background(), 7, "trades", day("2026-11-01"), day("2026-11-09"))
	if err != nil || len(got) != 1 || got[0].AdjustEpoch != 2 || got[0].InstrumentID != 7 || got[0].Volume != nil ||
		got[0].SessionDay.Format("2006-01-02") != "2026-11-02" {
		t.Fatalf("bars = %+v, %v", got, err)
	}
	if f.selSQL != selectPriceBarSQL || !reflect.DeepEqual(f.selArgs, []any{int64(7), "trades", "2026-11-01", "2026-11-09"}) {
		t.Errorf("select = %q %v", f.selSQL, f.selArgs)
	}
	none, err := (&Store{db: &fakeDB{}}).PriceBars(context.Background(), 7, "trades", day("2026-11-01"), day("2026-11-09"))
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("empty = %v, %v", none, err)
	}
}

func TestLatestFxRateNotFound(t *testing.T) {
	t.Parallel()
	r, found, err := (&Store{db: &fakeDB{}}).LatestFxRate(context.Background(), "USDJPY", day("2026-11-09"))
	if err != nil || found || r != (contracts.FxRate{}) {
		t.Errorf("got %+v %v %v", r, found, err)
	}
	f := &fakeDB{rows: [][]any{{"USDJPY", "2026-11-06", "151.2", "ibkr_midpoint", time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)}}}
	r, found, err = (&Store{db: f}).LatestFxRate(context.Background(), "USDJPY", day("2026-11-09"))
	if err != nil || !found || r.Rate != "151.2" || r.RateDay.Format("2006-01-02") != "2026-11-06" {
		t.Errorf("got %+v %v %v", r, found, err)
	}
}

func TestOneCallOneTransaction(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	s := &Store{db: f}
	n, err := s.InsertPriceBars(context.Background(), bars(3))
	if err != nil || n != 3 || len(f.calls) != 3 || !reflect.DeepEqual(f.events, []string{"begin", "commit"}) {
		t.Fatalf("n=%d err=%v calls=%d events=%v", n, err, len(f.calls), f.events)
	}
	if f.calls[0].args[2] != "trades" || f.calls[0].args[3] != int32(1) || f.calls[0].args[1] != "2026-11-02" {
		t.Errorf("args = %v", f.calls[0].args)
	}

	f = &fakeDB{failAt: 2}
	s = &Store{db: f}
	n, err = s.InsertPriceBars(context.Background(), bars(3))
	var te contracts.TypedError
	if err == nil || n != 0 || !errors.As(err, &te) || te.Kind() != contracts.Retryable ||
		!reflect.DeepEqual(f.events, []string{"begin", "rollback"}) {
		t.Errorf("n=%d err=%v events=%v", n, err, f.events)
	}

	f = &fakeDB{}
	if n, err := (&Store{db: f}).InsertPriceBars(context.Background(), nil); err != nil || n != 0 || len(f.events) != 0 {
		t.Errorf("empty: n=%d err=%v events=%v", n, err, f.events)
	}
	fx := contracts.FxRate{Pair: "USDJPY", Rate: "151.2", Source: "ibkr_midpoint"}
	fx.RateDay.Time = time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	f = &fakeDB{}
	if n, err := (&Store{db: f}).InsertFxRates(context.Background(), []contracts.FxRate{fx, fx}); err != nil || n != 2 ||
		!reflect.DeepEqual(f.events, []string{"begin", "commit"}) {
		t.Errorf("fx: n=%d err=%v events=%v", n, err, f.events)
	}
}

func TestInsertConstraintViolationIsPermanent(t *testing.T) {
	t.Parallel()
	fx := contracts.FxRate{Pair: "USDJPY", Rate: "151.2", Source: "ibkr_midpoint"}
	fx.RateDay.Time = time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	writes := map[string]func(s *Store) error{
		"price_bars": func(s *Store) error { _, err := s.InsertPriceBars(context.Background(), bars(2)); return err },
		"fx_rates": func(s *Store) error {
			_, err := s.InsertFxRates(context.Background(), []contracts.FxRate{fx, fx})
			return err
		},
	}
	cases := []struct {
		name string
		err  error
		want contracts.ErrorKind
	}{
		{"not null", &pgconn.PgError{Code: "23502"}, contracts.Permanent},
		{"foreign key", &pgconn.PgError{Code: "23503"}, contracts.Permanent},
		{"unique", &pgconn.PgError{Code: "23505"}, contracts.Permanent},
		{"check", &pgconn.PgError{Code: "23514"}, contracts.Permanent},
		{"not null wrapped", fmt.Errorf("x: %w", &pgconn.PgError{Code: "23502"}), contracts.Permanent},
		{"foreign key wrapped", fmt.Errorf("x: %w", &pgconn.PgError{Code: "23503"}), contracts.Permanent},
		{"unique wrapped", fmt.Errorf("x: %w", &pgconn.PgError{Code: "23505"}), contracts.Permanent},
		{"check wrapped", fmt.Errorf("x: %w", &pgconn.PgError{Code: "23514"}), contracts.Permanent},
		{"serialization", &pgconn.PgError{Code: "40001"}, contracts.Retryable},
		{"deadlock", &pgconn.PgError{Code: "40P01"}, contracts.Retryable},
		{"connection", &pgconn.PgError{Code: "08006"}, contracts.Retryable},
		{"plain", errors.New("boom"), contracts.Retryable},
	}
	for wname, write := range writes {
		for _, c := range cases {
			t.Run(wname+"/"+c.name, func(t *testing.T) {
				t.Parallel()
				f := &fakeDB{failAt: 1, failErr: c.err}
				err := write(&Store{db: f})
				var te contracts.TypedError
				if err == nil || !errors.As(err, &te) || te.Kind() != c.want {
					t.Fatalf("err = %v, want kind %v", err, c.want)
				}
				if !errors.Is(err, c.err) {
					t.Errorf("err %v does not wrap the cause", err)
				}
				if !reflect.DeepEqual(f.events, []string{"begin", "rollback"}) {
					t.Errorf("events = %v", f.events)
				}
			})
		}
	}
	if isConstraintViolation(nil) {
		t.Error("nil is not a constraint violation")
	}
}
