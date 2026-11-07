//go:build pg

package instrument

import (
	"context"
	"testing"
)

// newNamesDatabase is newDatabase (0001 to 0008) with 0009 and 0010 applied, and a stock and an option seeded by the owner.
func newNamesDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	for _, m := range []string{"0009_master_option_terms.sql", "0010_instrument_names_view.sql"} {
		applyMigration(t, connURL(t, name, ""), m)
	}
	owner := connectAs(t, name, "")
	for _, row := range [][2]string{{"stock", "SPY"}, {"option", "SPY C"}} {
		if _, err := owner.Exec(context.Background(), `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ($1, $2, 'USD', 'XNYS', 'XNYS', 'active', 0.01, 100, now())`, row[0], row[1]); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

func TestInstrumentNamesPG(t *testing.T) {
	name := newNamesDatabase(t)
	ctx := context.Background()
	s, err := OpenInstrumentNames(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	got, err := s.InstrumentNames(ctx, []int64{2, 1, 999999})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].InstrumentID != 1 || got[0].Type != "stock" || got[0].Name != "SPY" ||
		got[1].InstrumentID != 2 || got[1].Type != "option" || got[1].Name != "SPY C" {
		t.Fatalf("names = %+v", got)
	}
	pages := connectAs(t, name, "ot_ro_pages")
	mustFail(t, pages, `SELECT count(*) FROM master.instrument`)
}
