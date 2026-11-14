//go:build pg

package setup

import (
	"context"
	"io/fs"
	"os"
	"strconv"
	"testing"
	"testing/fstest"
)

// migrationsUpTo is the migrations of the repository with a version of at most n.
func migrationsUpTo(t *testing.T, n int) fs.FS {
	t.Helper()
	out := fstest.MapFS{}
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) < 5 || name[len(name)-4:] != ".sql" {
			continue
		}
		v, err := strconv.Atoi(name[:4])
		if err != nil || v > n {
			continue
		}
		b, err := os.ReadFile(migrationsDir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = &fstest.MapFile{Data: b}
	}
	return out
}

// assertMig47 checks the three facts both migration tests share: segmentby, the policy and the goose row of version 47.
func assertMig47(t *testing.T, s scratch) {
	t.Helper()
	if n := s.count(t, `SELECT count(*) FROM timescaledb_information.compression_settings
		WHERE hypertable_schema = 'obs' AND hypertable_name = 'option_open' AND attname = 'underlying_id' AND segmentby_column_index IS NOT NULL`); n != 1 {
		t.Fatalf("underlying_id is segmentby %d times, want 1", n)
	}
	if n := s.count(t, `SELECT count(*) FROM timescaledb_information.compression_settings
		WHERE hypertable_schema = 'obs' AND hypertable_name = 'option_open' AND segmentby_column_index IS NOT NULL`); n != 1 {
		t.Fatalf("%d segmentby columns, want only underlying_id", n)
	}
	if n := s.count(t, `SELECT count(*) FROM timescaledb_information.jobs
		WHERE proc_name = 'policy_compression' AND hypertable_schema = 'obs' AND hypertable_name = 'option_open'`); n != 1 {
		t.Fatalf("%d compression policies, want 1", n)
	}
	if n := s.count(t, `SELECT count(*) FROM meta.goose_db_version WHERE version_id = 47 AND is_applied`); n != 1 {
		t.Fatalf("version 47 recorded %d times, want 1", n)
	}
}

func TestMig47FreshPG(t *testing.T) {
	t.Parallel()
	s := newScratch(t, true)
	if err := migrateWith(context.Background(), s.openAs(t, testPassword), gooseUp, migrationsUpTo(t, 47), quietLog()); err != nil {
		t.Fatalf("apply 1 to 47: %v", err)
	}
	assertMig47(t, s)
}

// TestMig47GrantsPG: the two capture writers hold table-level SELECT, UPDATE and INSERT and nothing that removes rows.
func TestMig47GrantsPG(t *testing.T) {
	t.Parallel()
	s := newScratch(t, true)
	if err := migrateWith(context.Background(), s.openAs(t, testPassword), gooseUp, migrationsUpTo(t, 47), quietLog()); err != nil {
		t.Fatalf("apply 1 to 47: %v", err)
	}
	for _, c := range []struct{ role, table string }{
		{"ot_w_option_eod", "obs.option_eod"},
		{"ot_w_option_open", "obs.option_open"},
	} {
		for priv, want := range map[string]bool{"SELECT": true, "UPDATE": true, "INSERT": true, "DELETE": false, "TRUNCATE": false} {
			n := s.count(t, `SELECT count(*) FROM (SELECT 1) x WHERE has_table_privilege('`+c.role+`', '`+c.table+`', '`+priv+`')`)
			if got := n == 1; got != want {
				t.Errorf("%s %s on %s = %v, want %v", c.role, priv, c.table, got, want)
			}
		}
	}
}
