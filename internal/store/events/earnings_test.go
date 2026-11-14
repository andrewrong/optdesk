package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const issuer = "cik:0000320193"

// fakeDB keeps the rows by the key (observed_on, issuer, attempt), as the table with ON CONFLICT DO NOTHING
// would, and answers a select with the rows of the issuer in the range, ordered by day then attempt, as the
// view query does.
type fakeDB struct {
	rows   map[[3]string][]any
	keys   [][3]string
	failed error
	execs  int
	stmts  []stmt
}

func newFake() *fakeDB { return &fakeDB{rows: map[[3]string][]any{}} }

// Upsert reads the argument lists of the statements of one call (the data row, when there is one, then the answer)
// and stores the row in the argument order of the view, attempt 1; a later call of one key replaces the row.
func (f *fakeDB) Upsert(_ context.Context, stmts []stmt) ([]int, error) {
	if f.failed != nil {
		return nil, f.failed
	}
	f.execs++
	f.stmts = append(f.stmts, stmts...)
	ans := stmts[len(stmts)-1].args
	var report, fiscal, estimate *string
	tod := ""
	if len(stmts) == 2 {
		d := stmts[0].args
		r := d[4].(string)
		report, fiscal, estimate, tod = &r, d[5].(*string), d[7].(*string), d[6].(string)
	}
	f.put([]any{ans[1], ans[0], 1, ans[3], ans[2], ans[4], report, fiscal, tod, estimate, ans[5], ans[6], ans[8]})
	return make([]int, len(stmts)), nil
}

func (f *fakeDB) put(args []any) {
	k := [3]string{args[0].(string), args[1].(string), itoa(args[2].(int))}
	if _, ok := f.rows[k]; !ok {
		f.keys = append(f.keys, k)
	}
	f.rows[k] = args
}

func itoa(n int) string { return string(rune('0' + n)) }

