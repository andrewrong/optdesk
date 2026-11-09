//go:build pg

package events

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

// newDividendDatabase makes the throwaway database of newDatabase (roles, schemas, migration 0020) and applies
// migration 0023 on top.
func newDividendDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	raw, err := os.ReadFile("../../../migrations/0023_dividends.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(context.Background(), connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()
	if _, err := c.Exec(context.Background(), strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0023: %v", err)
	}
	return name
}

func TestDividendWriterKeepsExistingKeyPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDividendDatabase(t)
	w, err := OpenDividend(ctx, connURL(t, name, "ot_w_events"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.InsertDividend(ctx, divRow("2026-11-09", 1, "error")); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertDividend(ctx, divRow("2026-11-09", 1, "no_value")); err != nil {
		t.Fatalf("second insert of one key: %v", err)
	}
	var n int
	if err := pgxAs(t, name).QueryRow(ctx, `SELECT count(*) FROM obs.dividend_expectation`).Scan(&n); err != nil || n != 1 {
		t.Errorf("%d rows, %v; want 1", n, err)
	}
	got, err := w.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].Status != "error" {
		t.Errorf("rows = %+v, %v; want the first row", got, err)
	}
}

func TestDividendReaderTakesHighestAttemptOnBothRolesPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDividendDatabase(t)
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
		if err != nil || len(got) != 2 || got[0].Attempt != 2 || got[0].NextExDate == nil || got[0].NextAmount == nil {
			t.Fatalf("%s: %+v, %v; want day 9 attempt 2 present, then day 10", role, got, err)
		}
	}
}

func TestDividendRolesCannotReadTableOrPageWriteOrChangeRowsPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDividendDatabase(t)
	for _, role := range []string{"ot_w_events", "ot_ro_pages"} {
		c, err := pgx.Connect(ctx, connURL(t, name, role))
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			`SELECT 1 FROM obs.dividend_expectation`, `UPDATE obs.dividend_expectation SET issuer = 'X'`,
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
	if err := p.InsertDividend(ctx, divRow("2026-11-09", 1, "no_value")); err == nil {
		t.Error("the page role wrote a row")
	}
}
