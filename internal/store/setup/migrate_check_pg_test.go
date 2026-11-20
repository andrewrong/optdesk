//go:build pg

package setup

import (
	"context"
	"net/url"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestMigrateMissingAsRunnerPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateLocked(ctx, t, s.openAs(t, testPassword), os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	owner := connectAs(ctx, t, adminDSN(t, s.db), os.Getenv("PGUSER"), os.Getenv("PGPASSWORD"))
	rows, err := owner.Query(ctx, `SELECT version_id FROM meta.goose_db_version WHERE is_applied AND version_id > 0 ORDER BY version_id`)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil || len(applied) == 0 {
		t.Fatalf("applied versions %v, err %v", applied, err)
	}
	last := slices.Max(applied)
	if _, err := owner.Exec(ctx, `CREATE TABLE meta.other_probe (a int)`); err != nil {
		t.Fatal(err)
	}

	// ot_runner's password is changed by TestSetupPG; lock 54 orders the two.
	if _, err := s.admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	_, err = s.admin.Exec(ctx, "ALTER ROLE ot_runner PASSWORD "+literal(testPassword))
	var runner *pgx.Conn
	if err == nil {
		runner = connectAs(ctx, t, adminDSN(t, s.db), roleRunner, testPassword)
	}
	_, _ = s.admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
	if err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(adminDSN(t, s.db))
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(roleRunner, testPassword)
	dsn := u.String()

	missing, _, err := MigrateMissing(ctx, dsn, applied)
	if err != nil || len(missing) != 0 {
		t.Fatalf("applied list: missing %v, err %v", missing, err)
	}
	missing, _, err = MigrateMissing(ctx, dsn, append(slices.Clone(applied), last+1))
	if err != nil || !slices.Equal(missing, []int64{last + 1}) {
		t.Fatalf("list with one more: missing %v, err %v", missing, err)
	}

	if _, err := runner.Exec(ctx, `SELECT 1 FROM meta.other_probe`); !isPermissionError(err) {
		t.Errorf("meta.other_probe: want a permission error, got %v", err)
	}
	if _, err := runner.Exec(ctx, `INSERT INTO meta.goose_db_version (version_id, is_applied) VALUES (1, true)`); !isPermissionError(err) {
		t.Errorf("insert: want a permission error, got %v", err)
	}
}
