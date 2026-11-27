//go:build pg

package findingpg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/store/finding"
)

// inputTables stand in, in the temporary database, for the two input views the migration 0075 grants.
var inputTables = map[string]string{
	"earnings_date_r1": `market text, observed_on date, status text`,
	"price_bar_r1":     `session_day date, basis text, open text, high text, low text, close text`,
}

func TestEarningsChecksCountsStatuses(t *testing.T) {
	t.Parallel()
	p, c := openProbe(t)
	addInputTables(t, c)
	for _, s := range []string{"listed", "absent", "error"} {
		exec(t, c, `INSERT INTO rd.earnings_date_r1 VALUES ('US', '2026-11-24', $1)`, s)
	}
	got, err := p.EarningsChecks(context.Background(), "2026-11-24", "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	want := finding.EarningsCheck{Market: "US", Day: "2026-11-24", Names: 3, Listed: 1, Errors: 1}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestBarShapesCountsBadTradesBars(t *testing.T) {
	t.Parallel()
	p, c := openProbe(t)
	addInputTables(t, c)
	ins := `INSERT INTO rd.price_bar_r1 VALUES ('2026-11-24', $1, $2, $3, $4, $5)`
	exec(t, c, ins, "trades", "10", "12", "9", "11")
	exec(t, c, ins, "trades", "13", "12", "9", "11")
	exec(t, c, ins, "trades", "10", "12", "9", "8")
	exec(t, c, ins, "adjusted_last", "13", "12", "9", "11")
	got, err := p.BarShapes(context.Background(), "2026-11-24", "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	want := finding.BarShape{Day: "2026-11-24", Rows: 3, Bad: 2}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// addInputTables creates the two tables and grants SELECT as migration 0075 does.
func addInputTables(t *testing.T, c *pgx.Conn) {
	t.Helper()
	for v, cols := range inputTables {
		exec(t, c, `CREATE TABLE rd.`+v+` (`+cols+`)`)
		exec(t, c, `GRANT SELECT ON rd.`+v+` TO ot_w_datacheck`)
	}
}
