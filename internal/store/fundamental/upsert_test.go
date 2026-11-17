package fundamental

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeReturning is the fake of the upsert seam: it records the statements of each call and answers that every
// statement returned a row, except the ones whose index is in unchanged.
type fakeReturning struct {
	*fakeDB
	calls     [][]stmt
	unchanged map[int]bool
	err       error
}

func (r *fakeReturning) ExecReturning(_ context.Context, stmts []stmt) ([]bool, error) {
	r.calls = append(r.calls, stmts)
	if r.err != nil {
		return nil, r.err
	}
	out := make([]bool, len(stmts))
	for i := range stmts {
		out[i] = !r.unchanged[i]
	}
	return out, nil
}

func newFakeReturning() *fakeReturning {
	return &fakeReturning{fakeDB: newFake(), unchanged: map[int]bool{}}
}

func TestUpsertFactsOneStatementPerFact(t *testing.T) {
	t.Parallel()
	db := newFakeReturning()
	db.unchanged[1] = true
	s := &factStore{db: db}
	rows := []contracts.FundamentalFact{
		fact("a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01"),
		fact("a1", "Assets", "", "2024-12-31", "2025-02-01"),
		fact("a2", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2026-02-01"),
	}
	got, err := s.UpsertFacts(context.Background(), rows, "fundamentals:2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(db.calls) != 1 || len(db.calls[0]) != 3 {
		t.Fatalf("calls = %d, statements = %v; want one call of 3", len(db.calls), db.calls)
	}
	if got.Written != 2 || got.Unchanged != 1 {
		t.Errorf("count = %+v, want 2 written, 1 unchanged", got)
	}
	st := db.calls[0][0]
	for _, want := range []string{
		"ON CONFLICT (source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end) DO UPDATE",
		"IS DISTINCT FROM", "RETURNING",
	} {
		if !strings.Contains(st.sql, want) {
			t.Errorf("statement lacks %q:\n%s", want, st.sql)
		}
	}
	if strings.Contains(strings.SplitN(st.sql, "WHERE", 2)[1], "retrieved_at") {
		t.Error("retrieved_at is compared")
	}
	if wb := st.args[len(st.args)-1]; wb != "fundamentals:2026-11-14" {
		t.Errorf("written_by arg = %v", wb)
	}
}

func TestUpsertFactsAbsentStartIsSentAsNoStart(t *testing.T) {
	t.Parallel()
	db := newFakeReturning()
	s := &factStore{db: db}
	if _, err := s.UpsertFacts(context.Background(), []contracts.FundamentalFact{
		fact("a1", "Assets", "", "2024-12-31", "2025-02-01"),
		fact("a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01"),
	}, "w"); err != nil {
		t.Fatal(err)
	}
	if got := db.calls[0][0].args[6]; got != noStart {
		t.Errorf("instant fact start = %v, want %s", got, noStart)
	}
	if got := db.calls[0][1].args[6]; got != "2024-01-01" {
		t.Errorf("duration fact start = %v", got)
	}
}

func TestUpsertFactsMissingKeyPartPermanent(t *testing.T) {
	t.Parallel()
	for _, mod := range []func(*contracts.FundamentalFact){
		func(f *contracts.FundamentalFact) { f.Subject = "" },
		func(f *contracts.FundamentalFact) { f.DocID = "" },
		func(f *contracts.FundamentalFact) { f.Source = "" },
	} {
		db := newFakeReturning()
		f := fact("a1", "Assets", "", "2024-12-31", "2025-02-01")
		mod(&f)
		_, err := (&factStore{db: db}).UpsertFacts(context.Background(), []contracts.FundamentalFact{f}, "w")
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("err = %v, want Permanent", err)
		}
		if len(db.calls) != 0 {
			t.Error("the database was called")
		}
	}
}

func TestUpsertFactsEmptyListMakesNoCall(t *testing.T) {
	t.Parallel()
	db := newFakeReturning()
	got, err := (&factStore{db: db}).UpsertFacts(context.Background(), nil, "w")
	if err != nil || got.Written != 0 || got.Unchanged != 0 || len(db.calls) != 0 {
		t.Errorf("got %+v, %v, %d calls; want zero and no call", got, err, len(db.calls))
	}
}

