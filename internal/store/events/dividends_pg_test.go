//go:build pg

package events

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

func TestInsertDividendUpsertsOneRowPerDayPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	seedInstrument(t, name, divInstrument)
	w, err := OpenDividend(ctx, connURL(t, name, "ot_w_events"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.InsertDividend(ctx, divRow("2026-11-09", 1, "error")); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertDividend(ctx, divRow("2026-11-09", 1, "present")); err != nil {
		t.Fatalf("second answer of one day: %v", err)
	}
	var n int
	if err := pgxAs(t, name).QueryRow(ctx, `SELECT count(*) FROM obs.dividend_check`).Scan(&n); err != nil || n != 1 {
		t.Errorf("%d answers, %v; want 1", n, err)
	}
	got, err := w.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].Status != "present" || got[0].Attempt != 1 {
		t.Errorf("rows = %+v, %v; want the present answer", got, err)
	}
}

func TestDividendReaderReadsTheViewOnBothRolesPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	seedInstrument(t, name, divInstrument)
	w, err := OpenDividend(ctx, connURL(t, name, "ot_w_events"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	for _, r := range []contracts.DividendExpectation{
		divRow("2026-11-09", 1, "error"), divRow("2026-11-09", 2, "present"), divRow("2026-11-10", 1, "no_value"),
	} {
		if err := w.InsertDividend(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	p, err := OpenDividend(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	for role, r := range map[string]dividend.Reader{"ot_w_events": w, "ot_ro_pages": p} {
		got, err := r.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-10")
		if err != nil || len(got) != 2 || got[0].Attempt != 1 || got[0].NextExDate == nil || got[0].NextAmount == nil {
			t.Fatalf("%s: %+v, %v; want day 9 present (attempt 1), then day 10", role, got, err)
		}
	}
}

func TestDividendRolesCannotReadTableOrPageWriteOrChangeRowsPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	for _, role := range []string{"ot_w_events", "ot_ro_pages"} {
		c, err := pgx.Connect(ctx, connURL(t, name, role))
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			`SELECT received_at FROM obs.dividend_expectation`, `UPDATE obs.dividend_expectation SET ex_date = DATE '2026-01-01'`,
			`DELETE FROM obs.dividend_expectation`, `TRUNCATE obs.dividend_expectation`,
		} {
			if _, err := c.Exec(ctx, q); err == nil {
				t.Errorf("%s ran %q", role, q)
			}
		}
		_ = c.Close(ctx)
	}
	p, err := OpenDividend(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if _, err := connectAs(t, name, "ot_ro_pages").Exec(ctx, `UPDATE obs.dividend_expectation SET issuer = 'X'`); err == nil {
		t.Error("the page role updated an issuer")
	}
	if err := p.InsertDividend(ctx, divRow("2026-11-09", 1, "no_value")); err == nil {
		t.Error("the page role wrote a row")
	}
}
