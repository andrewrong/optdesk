//go:build pg

package snapshot

import (
	"context"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// seedCatalog inserts one capture per (session_day, instrument): underlying 21 on three days, underlying 7 on two.
func seedCatalog(t *testing.T, name string) {
	t.Helper()
	admin := connectAs(t, name, "")
	const ins = `INSERT INTO obs.option_eod (session_day, capture_id, instrument_id, underlying_id, generation, ` +
		`market_data_type, received_at, status, currency, snapshot_kind, underlying_ccy) ` +
		`VALUES ($1::date, gen_random_uuid(), $2, $3, 1, 1, now(), 1, 'USD', 'eod', 'USD')`
	rows := []struct {
		day        string
		instrument int
		underlying int
	}{
		{"2026-11-02", 101, 21}, {"2026-11-03", 101, 21}, {"2026-11-04", 101, 21},
		{"2026-11-02", 201, 7}, {"2026-11-03", 201, 7},
	}
	for _, r := range rows {
		if _, err := admin.Exec(context.Background(), ins, r.day, r.instrument, r.underlying); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatalogPG(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	seedCatalog(t, name)
	c, err := OpenOptionEodCatalog(context.Background(), connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatalf("open as ot_ro_pages: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	cases := []struct {
		name  string
		since string
		want  []store.OptionEodLatest
	}{
		{"no bound", "", []store.OptionEodLatest{{UnderlyingID: 7, SessionDay: "2026-11-03"}, {UnderlyingID: 21, SessionDay: "2026-11-04"}}},
		{"since inside", "2026-11-03", []store.OptionEodLatest{{UnderlyingID: 7, SessionDay: "2026-11-03"}, {UnderlyingID: 21, SessionDay: "2026-11-04"}}},
		{"since after one last day", "2026-11-04", []store.OptionEodLatest{{UnderlyingID: 21, SessionDay: "2026-11-04"}}},
		{"since after all", "2026-11-05", []store.OptionEodLatest{}},
	}
	for _, tc := range cases {
		got, err := c.OptionEodLatest(context.Background(), contracts.Day(tc.since))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
