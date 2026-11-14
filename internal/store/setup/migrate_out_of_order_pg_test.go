//go:build pg

package setup

import (
	"context"
	"testing"
	"testing/fstest"
)

func fixtureMigration(table string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE meta." + table + " (id int);\n-- +goose Down\nDROP TABLE meta." + table + ";\n")}
}

func TestMigrateAppliesOutOfOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	open := s.openAs(t, testPassword)

	high := fstest.MapFS{"0200_high.sql": fixtureMigration("ooo_high")}
	if err := migrateLocked(ctx, t, open, high, quietLog()); err != nil {
		t.Fatalf("apply high: %v", err)
	}
	both := fstest.MapFS{"0200_high.sql": high["0200_high.sql"], "0100_low.sql": fixtureMigration("ooo_low")}
	if err := migrateLocked(ctx, t, open, both, quietLog()); err != nil {
		t.Fatalf("apply low after high: %v", err)
	}
	for _, v := range []int{100, 200} {
		if n := s.count(t, `SELECT count(*) FROM meta.goose_db_version WHERE version_id = $1 AND is_applied`, v); n != 1 {
			t.Fatalf("version %d recorded %d times, want 1", v, n)
		}
	}
	if n := s.count(t, `SELECT count(*) FROM pg_tables WHERE schemaname = 'meta' AND tablename IN ('ooo_low','ooo_high')`); n != 2 {
		t.Fatalf("%d fixture tables exist, want 2", n)
	}
}
