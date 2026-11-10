package fundamental

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

// fakeRow is one stored row in the columns of the table.
type fakeRow struct {
	source, subject, doc, taxonomy, concept, unit string
	startKey, end, filed                          string
	currency, value, form, sha                    string
	retrieved                                     time.Time
}

// fakeDB keeps the rows by the eight-column key, as the table with ON CONFLICT DO NOTHING would; its read is
// the view: period_start_key 0001-01-01 comes back as NULL.
type fakeDB struct {
	rows   map[[8]string]fakeRow
	order  [][8]string
	execs  int
	failed error
}

func newFake() *fakeDB { return &fakeDB{rows: map[[8]string]fakeRow{}} }

func (f *fakeDB) ExecAll(_ context.Context, stmts []stmt) error {
	if f.failed != nil {
		return f.failed
	}
	for _, s := range stmts {
		f.execs++
		a := s.args
		k := [8]string{a[0].(string), a[1].(string), a[2].(string), a[3].(string), a[4].(string), a[5].(string), a[6].(string), a[7].(string)}
		if _, ok := f.rows[k]; ok {
			continue
		}
		f.rows[k] = fakeRow{k[0], k[1], k[2], k[3], k[4], k[5], k[6], k[7], "", a[8].(string), a[9].(string), a[10].(string), a[13].(string), a[12].(time.Time)}
		r := f.rows[k]
		r.filed = a[11].(string)
		f.rows[k] = r
		f.order = append(f.order, k)
	}
	return nil
}

