//go:build pg

package instrument

import (
	"context"
	"testing"
)

// newStockIDsDatabase is newNamesDatabase with the view rd.stock_id_r1 (0069) applied.
func newStockIDsDatabase(t *testing.T) string {
	t.Helper()
	name := newNamesDatabase(t)
	applyMigration(t, connURL(t, name, ""), "0069_stock_id_view.sql")
	return name
}

func TestStockIDsPG(t *testing.T) {
	name := newStockIDsDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	for _, typ := range []string{"stock", "index"} {
		if _, err := owner.Exec(ctx, `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ($1, 'DUP', 'USD', 'XNYS', 'XNYS', 'active', 0.01, 1, now())`, typ); err != nil {
			t.Fatal(err)
		}
	}
	s, err := OpenStockIDs(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	got, err := s.StockIDs(ctx, []string{"DUP", "NONE"})
	if err != nil || len(got) != 1 || got[0].Type != "stock" || got[0].Name != "DUP" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestStockIDsPGLeavesOutConidZeroAndInactive(t *testing.T) {
	name := newStockIDsDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	insert := func(nm, status, conid string) int64 {
		t.Helper()
		var id int64
		if err := owner.QueryRow(ctx, `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ('stock', $1, 'JPY', 'XTKS', 'XTKS', $2, 1, 1, now()) RETURNING instrument_id`, nm, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if conid != "" {
			if _, err := owner.Exec(ctx, `INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from) VALUES ($1, 'ibkr_conid', $2, now())`, id, conid); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	insert("JP4063", "active", "0")
	real := insert("JP4063", "active", "13905438")
	insert("GONE", "inactive", "555")
	bare := insert("BARE", "active", "")
	s, err := OpenStockIDs(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	got, err := s.StockIDs(ctx, []string{"JP4063", "GONE", "BARE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].InstrumentID != real || got[0].Name != "JP4063" || got[1].InstrumentID != bare || got[1].Name != "BARE" {
		t.Fatalf("got %+v, want JP4063 id %d and BARE id %d", got, real, bare)
	}
}
