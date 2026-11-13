//go:build pg

package barpg

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

func TestPriceBarUpsertAsWriterRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, admin := openWriter(ctx, t, name), pgxAs(t, name)
	up := func(basis, closeText, by string) bar.BarWriteCount {
		t.Helper()
		c, err := st.UpsertPriceBars(ctx, []contracts.PriceBar{pbar(7, basis, 1, "2026-11-05", closeText)}, by)
		if err != nil {
			t.Fatalf("upsert %s: %v", closeText, err)
		}
		return c
	}
	stored := func() (cl, by string, rec, upd time.Time) {
		t.Helper()
		err := admin.QueryRow(ctx, `SELECT close::text, written_by, received_at, updated_at FROM obs.price_bar
			WHERE basis = 'trades'`).Scan(&cl, &by, &rec, &upd)
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	if c := up("trades", "100.50", "run1"); c != (bar.BarWriteCount{Written: 1}) {
		t.Fatalf("insert = %+v", c)
	}
	_, _, rec0, upd0 := stored()
	if c := up("trades", "100.50", "run2"); c != (bar.BarWriteCount{Unchanged: 1}) {
		t.Errorf("same value = %+v", c)
	}
	if c := up("trades", "100.5", "run2"); c != (bar.BarWriteCount{Unchanged: 1}) {
		t.Errorf("equal numeric = %+v", c)
	}
	if cl, by, _, upd := stored(); cl != "100.50" || by != "run1" || !upd.Equal(upd0) {
		t.Errorf("unchanged row moved: %s %s %v", cl, by, upd)
	}
	later := pbar(7, "trades", 1, "2026-11-05", "101")
	later.ReceivedAt = later.ReceivedAt.Add(time.Hour)
	if c, err := st.UpsertPriceBars(ctx, []contracts.PriceBar{later}, "run3"); err != nil || c != (bar.BarWriteCount{Written: 1}) {
		t.Errorf("different close = %+v, %v", c, err)
	}
	if cl, by, rec, upd := stored(); cl != "101" || by != "run3" || !rec.Equal(rec0) || !upd.After(upd0) {
		t.Errorf("replaced row: %s %s %v %v", cl, by, rec, upd)
	}
	checkVoidAndBasis(ctx, t, admin, up)
}

func checkVoidAndBasis(ctx context.Context, t *testing.T, admin *pgx.Conn, up func(basis, closeText, by string) bar.BarWriteCount) {
	t.Helper()
	if _, err := admin.Exec(ctx, `UPDATE obs.price_bar SET status = 'void', open = NULL, high = NULL, low = NULL, close = NULL`); err != nil {
		t.Fatal(err)
	}
	if c := up("trades", "101", "run4"); c != (bar.BarWriteCount{Written: 1}) {
		t.Errorf("void row answered = %+v", c)
	}
	var status string
	if err := admin.QueryRow(ctx, `SELECT status FROM obs.price_bar`).Scan(&status); err != nil || status != "ok" {
		t.Errorf("status = %q, %v", status, err)
	}
	if c := up("adjusted_last", "90", "run5"); c != (bar.BarWriteCount{Written: 1}) {
		t.Errorf("second basis = %+v", c)
	}
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM obs.price_bar`).Scan(&n); err != nil || n != 2 {
		t.Errorf("rows = %d, want 2 (%v)", n, err)
	}
}

func TestPriceBarViewHasOneRowPerKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, admin := openWriter(ctx, t, name), pgxAs(t, name)
	rows := []contracts.PriceBar{pbar(7, "trades", 1, "2026-11-04", "10"), pbar(7, "trades", 1, "2026-11-05", "11")}
	if _, err := st.UpsertPriceBars(ctx, rows, "r"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO obs.price_bar (instrument_id, session_day, basis, open, high, low, close, currency,
		source, received_at, status) VALUES (7, '2026-11-06', 'trades', NULL, NULL, NULL, NULL, 'USD', 'ibkr', now(), 'void')`); err != nil {
		t.Fatal(err)
	}
	if n := count(t, admin, "rd.price_bar_r1"); n != 2 {
		t.Errorf("view rows = %d, want 2", n)
	}
	got, err := st.PriceBars(ctx, 7, "trades", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 2 || got[0].AdjustEpoch != 1 || got[1].AdjustEpoch != 1 || got[1].Close != "11" {
		t.Errorf("bars = %+v, %v", got, err)
	}
}

func TestPriceBarMigrationKeepsWinners(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newBaseDatabase(t)
	admin := pgxAs(t, name)
	if _, err := admin.Exec(ctx, `INSERT INTO obs.price_bar VALUES
		(7, '2026-11-02', 'trades', 1, '10.50', '10.50', '10.50', '10.50', '1200', 'USD', 'ibkr', '2026-11-06T21:00:00Z'),
		(7, '2026-11-03', 'trades', 1, '11', '11', '11', '11', NULL, 'USD', 'ibkr', '2026-11-06T21:00:00Z'),
		(7, '2026-11-03', 'trades', 2, '5.10', '5.10', '5.10', '5.10', NULL, 'USD', 'ibkr', '2026-11-07T21:00:00Z'),
		(7, '2026-11-04', 'trades', 2, '6', '6', '6', '6', NULL, 'USD', 'ibkr', '2026-11-07T21:00:00Z'),
		(7, '2026-11-02', 'adjusted_last', 1, '9', '9', '9', '9', NULL, 'USD', 'ibkr', '2026-11-06T21:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	applyUp(t, name, "0039_fx_rate_repair.sql")
	applyUp(t, name, "0041_price_bar_repair.sql")
	rows, err := admin.Query(ctx, `SELECT v.session_day::text, v.basis, v.close, v.adjust_epoch, t.status, t.received_at = t.updated_at
		FROM rd.price_bar_r1 v JOIN obs.price_bar t USING (instrument_id, basis, session_day) ORDER BY 2, 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []struct{ day, basis, cl string }{
		{"2026-11-03", "trades", "5.10"}, {"2026-11-04", "trades", "6"}, {"2026-11-02", "adjusted_last", "9"},
	}
	var got []struct{ day, basis, cl string }
	for rows.Next() {
		var d, b, c, st string
		var ep int
		var same bool
		if err := rows.Scan(&d, &b, &c, &ep, &st, &same); err != nil {
			t.Fatal(err)
		}
		if ep != 1 || st != "ok" || !same {
			t.Errorf("row %s %s: epoch %d status %s updated=received %v", d, b, ep, st, same)
		}
		got = append(got, struct{ day, basis, cl string }{d, b, c})
	}
	if len(got) != 3 {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			found = found || g == w
		}
		if !found {
			t.Errorf("missing %v in %v", w, got)
		}
	}
}

func TestOldPriceBarInsertSQLWorksOnMigratedTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w := pgxAsUser(t, name, "ot_w_bar")
	at := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	args := func(day string, epoch int32) []any {
		return []any{int64(7), day, "trades", epoch, "1", "2", "1", "1.5", nil, "USD", "ibkr", at}
	}
	for i, tc := range []struct {
		day  string
		want int64
	}{{"2026-11-05", 1}, {"2026-11-05", 0}, {"2026-11-06", 1}} {
		tag, err := w.Exec(ctx, insertPriceBarSQL, args(tc.day, 2)...)
		if err != nil || tag.RowsAffected() != tc.want {
			t.Errorf("insert %d: rows %d, %v; want %d", i, tag.RowsAffected(), err, tc.want)
		}
	}
}
