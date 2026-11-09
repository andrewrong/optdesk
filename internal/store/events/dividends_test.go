package events

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const divInstrument = 265598

// divDB keeps the rows by (session_day, instrument_id, attempt) as the table does with ON CONFLICT DO NOTHING and
// answers a select as the view query does: the instrument in the range, ordered by day then attempt.
type divDB struct {
	rows   map[[3]string][]any
	failed error
	execs  int
}

func (f *divDB) Exec(_ context.Context, _ string, args ...any) error {
	if f.failed != nil {
		return f.failed
	}
	f.execs++
	k := [3]string{args[0].(string), string(rune('0' + args[1].(int)%10)), string(rune('0' + args[2].(int)))}
	if _, ok := f.rows[k]; !ok {
		f.rows[k] = args
	}
	return nil
}

func (f *divDB) Select(_ context.Context, _ string, args []any, scan func(scanner) error) error {
	if f.failed != nil {
		return f.failed
	}
	var picked [][]any
	for _, a := range f.rows {
		if a[1].(int) == args[0].(int) && a[0].(string) >= args[1].(string) && a[0].(string) <= args[2].(string) {
			picked = append(picked, a)
		}
	}
	sort.Slice(picked, func(i, j int) bool {
		if picked[i][0].(string) != picked[j][0].(string) {
			return picked[i][0].(string) < picked[j][0].(string)
		}
		return picked[i][2].(int) < picked[j][2].(int)
	})
	for _, a := range picked {
		if err := scan(divScanner{a}); err != nil {
			return err
		}
	}
	return nil
}

func (f *divDB) Close() error { return nil }

type divScanner struct{ a []any }

// Scan hands back the columns of the insert arguments in the order of the view.
func (s divScanner) Scan(dest ...any) error {
	a := s.a
	*dest[0].(*time.Time) = mustDay(a[0].(string))
	*dest[1].(*int) = a[1].(int)
	*dest[2].(*int) = a[2].(int)
	*dest[3].(*string), *dest[4].(*string) = a[3].(string), a[4].(string)
	*dest[5].(**string), *dest[6].(**string), *dest[7].(**string) = a[5].(*string), a[6].(*string), a[7].(*string)
	if v := a[8].(*string); v != nil {
		t := mustDay(*v)
		*dest[8].(**time.Time) = &t
	}
	*dest[9].(*string), *dest[10].(*string), *dest[11].(*string) = a[9].(string), a[10].(string), a[11].(string)
	*dest[12].(*time.Time) = a[12].(time.Time)
	return nil
}

func newDivStore() (*dividendStore, *divDB) {
	db := &divDB{rows: map[[3]string][]any{}}
	return &dividendStore{db: db}, db
}

func divRow(day string, attempt int, status string) contracts.DividendExpectation {
	r := contracts.DividendExpectation{
		SessionDay: types.SerializableDate{Time: mustDay(day)}, InstrumentID: divInstrument, Attempt: attempt,
		Issuer: issuer, Status: "no_value", Currency: "USD", Source: "ibkr_tick_456",
		ReceivedAt: time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC),
	}
	switch status {
	case "error":
		r.Status, r.ErrorCode = "error", "timeout"
	case "present":
		r.Status = "present"
		past, next12, amt := "0.96", "1.00", "0.25"
		r.Past12M, r.Next12M, r.NextAmount = &past, &next12, &amt
		r.NextExDate = &types.SerializableDate{Time: mustDay("2026-11-14")}
	}
	return r
}

func TestDividendWriterKeepsExistingKey(t *testing.T) {
	t.Parallel()
	s, _ := newDivStore()
	ctx := context.Background()
	if err := s.InsertDividend(ctx, divRow("2026-11-09", 1, "error")); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertDividend(ctx, divRow("2026-11-09", 1, "no_value")); err != nil {
		t.Fatalf("second insert of one key: %v", err)
	}
	got, err := s.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].Status != "error" || got[0].ErrorCode != "timeout" {
		t.Fatalf("rows = %+v, %v; want the first row only", got, err)
	}
}

