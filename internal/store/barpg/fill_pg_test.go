//go:build pg

package barpg

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// fredRow is fxRow with the fill source of docs/design/source-rank.md.
func fredRow(d, rate string) contracts.FxRate {
	r := fxRow(d, rate)
	setText(&r.Source, "fred_h10")
	return r
}

func storedSource(ctx context.Context, t *testing.T, name string) (source, rate string) {
	t.Helper()
	err := pgxAs(t, name).QueryRow(ctx, `SELECT source, rate::text FROM obs.fx_rate`).Scan(&source, &rate)
	if err != nil {
		t.Fatal(err)
	}
	return source, rate
}

func TestInsertFredRateForMissingKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st := openWriter(ctx, t, name)
	n, err := st.InsertFxRates(ctx, []contracts.FxRate{fredRow("2026-11-05", "153.42")})
	if err != nil || n != 1 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	if src, rate := storedSource(ctx, t, name); src != "fred_h10" || rate != "153.42" {
		t.Errorf("stored %s %s", src, rate)
	}
}

func TestInsertNeverReplacesIbkrRate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st := openWriter(ctx, t, name)
	if _, err := st.InsertFxRates(ctx, []contracts.FxRate{fxRow("2026-11-05", "153.425")}); err != nil {
		t.Fatal(err)
	}
	n, err := st.InsertFxRates(ctx, []contracts.FxRate{fredRow("2026-11-05", "150")})
	if err != nil || n != 0 {
		t.Fatalf("fill insert = %d, %v", n, err)
	}
	if src, rate := storedSource(ctx, t, name); src != "ibkr_midpoint" || rate != "153.425" {
		t.Errorf("stored %s %s", src, rate)
	}
}

func TestSourceOutsideListRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st := openWriter(ctx, t, name)
	r := fxRow("2026-11-05", "153.4")
	setText(&r.Source, "other_source")
	if _, err := st.InsertFxRates(ctx, []contracts.FxRate{r}); err == nil {
		t.Fatal("source outside the list accepted")
	}
	if c := count(t, pgxAs(t, name), "obs.fx_rate"); c != 0 {
		t.Errorf("rows = %d", c)
	}
}

func TestUpsertReplacesFredRate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st := openWriter(ctx, t, name)
	if _, err := st.InsertFxRates(ctx, []contracts.FxRate{fredRow("2026-11-05", "153.425")}); err != nil {
		t.Fatal(err)
	}
	c, err := st.UpsertFxRates(ctx, []contracts.FxRate{fxRow("2026-11-05", "153.4250")}, "run1")
	if err != nil || c != (bar.FxWriteCount{Written: 1}) {
		t.Fatalf("upsert over fred = %+v, %v", c, err)
	}
	if src, _ := storedSource(ctx, t, name); src != "ibkr_midpoint" {
		t.Errorf("source = %s", src)
	}
	c, err = st.UpsertFxRates(ctx, []contracts.FxRate{fxRow("2026-11-05", "153.425")}, "run2")
	if err != nil || c != (bar.FxWriteCount{Unchanged: 1}) {
		t.Errorf("equal ibkr row = %+v, %v", c, err)
	}
}

// downPart returns the Down part of 0050_fx_rate_source.sql.
func downPart(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/0050_fx_rate_source.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, down, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("no Down part")
	}
	return strings.NewReplacer("-- +goose StatementBegin", "", "-- +goose StatementEnd", "").Replace(down)
}

func TestMigrationDownRefusesFredRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	admin := pgxAs(t, name)
	if _, err := admin.Exec(ctx, `INSERT INTO obs.fx_rate (pair, rate_day, rate, source, received_at)
		VALUES ('USDJPY', '2026-11-05', 153.4, 'fred_h10', '2026-11-06T21:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, downPart(t)); err == nil {
		t.Fatal("Down ran over a fred_h10 row")
	}
	if src, _ := storedSource(ctx, t, name); src != "fred_h10" {
		t.Errorf("source = %s", src)
	}
	if _, err := admin.Exec(ctx, `DELETE FROM obs.fx_rate`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, downPart(t)); err != nil {
		t.Fatalf("Down without fred rows: %v", err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO obs.fx_rate (pair, rate_day, rate, source, received_at)
		VALUES ('USDJPY', '2026-11-05', 153.4, 'fred_h10', '2026-11-06T21:00:00Z')`); err == nil {
		t.Error("old check not restored")
	}
}
