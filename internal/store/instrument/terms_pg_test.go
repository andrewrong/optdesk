//go:build pg

package instrument

import (
	"context"
	"math/big"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

// newTermsDatabase is newDatabase with 0009 to 0011 applied on top of 0001 to 0008, and two instruments seeded by the owner.
func newTermsDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	applyMigration(t, connURL(t, name, ""), "0009_master_option_terms.sql")
	applyMigration(t, connURL(t, name, ""), "0010_instrument_names_view.sql")
	applyMigration(t, connURL(t, name, ""), "0011_option_terms_writer.sql")
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

// TestResolveWritesTermsPG resolves a new option as ot_w_master: the instrument and its current terms row appear
// together, read back as the owner; a second resolve of the same key writes no second row; a stock gets no row.
func TestResolveWritesTermsPG(t *testing.T) {
	name := newTermsDatabase(t)
	ctx := context.Background()
	s, err := Open(ctx, connURL(t, name, "ot_w_master"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// the writer role may run the terms insert as it is written; the resolve below swallows a refusal, this shows it
	wc := connectAs(t, name, "ot_w_master")
	tx, err := wc.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, insertTermsSQL, int64(1), "C", "450", "USD", "100", "2026-11-20"); err != nil {
		t.Fatalf("terms insert as ot_w_master: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	spec := store.InstrumentSpec{Type: "option", Name: "SPY 20261120 C 450", Currency: "USD", MIC: "XNYS", CalendarID: "XNYS",
		PriceIncrement: "0.01", LotSize: "100"}
	key := store.Identifier{Scheme: "ibkr_conid", Value: "777"}
	extra := []store.Identifier{{Scheme: "osi", Value: "SPY   261120C00450500"}}
	id, err := s.ResolveInstrument(ctx, key, spec, extra)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := s.ResolveInstrument(ctx, key, spec, extra); err != nil || again != id {
		t.Fatalf("second resolve %d, %v", again, err)
	}
	stock := spec
	stock.Type = "stock"
	if _, err := s.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: "778"}, stock,
		[]store.Identifier{{Scheme: "osi", Value: "SPY   261120C00450500"}}); err == nil {
		t.Fatal("a second instrument took the osi of the first")
	}
	owner := connectAs(t, name, "")
	var right, cur, style, settle, expiry string
	var strike, mult big.Rat
	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM master.option_terms WHERE instrument_id = $1`, id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows = %d, %v", n, err)
	}
	var strikeText, multText string
	if err := owner.QueryRow(ctx, `SELECT "right", strike::text, strike_currency, multiplier::text, expiration_date::text, exercise_style,
  settlement_type FROM master.option_terms WHERE instrument_id = $1 AND valid_to IS NULL`, id).
		Scan(&right, &strikeText, &cur, &multText, &expiry, &style, &settle); err != nil {
		t.Fatal(err)
	}
	strike.SetString(strikeText)
	mult.SetString(multText)
	if right != "C" || strike.Cmp(big.NewRat(4505, 10)) != 0 || cur != "USD" || mult.Cmp(big.NewRat(100, 1)) != 0 ||
		expiry != "2026-11-20" || style != "unknown" || settle != "unknown" {
		t.Fatalf("terms = %s %s %s %s %s %s %s", right, strikeText, cur, multText, expiry, style, settle)
	}
	// a stock with a new osi is created without terms
	sid, err := s.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: "779"}, stock,
		[]store.Identifier{{Scheme: "osi", Value: "SPY   261120P00450500"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM master.option_terms WHERE instrument_id = $1`, sid).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stock terms rows = %d, %v", n, err)
	}
}