// periodFake answers like the statement: a fact is a Written one when no row is stored under its key or the
// compared columns, fy, fp and frame among them, differ from the stored ones.
type periodFake struct {
	*fakeDB
	stored map[[8]string][5]any
}

func (p *periodFake) ExecReturning(_ context.Context, stmts []stmt) ([]bool, error) {
	out := make([]bool, len(stmts))
	for i, s := range stmts {
		a := s.args
		var k [8]string
		for j := range k {
			k[j] = a[j].(string)
		}
		fy := 0
		if v := a[14].(*int); v != nil {
			fy = *v
		}
		cur := [5]any{a[10], a[13], fy, a[15], a[16]}
		old, ok := p.stored[k]
		out[i] = !ok || old != cur
		p.stored[k] = cur
	}
	return out, nil
}

func periodFact(fy *int, fp string) contracts.FundamentalFact {
	f := fact("a1", "NetIncomeLoss", "2025-04-01", "2025-06-30", "2025-08-01")
	f.Fy, f.Fp, f.Frame = fy, fp, "CY2025Q2"
	return f
}

func TestUpsertFillsPeriodFieldsOfStoredRow(t *testing.T) {
	t.Parallel()
	db := &periodFake{fakeDB: newFake(), stored: map[[8]string][5]any{}}
	s := &factStore{db: db}
	ctx := context.Background()
	if _, err := s.UpsertFacts(ctx, []contracts.FundamentalFact{periodFact(nil, "")}, "w1"); err != nil {
		t.Fatal(err)
	}
	fy := 2025
	got, err := s.UpsertFacts(ctx, []contracts.FundamentalFact{periodFact(&fy, "Q2")}, "w2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Written != 1 || got.Unchanged != 0 {
		t.Errorf("count = %+v, want the stored row filled (1 written)", got)
	}
	for _, want := range []string{"f.fy", "f.fp", "f.frame", "EXCLUDED.fy", "EXCLUDED.fp", "EXCLUDED.frame", "fy = EXCLUDED.fy"} {
		if !strings.Contains(upsertFactSQL, want) {
			t.Errorf("statement lacks %q", want)
		}
	}
}

func TestUpsertSamePeriodFieldsUnchanged(t *testing.T) {
	t.Parallel()
	db := &periodFake{fakeDB: newFake(), stored: map[[8]string][5]any{}}
	s := &factStore{db: db}
	ctx := context.Background()
	fy := 2025
	if _, err := s.UpsertFacts(ctx, []contracts.FundamentalFact{periodFact(&fy, "Q2")}, "w1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.UpsertFacts(ctx, []contracts.FundamentalFact{periodFact(&fy, "Q2")}, "w2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Written != 0 || got.Unchanged != 1 {
		t.Errorf("count = %+v, want 1 unchanged", got)
	}
}

func TestFactsWithPeriodFiledByReturnsPeriodFields(t *testing.T) {
	t.Parallel()
	db := newFake()
	s := &factStore{db: db}
	fy := 2025
	f := fact("a1", "NetIncomeLoss", "2025-04-01", "2025-06-30", "2025-08-01")
	f.Fy, f.Fp, f.Frame = &fy, "Q2", "CY2025Q2"
	if err := s.InsertFacts(context.Background(), []contracts.FundamentalFact{f}); err != nil {
		t.Fatal(err)
	}
	got, err := s.FactsWithPeriodFiledBy(context.Background(), "sec", "cik:0000320193", "2025-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Fy == nil || *got[0].Fy != 2025 || got[0].Fp != "Q2" || got[0].Frame != "CY2025Q2" {
		t.Fatalf("got %+v, want fy 2025, fp Q2, frame CY2025Q2", got)
	}
	if !strings.Contains(db.lastSQL, "rd.fundamental_fact_r2") {
		t.Errorf("sql does not read r2:\n%s", db.lastSQL)
	}
	if _, err := s.FactsWithPeriodFiledBy(context.Background(), "sec", "x", "bad"); err == nil {
		t.Error("bad cutoff accepted")
	}
}