func (f *fakeDB) Select(_ context.Context, _ string, args []any, scan func(scanner) error) error {
	if f.failed != nil {
		return f.failed
	}
	for _, k := range f.order {
		r := f.rows[k]
		if r.source == args[0].(string) && r.subject == args[1].(string) && r.filed <= args[2].(string) {
			if err := scan(rowScanner{r}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

type rowScanner struct{ r fakeRow }

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func (s rowScanner) Scan(dest ...any) error {
	r := s.r
	var start *time.Time
	if r.startKey != noStart {
		t := day(r.startKey)
		start = &t
	}
	*dest[0].(*string), *dest[1].(*string), *dest[2].(*string) = r.source, r.subject, r.doc
	*dest[3].(*string), *dest[4].(*string), *dest[5].(*string) = r.taxonomy, r.concept, r.unit
	*dest[6].(**time.Time) = start
	*dest[7].(*time.Time) = day(r.end)
	*dest[8].(*string), *dest[9].(*string), *dest[10].(*string) = r.currency, r.value, r.form
	*dest[11].(*time.Time) = day(r.filed)
	*dest[12].(*time.Time) = r.retrieved
	*dest[13].(*string) = r.sha
	return nil
}

func fact(doc, concept, start, end, filed string) contracts.FundamentalFact {
	f := contracts.FundamentalFact{
		Source: "sec", Subject: "cik:0000320193", DocID: doc, Taxonomy: "us-gaap", Concept: concept, Unit: "USD",
		Currency: "USD", Value: "12.5", Form: "10-K", RawSha256: "ab",
		PeriodEnd: types.SerializableDate{Time: day(end)}, Filed: types.SerializableDate{Time: day(filed)},
		RetrievedAt: time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC),
	}
	if start != "" {
		f.PeriodStart = &types.SerializableDate{Time: day(start)}
	}
	return f
}

func TestInsertTwiceAddsNothing(t *testing.T) {
	t.Parallel()
	db := newFake()
	s := &factStore{db: db}
	rows := []contracts.FundamentalFact{
		fact("a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01"),
		fact("a1", "Assets", "", "2024-12-31", "2025-02-01"),
	}
	for i := 0; i < 2; i++ {
		if err := s.InsertFacts(context.Background(), rows); err != nil {
			t.Fatal(err)
		}
	}
	if len(db.rows) != 2 {
		t.Fatalf("%d rows after two inserts, want 2", len(db.rows))
	}
}

func TestInstantFactStoredWithStartKey(t *testing.T) {
	t.Parallel()
	db := newFake()
	s := &factStore{db: db}
	if err := s.InsertFacts(context.Background(), []contracts.FundamentalFact{fact("a1", "Assets", "", "2024-12-31", "2025-02-01")}); err != nil {
		t.Fatal(err)
	}
	for k := range db.rows {
		if k[6] != "0001-01-01" {
			t.Errorf("period_start_key = %q, want 0001-01-01", k[6])
		}
	}
}

func TestFactsFiledByRoundTrip(t *testing.T) {
	t.Parallel()
	s := &factStore{db: newFake()}
	ctx := context.Background()
	dur := fact("a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01")
	inst := fact("a1", "Assets", "", "2024-12-31", "2025-02-01")
	late := fact("a2", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2026-02-01")
	if err := s.InsertFacts(ctx, []contracts.FundamentalFact{dur, inst, late}); err != nil {
		t.Fatal(err)
	}
	got, err := s.FactsFiledBy(ctx, "sec", "cik:0000320193", "2025-06-30")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d rows, %v; want 2", len(got), err)
	}
	for _, g := range got {
		switch g.Concept {
		case "Assets":
			if g.PeriodStart != nil {
				t.Errorf("instant fact read back with period_start %v, want nil", g.PeriodStart)
			}
		case "NetIncomeLoss":
			if g.PeriodStart == nil || g.PeriodStart.Format(time.DateOnly) != "2024-01-01" {
				t.Errorf("duration fact period_start = %v", g.PeriodStart)
			}
		}
		if g.Source != "sec" || g.Value != "12.5" || !g.RetrievedAt.Equal(dur.RetrievedAt) {
			t.Errorf("row = %+v", g)
		}
	}
	if all, _ := s.FactsFiledBy(ctx, "sec", "cik:0000320193", "2026-02-01"); len(all) != 3 {
		t.Errorf("by the late day: %d rows, want 3", len(all))
	}
	none, err := s.FactsFiledBy(ctx, "sec", "cik:1", "2026-02-01")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("unknown subject = %v, %v; want empty slice", none, err)
	}
}

func TestBadInputIsPermanentAndErrorsAreRetryable(t *testing.T) {
	t.Parallel()
	db := newFake()
	s := &factStore{db: db}
	ctx := context.Background()
	kind := func(err error) contracts.ErrorKind {
		var te contracts.TypedError
		if !errors.As(err, &te) {
			t.Fatalf("not typed: %v", err)
		}
		return te.Kind()
	}
	bad := fact("", "Assets", "", "2024-12-31", "2025-02-01")
	if err := s.InsertFacts(ctx, []contracts.FundamentalFact{bad}); kind(err) != contracts.Permanent || db.execs != 0 {
		t.Errorf("empty doc id: %v, %d execs", err, db.execs)
	}
	if _, err := s.FactsFiledBy(ctx, "sec", "cik:1", "yesterday"); kind(err) != contracts.Permanent {
		t.Errorf("bad cutoff: %v", err)
	}
	db.failed = errors.New("down")
	if err := s.InsertFacts(ctx, []contracts.FundamentalFact{fact("a", "Assets", "", "2024-12-31", "2025-02-01")}); kind(err) != contracts.Retryable {
		t.Errorf("insert error: %v", err)
	}
	if _, err := s.FactsFiledBy(ctx, "sec", "cik:1", "2025-01-01"); kind(err) != contracts.Retryable {
		t.Errorf("select error: %v", err)
	}
}

func TestInsertFactsConstraintViolationIsPermanent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want contracts.ErrorKind
	}{
		{"plain", errors.New("boom"), contracts.Retryable},
		{"class 40", &pgconn.PgError{Code: "40001"}, contracts.Retryable},
		{"class 08", &pgconn.PgError{Code: "08006"}, contracts.Retryable},
	}
	for _, code := range []string{"23502", "23503", "23505", "23514"} {
		cases = append(cases,
			struct {
				name string
				err  error
				want contracts.ErrorKind
			}{code, &pgconn.PgError{Code: code}, contracts.Permanent},
			struct {
				name string
				err  error
				want contracts.ErrorKind
			}{code + " wrapped", fmt.Errorf("x: %w", &pgconn.PgError{Code: code}), contracts.Permanent})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			db := newFake()
			db.failed = c.err
			err := (&factStore{db: db}).InsertFacts(context.Background(), []contracts.FundamentalFact{fact("a", "Assets", "", "2024-12-31", "2025-02-01")})
			var te contracts.TypedError
			if !errors.As(err, &te) || te.Kind() != c.want {
				t.Fatalf("err = %v, want kind %v", err, c.want)
			}
			if !strings.HasPrefix(err.Error(), "fundamental: insert") {
				t.Errorf("text = %q, want prefix fundamental: insert", err.Error())
			}
		})
	}
}

func TestUnknownSourceInViewRefused(t *testing.T) {
	t.Parallel()
	db := newFake()
	db.rows[[8]string{"fmp"}] = fakeRow{source: "fmp", subject: "cik:1", startKey: noStart, end: "2024-12-31", filed: "2025-01-01"}
	db.order = [][8]string{{"fmp"}}
	if _, err := (&factStore{db: db}).FactsFiledBy(context.Background(), "fmp", "cik:1", "2025-01-02"); err == nil {
		t.Error("a source outside the dataset enum was returned")
	}
}
