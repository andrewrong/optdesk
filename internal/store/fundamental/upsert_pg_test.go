//go:build pg

package fundamental

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

func openUpserter(t *testing.T, name string) (fundkey.FactUpserter, interface {
	FactsFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error)
}) {
	t.Helper()
	s, err := Open(context.Background(), connURL(t, name, "ot_w_fundamental"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s.(fundkey.FactUpserter), s
}

func retrievedOf(t *testing.T, name, doc string) time.Time {
	t.Helper()
	var at time.Time
	if err := pgxAs(t, name).QueryRow(context.Background(),
		`SELECT retrieved_at FROM obs.fundamental_fact WHERE doc_id = $1`, doc).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestFactUpsertReplacesDifferingValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	u, r := openUpserter(t, name)
	first := fact("a1", "Assets", "", "2024-12-31", "2025-02-01")
	if got, err := u.UpsertFacts(ctx, []contracts.FundamentalFact{first}, "w1"); err != nil || got.Written != 1 {
		t.Fatalf("first = %+v, %v", got, err)
	}
	second := first
	second.Value = "13.5"
	second.RetrievedAt = first.RetrievedAt.Add(time.Hour)
	got, err := u.UpsertFacts(ctx, []contracts.FundamentalFact{second}, "w2")
	if err != nil || got.Written != 1 || got.Unchanged != 0 {
		t.Fatalf("second = %+v, %v; want 1 written", got, err)
	}
	if !retrievedOf(t, name, "a1").Equal(second.RetrievedAt) {
		t.Error("retrieved_at did not move")
	}
	rows, err := r.FactsFiledBy(ctx, "sec", "cik:0000320193", "2025-06-30")
	if err != nil || len(rows) != 1 || rows[0].Value != "13.5" {
		t.Errorf("rows = %+v, %v; want the one replaced row", rows, err)
	}
}

func TestFactUpsertEqualKeepsRetrievedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	u, _ := openUpserter(t, name)
	first := fact("a1", "Assets", "", "2024-12-31", "2025-02-01")
	if _, err := u.UpsertFacts(ctx, []contracts.FundamentalFact{first}, "w1"); err != nil {
		t.Fatal(err)
	}
	again := first
	again.RetrievedAt = first.RetrievedAt.Add(time.Hour)
	got, err := u.UpsertFacts(ctx, []contracts.FundamentalFact{again}, "w2")
	if err != nil || got.Written != 0 || got.Unchanged != 1 {
		t.Fatalf("got %+v, %v; want 1 unchanged", got, err)
	}
	if !retrievedOf(t, name, "a1").Equal(first.RetrievedAt) {
		t.Error("retrieved_at moved for an equal fact")
	}
}

func TestRestatingFilingIsSecondRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	u, r := openUpserter(t, name)
	orig := fact("a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01")
	restated := fact("a2", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2026-02-01")
	restated.Value = "99"
	if got, err := u.UpsertFacts(ctx, []contracts.FundamentalFact{orig, restated}, "w"); err != nil || got.Written != 2 {
		t.Fatalf("got %+v, %v; want 2 written", got, err)
	}
	if n := count(t, name); n != 2 {
		t.Errorf("%d rows, want 2", n)
	}
	early, err := r.FactsFiledBy(ctx, "sec", "cik:0000320193", "2025-06-30")
	if err != nil || len(early) != 1 || early[0].DocID != "a1" {
		t.Errorf("early = %+v, %v; want only the first filing", early, err)
	}
}

func TestFactValueCheckRefusesNonDecimal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	u, _ := openUpserter(t, name)
	bad := fact("a1", "Assets", "", "2024-12-31", "2025-02-01")
	bad.Value = "twelve"
	_, err := u.UpsertFacts(ctx, []contracts.FundamentalFact{bad}, "w")
	var te contracts.TypedError
	if err == nil || !asTyped(err, &te) || te.Kind() != contracts.Permanent {
		t.Errorf("err = %v, want Permanent", err)
	}
	if n := count(t, name); n != 0 {
		t.Errorf("%d rows, want 0", n)
	}
}

func TestFactsRoleCannotDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	u, _ := openUpserter(t, name)
	if _, err := u.UpsertFacts(ctx, []contracts.FundamentalFact{fact("a1", "Assets", "", "2024-12-31", "2025-02-01")}, "w"); err != nil {
		t.Fatal(err)
	}
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_w_fundamental"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, `DELETE FROM obs.fundamental_fact`); err == nil {
		t.Error("the writer deleted a fact")
	}
	if n := count(t, name); n != 1 {
		t.Errorf("%d rows, want 1", n)
	}
}

func asTyped(err error, te *contracts.TypedError) bool { return errors.As(err, te) }
