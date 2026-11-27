//go:build pg

package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGatewayStateGrants: ot_runner inserts and updates the table and cannot delete or truncate; ot_ro_pages reads the
// view and not the table.
func TestGatewayStateGrants(t *testing.T) {
	t.Parallel()
	ctx, c := sessionDayDB(t)
	if err := asRole(ctx, t, c, "ot_runner", `INSERT INTO ops.gateway_state (alias, down, since, observed_at)
		VALUES ('main', true, now(), now())`); err != nil {
		t.Errorf("insert as ot_runner: %v", err)
	}
	if err := asRole(ctx, t, c, "ot_runner", `UPDATE ops.gateway_state SET down = false, since = now(), observed_at = now()
		WHERE alias = 'main'`); err != nil {
		t.Errorf("update as ot_runner: %v", err)
	}
	for name, q := range map[string]string{
		"delete":   `DELETE FROM ops.gateway_state`,
		"truncate": `TRUNCATE ops.gateway_state`,
	} {
		if err := asRole(ctx, t, c, "ot_runner", q); err == nil {
			t.Errorf("%s as ot_runner succeeded", name)
		}
	}
	if err := asRole(ctx, t, c, "ot_runner", `INSERT INTO ops.gateway_state (alias, down, since, observed_at)
		VALUES ('Bad-Alias', true, now(), now())`); err == nil {
		t.Errorf("insert of an alias outside ^[a-z0-9]+$ succeeded")
	}
	if err := asRole(ctx, t, c, roleRoPages, `SELECT alias, down, since, observed_at FROM rd.gateway_state_r1`); err != nil {
		t.Errorf("ot_ro_pages select view: %v", err)
	}
	if err := asRole(ctx, t, c, roleRoPages, `SELECT * FROM ops.gateway_state`); err == nil {
		t.Errorf("ot_ro_pages read the table")
	}
}

// TestMigrate0074Down: after the Down section the table and the view are gone.
func TestMigrate0074Down(t *testing.T) {
	t.Parallel()
	ctx, c := sessionDayDB(t)
	raw, err := os.ReadFile(filepath.Join(migrationsDir, "0074_gateway_state.sql"))
	if err != nil {
		t.Fatal(err)
	}
	_, down, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("no Down section")
	}
	if _, err := c.Exec(ctx, down); err != nil {
		t.Fatalf("down: %v", err)
	}
	var n int
	if err := c.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace s ON s.oid = c.relnamespace
		WHERE (s.nspname = 'ops' AND c.relname = 'gateway_state') OR (s.nspname = 'rd' AND c.relname = 'gateway_state_r1')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d objects remain", n)
	}
}
