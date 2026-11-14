package events

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// recDB records the statements of every Upsert call and answers with the rows each is told to return.
type recDB struct {
	calls  [][]stmt
	counts []int // rows returned per statement of a call; 1 each when shorter
	err    error
}

func (r *recDB) Upsert(_ context.Context, stmts []stmt) ([]int, error) {
	r.calls = append(r.calls, stmts)
	if r.err != nil {
		return nil, r.err
	}
	out := make([]int, len(stmts))
	for i := range out {
		out[i] = 1
		if i < len(r.counts) {
			out[i] = r.counts[i]
		}
	}
	return out, nil
}

func (r *recDB) Select(context.Context, string, []any, func(scanner) error) error { return nil }
func (r *recDB) Close() error                                                     { return nil }

func dayPtr(s string) *types.SerializableDate { return &types.SerializableDate{Time: mustDay(s)} }

func listedRow(source string, report string, fiscal *types.SerializableDate) contracts.EarningsDate {
	r := contracts.EarningsDate{
		ObservedOn: types.SerializableDate{Time: mustDay("2026-11-09")}, Issuer: issuer, Attempt: 7, Symbol: "AAPL",
		Market: "US", Status: "listed", ReportDate: dayPtr(report), FiscalPeriodEnd: fiscal,
		ReceivedAt: time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC),
	}
	// The enum type lives in the generated package, which this package does not import.
	reflect.ValueOf(&r.Source).Elem().SetString(source)
	return r
}

func TestUpsertEarningsListedSendsDataBeforeAnswer(t *testing.T) {
	t.Parallel()
	db := &recDB{}
	s := &earningsStore{db: db}
	in := listedRow("alphavantage", "2026-11-20", dayPtr("2026-09-30"))
	wc, err := s.UpsertEarnings(context.Background(), []contracts.EarningsDate{in}, "earnings:2026-11-09")
	if err != nil || wc.Written != 1 || wc.Unchanged != 0 {
		t.Fatalf("%+v, %v", wc, err)
	}
	if len(db.calls) != 1 || len(db.calls[0]) != 2 {
		t.Fatalf("calls = %d, want one call of two statements", len(db.calls))
	}
	data, answer := db.calls[0][0], db.calls[0][1]
	if !strings.Contains(data.sql, "INTO obs.earnings_date") || !strings.Contains(answer.sql, "INTO obs.earnings_check") {
		t.Fatalf("order: %q then %q; the data row must come first", data.sql, answer.sql)
	}
	if data.args[1] != "2026-09-30" || data.args[10] != "earnings:2026-11-09" {
		t.Errorf("data args = %v", data.args)
	}
	if answer.args[7] == nil || *answer.args[7].(*string) != "2026-09-30" || answer.args[9] != "earnings:2026-11-09" {
		t.Errorf("answer args = %v", answer.args)
	}
	for _, st := range db.calls[0] {
		for _, a := range st.args {
			if a == 7 {
				t.Errorf("Attempt travelled as an argument: %v", st.args)
			}
		}
	}
}

func TestUpsertEarningsAbsentSendsOnlyAnswer(t *testing.T) {
	t.Parallel()
	db := &recDB{counts: []int{0}}
	s := &earningsStore{db: db}
	r := row("2026-11-09", 1, "absent")
	wc, err := s.UpsertEarnings(context.Background(), []contracts.EarningsDate{r}, "w")
	if err != nil || wc.Written != 0 || wc.Unchanged != 1 {
		t.Fatalf("%+v, %v", wc, err)
	}
	if len(db.calls) != 1 || len(db.calls[0]) != 1 || !strings.Contains(db.calls[0][0].sql, "INTO obs.earnings_check") {
		t.Fatalf("calls = %+v, want the answer statement alone", db.calls)
	}
	if db.calls[0][0].args[7] != (*string)(nil) {
		t.Errorf("period_key = %v, want NULL", db.calls[0][0].args[7])
	}
}

