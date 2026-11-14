//go:build pg

package setup

import (
	"context"
	"io/fs"
	"os"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
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

// assertMig47 checks the three facts both tests share: segmentby, the policy and the goose row of version 47.
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

func TestMig47OverTwinPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, migrationsUpTo(t, 46), quietLog()); err != nil {
		t.Fatalf("apply 1 to 46: %v", err)
	}

	// Production before 0047: a compressed twin segmented by another column, holding one compressed chunk.
	c, err := pgx.Connect(ctx, adminDSN(t, s.db))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, q := range []string{
		`ALTER TABLE obs.option_open SET (timescaledb.compress, timescaledb.compress_segmentby = 'instrument_id')`,
		// The foreign keys to master.instrument are not the subject; skip them for this session.
		`SET session_replication_role = replica`,
		`INSERT INTO obs.option_open (session_day, capture_id, slot, instrument_id, underlying_id, plan_id, generation,
			market_data_type, requested_at, received_at, connection_generation, status, skew, outside_band, currency, underlying_ccy)
			VALUES ('2026-01-05', '00000000-0000-0000-0000-000000000001', '0930', 1, 2, '00000000-0000-0000-0000-000000000002', 1,
			1, now(), now(), 1, 1, false, false, 'USD', 'USD')`,
		`SELECT public.compress_chunk(c) FROM public.show_chunks('obs.option_open') c`,
	} {
		if _, err := c.Exec(ctx, q); err != nil {
			t.Fatalf("%.60s: %v", q, err)
		}
	}

	both := migrationsUpTo(t, 47)
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, both, quietLog()); err != nil {
		t.Fatalf("apply 47 over the twin: %v", err)
	}
	assertMig47(t, s)
}
