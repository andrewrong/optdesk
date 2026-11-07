//go:build pg

package instrument

import (
	"context"
	"math/big"
	"testing"
)

// newTermsDatabase is newDatabase with 0009 applied on top of 0001 to 0008, and two instruments seeded by the owner.
func newTermsDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	applyMigration(t, connURL(t, name, ""), "0009_master_option_terms.sql")
	owner := connectAs(t, name, "")
	for range 2 {
		if _, err := owner.Exec(context.Background(), `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ('option', 'SPY C', 'USD', 'XNYS', 'XNYS', 'active', 0.01, 100, now())`); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

const insertTerms = `INSERT INTO master.option_terms (instrument_id, "right", strike, strike_currency, multiplier, expiration_date,
  exercise_style, settlement_type, valid_from, valid_to)
VALUES ($1, 'C', 450.00, 'USD', 100, '2026-12-18', 'american', 'physical', '2026-11-01T00:00:00Z', $2)`

func TestOptionTermsConstraintsPG(t *testing.T) {
	name := newTermsDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	mustFail(t, owner, insertTerms, int64(999999), nil)
	mustFail(t, owner, `INSERT INTO master.option_terms (instrument_id, "right", strike, strike_currency, multiplier, expiration_date,
  exercise_style, settlement_type, valid_from) VALUES (1, 'X', 1, 'USD', 100, '2026-12-18', 'american', 'physical', now())`)
	if _, err := owner.Exec(ctx, insertTerms, int64(1), nil); err != nil {
		t.Fatalf("first current row: %v", err)
	}
	mustFail(t, owner, insertTerms, int64(1), nil) // a second current row of one instrument
	if _, err := owner.Exec(ctx, insertTerms, int64(1), "2026-11-02T00:00:00Z"); err != nil {
		t.Fatalf("closed row: %v", err)
	}
}

func TestOptionTermsReaderPG(t *testing.T) {
	name := newTermsDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	for _, vt := range []any{"2026-11-02T00:00:00Z", nil} { // a closed version, then the current one
		if _, err := owner.Exec(ctx, insertTerms, int64(1), vt); err != nil {
			t.Fatal(err)
		}
	}
	s, err := OpenOptionTerms(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	got, err := s.OptionTerms(ctx, []int64{2, 1})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	g := got[0]
	strike, _ := new(big.Rat).SetString(g.Strike)
	if g.InstrumentID != 1 || g.Right != "C" || strike.Cmp(big.NewRat(450, 1)) != 0 || g.StrikeCurrency != "USD" ||
		g.ExpirationDate != "2026-12-18" || g.ExerciseStyle != "american" || g.SettlementType != "physical" {
		t.Fatalf("terms = %+v", g)
	}
}

func TestOptionTermsGrantsPG(t *testing.T) {
	name := newTermsDatabase(t)
	owner := connectAs(t, name, "")
	if _, err := owner.Exec(context.Background(), insertTerms, int64(1), nil); err != nil {
		t.Fatal(err)
	}
	pages := connectAs(t, name, "ot_ro_pages")
	var n int
	if err := pages.QueryRow(context.Background(), `SELECT count(*) FROM rd.option_terms_r1`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("view rows = %d, %v", n, err)
	}
	mustFail(t, pages, `SELECT count(*) FROM master.option_terms`)
	mustFail(t, pages, insertTerms, int64(2), nil)
	mustFail(t, pages, `INSERT INTO rd.option_terms_r1 (instrument_id) VALUES (2)`)
}