func TestUpsertEmptyListMakesNoCall(t *testing.T) {
	t.Parallel()
	db := &recDB{}
	e := &earningsStore{db: db}
	d := &dividendStore{db: db}
	ctx := context.Background()
	for name, f := range map[string]func() error{
		"upsert earnings":  func() error { _, err := e.UpsertEarnings(ctx, nil, "w"); return err },
		"replay earnings":  func() error { _, err := e.ReplayEarnings(ctx, nil, "w"); return err },
		"upsert dividends": func() error { _, err := d.UpsertDividends(ctx, []contracts.DividendExpectation{}, "w"); return err },
		"replay dividends": func() error { _, err := d.ReplayDividends(ctx, nil, "w"); return err },
	} {
		if err := f(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(db.calls) != 0 {
		t.Errorf("%d calls for empty lists", len(db.calls))
	}
}

func TestUpsertConstraintViolationPermanent(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		want contracts.ErrorKind
	}{
		{"foreign key", &pgconn.PgError{Code: "23503"}, contracts.Permanent},
		{"check", &pgconn.PgError{Code: "23514"}, contracts.Permanent},
		{"serialization", &pgconn.PgError{Code: "40001"}, contracts.Retryable},
		{"plain", errors.New("down"), contracts.Retryable},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			db := &recDB{err: c.err}
			_, e1 := (&earningsStore{db: db}).UpsertEarnings(context.Background(), []contracts.EarningsDate{row("2026-11-09", 1, "absent")}, "w")
			_, e2 := (&dividendStore{db: db}).UpsertDividends(context.Background(), []contracts.DividendExpectation{divRow("2026-11-09", 1, "no_value")}, "w")
			for _, err := range []error{e1, e2} {
				var te contracts.TypedError
				if !errors.As(err, &te) || te.Kind() != c.want || !errors.Is(err, c.err) {
					t.Errorf("got %v, want kind %s with the cause kept", err, c.want)
				}
			}
		})
	}
}

func TestReplaySendsNoOverwriteOfAnswer(t *testing.T) {
	t.Parallel()
	db := &recDB{}
	e := &earningsStore{db: db}
	if _, err := e.ReplayEarnings(context.Background(), []contracts.EarningsDate{listedRow("jquants", "2026-11-20", nil)}, "w"); err != nil {
		t.Fatal(err)
	}
	d := &dividendStore{db: db}
	if _, err := d.ReplayDividends(context.Background(), []contracts.DividendExpectation{divRow("2026-11-09", 1, "present")}, "w"); err != nil {
		t.Fatal(err)
	}
	if len(db.calls) != 2 {
		t.Fatalf("%d calls", len(db.calls))
	}
	for _, call := range db.calls {
		data, answer := call[0], call[1]
		if !strings.Contains(data.sql, "DO UPDATE") {
			t.Errorf("the data row of a replay is upserted: %q", data.sql)
		}
		if !strings.Contains(answer.sql, "DO NOTHING") || strings.Contains(answer.sql, "DO UPDATE") {
			t.Errorf("the answer of a replay must not overwrite: %q", answer.sql)
		}
	}
}

func TestEarningsPeriodKeyNewByVendor(t *testing.T) {
	t.Parallel()
	// Rows after jquants/testdata/earnings_date.json and alphavantage/testdata/calendar.csv.
	rows := []contracts.EarningsDate{
		listedRow("jquants", "2026-08-04", dayPtr("2027-03-31")),
		listedRow("jquants", "2026-05-12", dayPtr("2026-03-31")),
		listedRow("jquants", "2026-02-13", dayPtr("2026-03-31")),
		listedRow("alphavantage", "2026-11-20", dayPtr("2026-09-30")),
		listedRow("owner", "2026-12-01", nil),
	}
	db := &recDB{}
	if _, err := (&earningsStore{db: db}).UpsertEarnings(context.Background(), rows, "w"); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, st := range db.calls[0] {
		if strings.Contains(st.sql, "INTO obs.earnings_date") {
			keys = append(keys, st.args[1].(string))
		}
	}
	want := []string{"2026-08-04", "2026-05-12", "2026-02-13", "2026-09-30", "2026-12-01"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("period keys = %v, want %v", keys, want)
	}
	// An Alpha Vantage row without a fiscal end falls back to its report date.
	if got := earningsPeriodKey(listedRow("alphavantage", "2026-11-20", nil)); got != "2026-11-20" {
		t.Errorf("alphavantage without a fiscal end = %s", got)
	}
}

func TestUpsertEarningsRefusesBadRows(t *testing.T) {
	t.Parallel()
	db := &recDB{}
	s := &earningsStore{db: db}
	noDate := listedRow("jquants", "2026-11-20", nil)
	noDate.ReportDate = nil
	noIssuer := row("2026-11-09", 1, "absent")
	noIssuer.Issuer = ""
	for _, r := range []contracts.EarningsDate{noDate, noIssuer} {
		_, err := s.UpsertEarnings(context.Background(), []contracts.EarningsDate{r}, "w")
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("got %v, want Permanent", err)
		}
	}
	if len(db.calls) != 0 {
		t.Errorf("%d calls for refused rows", len(db.calls))
	}
}

func TestUpsertEarningsCountsPerRow(t *testing.T) {
	t.Parallel()
	// Row 1: data row and answer both unchanged; row 2: only the answer changed.
	db := &recDB{counts: []int{0, 0, 0, 1}}
	rows := []contracts.EarningsDate{listedRow("jquants", "2026-11-20", nil), listedRow("owner", "2026-12-01", nil)}
	wc, err := (&earningsStore{db: db}).UpsertEarnings(context.Background(), rows, "w")
	if err != nil || wc.Written != 1 || wc.Unchanged != 1 {
		t.Errorf("%+v, %v; want 1 written, 1 unchanged", wc, err)
	}
}
