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