func TestDividendReaderTakesHighestAttempt(t *testing.T) {
	t.Parallel()
	s, _ := newDivStore()
	ctx := context.Background()
	for _, r := range []contracts.DividendExpectation{
		divRow("2026-11-10", 1, "no_value"), divRow("2026-11-09", 2, "present"),
		divRow("2026-11-09", 1, "error"), divRow("2026-11-12", 1, "no_value"),
	} {
		if err := s.InsertDividend(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-10")
	if err != nil || len(got) != 2 {
		t.Fatalf("%d rows, %v; want 2 (day 12 is out of range)", len(got), err)
	}
	if got[0].SessionDay.Format(time.DateOnly) != "2026-11-09" || got[0].Attempt != 2 || got[0].Status != "present" ||
		got[1].SessionDay.Format(time.DateOnly) != "2026-11-10" {
		t.Errorf("rows = %+v", got)
	}
	other, err := s.Dividends(ctx, 1, "2026-11-09", "2026-11-12")
	if err != nil || other == nil || len(other) != 0 {
		t.Errorf("other instrument = %v, %v; want an empty slice", other, err)
	}
}

func TestDividendRowRoundTrip(t *testing.T) {
	t.Parallel()
	s, _ := newDivStore()
	ctx := context.Background()
	in := divRow("2026-11-09", 1, "present")
	if err := s.InsertDividend(ctx, in); err != nil {
		t.Fatal(err)
	}
	got, err := s.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 {
		t.Fatalf("%v, %v", got, err)
	}
	g := got[0]
	if g.NextExDate == nil || g.NextExDate.Format(time.DateOnly) != "2026-11-14" || g.NextAmount == nil || *g.NextAmount != "0.25" ||
		*g.Past12M != "0.96" || *g.Next12M != "1.00" || g.Currency != "USD" || g.Source != "ibkr_tick_456" ||
		g.Issuer != issuer || !g.ReceivedAt.Equal(in.ReceivedAt) {
		t.Errorf("row = %+v", g)
	}
	nv := divRow("2026-11-10", 1, "no_value")
	if err := s.InsertDividend(ctx, nv); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Dividends(ctx, divInstrument, "2026-11-10", "2026-11-10")
	if len(got) != 1 || got[0].NextExDate != nil || got[0].NextAmount != nil {
		t.Errorf("no_value row = %+v; want no ex-date and no amount", got)
	}
}

func TestDividendBadInputIsPermanentAndErrorsAreRetryable(t *testing.T) {
	t.Parallel()
	s, db := newDivStore()
	ctx := context.Background()
	kind := func(err error) contracts.ErrorKind {
		var te contracts.TypedError
		if !errors.As(err, &te) {
			t.Fatalf("not typed: %v", err)
		}
		return te.Kind()
	}
	noID := divRow("2026-11-09", 1, "no_value")
	noID.InstrumentID = 0
	if err := s.InsertDividend(ctx, noID); kind(err) != contracts.Permanent || db.execs != 0 {
		t.Errorf("instrument 0: %v, %d execs", err, db.execs)
	}
	if err := s.InsertDividend(ctx, divRow("2026-11-09", 0, "no_value")); kind(err) != contracts.Permanent || db.execs != 0 {
		t.Errorf("attempt 0: %v, %d execs", err, db.execs)
	}
	if _, err := s.Dividends(ctx, divInstrument, "yesterday", "2026-11-09"); kind(err) != contracts.Permanent {
		t.Errorf("bad from: %v", err)
	}
	if _, err := s.Dividends(ctx, divInstrument, "2026-11-10", "2026-11-09"); kind(err) != contracts.Permanent {
		t.Errorf("reversed range: %v", err)
	}
	db.failed = errors.New("down")
	if err := s.InsertDividend(ctx, divRow("2026-11-09", 1, "no_value")); kind(err) != contracts.Retryable {
		t.Errorf("insert error: %v", err)
	}
	if _, err := s.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-09"); kind(err) != contracts.Retryable {
		t.Errorf("select error: %v", err)
	}
}

func TestDividendUnknownEnumInViewRefused(t *testing.T) {
	t.Parallel()
	s, _ := newDivStore()
	bad := divRow("2026-11-09", 1, "no_value")
	bad.Source = "fmp"
	if err := s.InsertDividend(context.Background(), bad); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Dividends(context.Background(), divInstrument, "2026-11-09", "2026-11-09"); err == nil {
		t.Error("a source outside the dataset enum was returned")
	}
}