func (f *fakeDB) Select(_ context.Context, _ string, args []any, scan func(scanner) error) error {
	if f.failed != nil {
		return f.failed
	}
	var picked [][3]string
	for _, k := range f.keys {
		if k[1] == args[0].(string) && k[0] >= args[1].(string) && k[0] <= args[2].(string) {
			picked = append(picked, k)
		}
	}
	for i := range picked { // day, then attempt ascending
		for j := i + 1; j < len(picked); j++ {
			if picked[j][0] < picked[i][0] || (picked[j][0] == picked[i][0] && picked[j][2] < picked[i][2]) {
				picked[i], picked[j] = picked[j], picked[i]
			}
		}
	}
	for _, k := range picked {
		if err := scan(rowScanner{f.rows[k]}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

type rowScanner struct{ a []any }

func mustDay(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

// Scan hands back the columns of the insert arguments in the order of the view.
func (s rowScanner) Scan(dest ...any) error {
	a := s.a
	*dest[0].(*time.Time) = mustDay(a[0].(string))
	*dest[1].(*string) = a[1].(string)
	*dest[2].(*int) = a[2].(int)
	for i := 3; i <= 5; i++ {
		*dest[i].(*string) = a[i].(string)
	}
	for _, i := range []int{6, 7} {
		if v := a[i].(*string); v != nil {
			t := mustDay(*v)
			*dest[i].(**time.Time) = &t
		}
	}
	*dest[8].(*string) = a[8].(string)
	*dest[9].(**string) = a[9].(*string)
	*dest[10].(*string), *dest[11].(*string) = a[10].(string), a[11].(string)
	*dest[12].(*time.Time) = a[12].(time.Time)
	return nil
}

func row(day string, attempt int, status string) contracts.EarningsDate {
	r := contracts.EarningsDate{
		ObservedOn: types.SerializableDate{Time: mustDay(day)}, Issuer: issuer, Attempt: attempt, Symbol: "AAPL",
		Market: "US", Status: "absent", Source: "alphavantage", ReceivedAt: time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC),
	}
	switch status {
	case "error":
		r.Status, r.ErrorCode = "error", "rate_limited"
	case "listed":
		r.Status = "listed"
		r.ReportDate = &types.SerializableDate{Time: mustDay("2026-11-20")}
		est := "1.25"
		r.Estimate = &est
		r.TimeOfDay = "post-market"
	}
	return r
}

func TestInsertEarningsDateSendsTheUpsertOfOneRow(t *testing.T) {
	t.Parallel()
	db := newFake()
	s := &earningsStore{db: db}
	if err := s.InsertEarningsDate(context.Background(), row("2026-11-09", 1, "listed")); err != nil {
		t.Fatal(err)
	}
	if db.execs != 1 || len(db.stmts) != 2 {
		t.Fatalf("%d calls, %d statements; want one call of the data row and the answer", db.execs, len(db.stmts))
	}
	if !strings.Contains(db.stmts[0].sql, "ON CONFLICT (issuer, period_key) DO UPDATE") ||
		!strings.Contains(db.stmts[1].sql, "ON CONFLICT (issuer, observed_on) DO UPDATE") {
		t.Errorf("statements are not the upserts of one row: %q, %q", db.stmts[0].sql, db.stmts[1].sql)
	}
	for _, st := range db.stmts {
		if last := st.args[len(st.args)-1]; last != "" {
			t.Errorf("writtenBy = %v, want empty", last)
		}
	}
}

func TestReaderTakesHighestAttempt(t *testing.T) {
	t.Parallel()
	db := newFake()
	s := &earningsStore{db: db}
	ctx := context.Background()
	for _, r := range []contracts.EarningsDate{
		row("2026-11-10", 1, "absent"),
		row("2026-11-09", 2, "listed"),
		row("2026-11-09", 1, "error"),
		row("2026-11-12", 1, "absent"),
	} {
		db.put([]any{r.ObservedOn.Format(time.DateOnly), r.Issuer, r.Attempt, r.Symbol, string(r.Market), string(r.Status),
			dayText(r.ReportDate), dayText(r.FiscalPeriodEnd), r.TimeOfDay, r.Estimate, r.ErrorCode, string(r.Source), r.ReceivedAt})
	}
	got, err := s.EarningsDates(ctx, issuer, "2026-11-09", "2026-11-10")
	if err != nil || len(got) != 2 {
		t.Fatalf("%d rows, %v; want 2 (day 12 is out of range)", len(got), err)
	}
	if got[0].ObservedOn.Format(time.DateOnly) != "2026-11-09" || got[0].Attempt != 2 || got[0].Status != "listed" {
		t.Errorf("first = %+v, want day 9 attempt 2 listed", got[0])
	}
	if got[1].ObservedOn.Format(time.DateOnly) != "2026-11-10" {
		t.Errorf("second = %+v, want day 10", got[1])
	}
}

func TestRowRoundTrip(t *testing.T) {
	t.Parallel()
	s := &earningsStore{db: newFake()}
	ctx := context.Background()
	in := row("2026-11-09", 1, "listed")
	if err := s.InsertEarningsDate(ctx, in); err != nil {
		t.Fatal(err)
	}
	got, err := s.EarningsDates(ctx, issuer, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 {
		t.Fatalf("%v, %v", got, err)
	}
	g := got[0]
	if g.ReportDate == nil || g.ReportDate.Format(time.DateOnly) != "2026-11-20" || g.Estimate == nil || *g.Estimate != "1.25" ||
		g.TimeOfDay != "post-market" || g.FiscalPeriodEnd != nil || g.Source != "alphavantage" || g.Market != "US" ||
		!g.ReceivedAt.Equal(in.ReceivedAt) || g.Symbol != "AAPL" {
		t.Errorf("row = %+v", g)
	}
}

func TestEmptyIsEmptySlice(t *testing.T) {
	t.Parallel()
	s := &earningsStore{db: newFake()}
	got, err := s.EarningsDates(context.Background(), issuer, "2026-11-09", "2026-11-12")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got %v, %v; want an empty slice", got, err)
	}
}

func TestBadInputIsPermanentAndErrorsAreRetryable(t *testing.T) {
	t.Parallel()
	db := newFake()
	s := &earningsStore{db: db}
	ctx := context.Background()
	kind := func(err error) contracts.ErrorKind {
		var te contracts.TypedError
		if !errors.As(err, &te) {
			t.Fatalf("not typed: %v", err)
		}
		return te.Kind()
	}
	noIssuer := row("2026-11-09", 1, "absent")
	noIssuer.Issuer = ""
	if err := s.InsertEarningsDate(ctx, noIssuer); kind(err) != contracts.Permanent || db.execs != 0 {
		t.Errorf("empty issuer: %v, %d execs", err, db.execs)
	}
	if err := s.InsertEarningsDate(ctx, row("2026-11-09", 0, "absent")); kind(err) != contracts.Permanent || db.execs != 0 {
		t.Errorf("attempt 0: %v, %d execs", err, db.execs)
	}
	if _, err := s.EarningsDates(ctx, issuer, "yesterday", "2026-11-09"); kind(err) != contracts.Permanent {
		t.Errorf("bad from: %v", err)
	}
	if _, err := s.EarningsDates(ctx, issuer, "2026-11-10", "2026-11-09"); kind(err) != contracts.Permanent {
		t.Errorf("reversed range: %v", err)
	}
	db.failed = errors.New("down")
	if err := s.InsertEarningsDate(ctx, row("2026-11-09", 1, "absent")); kind(err) != contracts.Retryable {
		t.Errorf("insert error: %v", err)
	}
	if _, err := s.EarningsDates(ctx, issuer, "2026-11-09", "2026-11-09"); kind(err) != contracts.Retryable {
		t.Errorf("select error: %v", err)
	}
}

func TestUnknownEnumInViewRefused(t *testing.T) {
	t.Parallel()
	db := newFake()
	bad := row("2026-11-09", 1, "absent")
	bad.Source = "fmp"
	if err := (&earningsStore{db: db}).InsertEarningsDate(context.Background(), bad); err != nil {
		t.Fatal(err)
	}
	if _, err := (&earningsStore{db: db}).EarningsDates(context.Background(), issuer, "2026-11-09", "2026-11-09"); err == nil {
		t.Error("a source outside the dataset enum was returned")
	}
}

func TestInsertEarningsConstraintViolationIsPermanent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want contracts.ErrorKind
	}{
		{"not null", &pgconn.PgError{Code: "23502"}, contracts.Permanent},
		{"foreign key", &pgconn.PgError{Code: "23503"}, contracts.Permanent},
		{"unique", &pgconn.PgError{Code: "23505"}, contracts.Permanent},
		{"check", &pgconn.PgError{Code: "23514"}, contracts.Permanent},
		{"wrapped check", fmt.Errorf("x: %w", &pgconn.PgError{Code: "23514"}), contracts.Permanent},
		{"serialization", &pgconn.PgError{Code: "40001"}, contracts.Retryable},
		{"deadlock", &pgconn.PgError{Code: "40P01"}, contracts.Retryable},
		{"connection", &pgconn.PgError{Code: "08006"}, contracts.Retryable},
		{"plain", errors.New("boom"), contracts.Retryable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			db := newFake()
			db.failed = c.err
			err := (&earningsStore{db: db}).InsertEarningsDate(context.Background(), row("2026-11-09", 1, "absent"))
			var te contracts.TypedError
			if !errors.As(err, &te) || te.Kind() != c.want {
				t.Fatalf("got %v, want kind %s", err, c.want)
			}
			if !errors.Is(err, c.err) || !strings.HasPrefix(err.Error(), "events: upsert earnings: ") {
				t.Errorf("cause or text lost: %v", err)
			}
		})
	}
}
