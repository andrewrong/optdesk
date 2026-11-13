//go:build pg

package ops

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// aliasTables are the nine tables the rename touches, each with the SQL that inserts one row under alias $1.
var aliasTables = []struct{ table, insert string }{
	{"account.capture", `INSERT INTO account.capture VALUES ('c-' || $1, $1, now(), '2026-11-05', 'ok')`},
	{"account.position", `INSERT INTO account.position VALUES ('c-' || $1, $1, now(), '2026-11-05', 'AAPL', 'stock', '', 'USD', '', '', '', '', 1, 1, 1)`},
	{"account.value", `INSERT INTO account.value VALUES ('c-' || $1, $1, now(), '2026-11-05', 'NetLiquidation', 1, 'USD')`},
	{"account.execution", `INSERT INTO account.execution VALUES ($1, 'e1', 1, now(), '2026-11-05', 'AAPL', 'stock', '', '', '', '', 'USD', 'BOT', 1, 1, 0, 'USD')`},
	{"account.position_mark", `INSERT INTO account.position_mark (capture_id, alias, marked_at, session_day, con_id, attempt, currency, basis, status) VALUES ('c-' || $1, $1, now(), '2026-11-05', 1, 1, 'USD', 'none', 'ok')`},
	{"candidates.option_candidate", `INSERT INTO candidates.option_candidate (session_day, run_id, kind, instrument_id, alias, rule_version, issuer, underlying_id, expiry, dte, option_right, strike, multiplier, bid_eod, break_even, status, gates, marks, fits, earnings_state, computed_at) VALUES ('2026-11-05', 'r', 'short_put', 1, $1, 'v', 'i', 1, '2026-12-18', 40, 'put', '100', 100, '1', '99', 'ok', '', '', '', '', now())`},
	{"optionledger.flex_trade_row", `INSERT INTO optionledger.flex_trade_row VALUES ($1, 'EXECUTION', 't1', 1, 'a', 'h', 'flex', now(), 'f', 0)`},
	{"optionledger.flex_section_row", `INSERT INTO optionledger.flex_section_row VALUES ($1, 's', 'k', 1, 'f', 0, 'a', 'h', now())`},
	{"optionledger.flex_section_status", `INSERT INTO optionledger.flex_section_status (fetch_id, alias, section, state, row_count, rows_refused, recorded_at) VALUES ('f', $1, 's', 'present', 0, 0, now())`},
}

// renameDatabase makes a database with the nine tables and the rename function, applied as the owner would.
func renameDatabase(t *testing.T) *pgx.Conn {
	t.Helper()
	name := newDatabase(t)
	for _, f := range []string{"0018_candidates.sql", "0019_account.sql", "0022_optionledger.sql", "0024_flex_section.sql", "0026_position_mark.sql", "0027_candidate_settings_version.sql"} {
		applyMigration(t, connURL(t, name, ""), f)
	}
	applyMigration(t, connURL(t, name, ""), "0037_account_alias_rename.sql")
	return connectAs(t, name, "")
}

func countAlias(t *testing.T, c *pgx.Conn, table, alias string) int {
	t.Helper()
	var n int
	if err := c.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE alias = $1", alias).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRenameAccountAliasRenamesNineTablesPG(t *testing.T) {
	t.Parallel()
	c := renameDatabase(t)
	ctx := context.Background()
	for _, tb := range aliasTables {
		for _, a := range []string{"a", "b"} {
			if _, err := c.Exec(ctx, tb.insert, a); err != nil {
				t.Fatalf("insert %s %s: %v", tb.table, a, err)
			}
		}
	}
	if _, err := c.Exec(ctx, `SELECT ops.rename_account_alias('a', 'main')`); err != nil {
		t.Fatal(err)
	}
	for _, tb := range aliasTables {
		if n := countAlias(t, c, tb.table, "a"); n != 0 {
			t.Errorf("%s: %d rows left under the old alias", tb.table, n)
		}
		if n := countAlias(t, c, tb.table, "main"); n != 1 {
			t.Errorf("%s: %d rows under the new alias, want 1", tb.table, n)
		}
		if n := countAlias(t, c, tb.table, "b"); n != 1 {
			t.Errorf("%s: other alias has %d rows, want 1", tb.table, n)
		}
	}
}

func TestRenameAccountAliasCollisionRefusedPG(t *testing.T) {
	t.Parallel()
	// The keys of the tables whose primary key holds alias collide when both aliases hold the same key.
	for _, tb := range aliasTables {
		switch tb.table {
		case "account.execution", "candidates.option_candidate", "optionledger.flex_trade_row", "optionledger.flex_section_row", "optionledger.flex_section_status":
		default:
			continue
		}
		t.Run(tb.table, func(t *testing.T) {
			t.Parallel()
			c := renameDatabase(t)
			ctx := context.Background()
			for _, a := range []string{"a", "main"} {
				if _, err := c.Exec(ctx, tb.insert, a); err != nil {
					t.Fatal(err)
				}
			}
			// A row of another table under the old alias must stay untouched when the call is refused.
			if _, err := c.Exec(ctx, aliasTables[0].insert, "a"); err != nil {
				t.Fatal(err)
			}
			_, err := c.Exec(ctx, `SELECT ops.rename_account_alias('a', 'main')`)
			if err == nil || !strings.Contains(err.Error(), tb.table) {
				t.Fatalf("err = %v, want a refusal naming %s", err, tb.table)
			}
			if n := countAlias(t, c, tb.table, "a"); n != 1 {
				t.Errorf("%s: %d rows under the old alias after the refusal, want 1", tb.table, n)
			}
			if n := countAlias(t, c, "account.capture", "a"); n != 1 {
				t.Errorf("account.capture: %d rows under the old alias after the refusal, want 1", n)
			}
		})
	}
}
