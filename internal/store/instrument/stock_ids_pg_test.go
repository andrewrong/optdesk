//go:build pg

package instrument

import (
	"context"
	"testing"
)

func TestStockIDsPG(t *testing.T) {
	name := newNamesDatabase(t)
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
