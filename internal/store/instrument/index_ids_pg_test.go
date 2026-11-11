//go:build pg

package instrument

import (
	"context"
	"testing"
)

func TestIndexIDsPG(t *testing.T) {
	name := newNamesDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	for _, n := range []string{"SPX", "NDX"} {
		if _, err := owner.Exec(ctx, `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ('index', $1, 'USD', 'XNYS', 'XNYS', 'active', 0.01, 1, now())`, n); err != nil {
			t.Fatal(err)
		}
	}
	s, err := OpenIndexIDs(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// SPY is a stock, not an index: it is not returned
	got, err := s.IndexIDs(ctx, []string{"NDX", "SPX", "SPY", "RUT"})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].InstrumentID >= got[1].InstrumentID || got[0].Type != "index" || got[0].Name != "SPX" || got[1].Name != "NDX" {
		t.Fatalf("rows = %+v", got)
	}
}
