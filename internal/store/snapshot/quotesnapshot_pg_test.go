//go:build pg

package snapshot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// newQuoteDatabase creates a database with a random name, the Timescale extension and the roles, applies 0002, 0003
// and 0007 there, and drops the database in t.Cleanup. It returns its name. It shares connURL, applyMigration,
// connectAs and mustFail with optioneod_pg_test.go.
func newQuoteDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, pgEnv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	roles := []string{"ot_owner", "ot_runner", "ot_w_series_fred", "ot_ro_pages", "ot_w_quote_snapshot"}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range roles {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r).Scan(&have); err != nil {
			t.Fatal(err)
		}
		verb := "ALTER"
		if !have {
			verb = "CREATE"
		}
		if _, err := admin.Exec(ctx, verb+" ROLE "+r+" LOGIN PASSWORD '"+rolePassword+"'"); err != nil {
			t.Fatalf("%s role %s: %v", strings.ToLower(verb), r, err)
		}
	}
	// The lock stays held until the admin connection closes in t.Cleanup: TestSetupPG changes the passwords of
	// these roles under the same lock.
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := nonIdent.ReplaceAllString(strings.ToLower(t.Name()), "_")
	if len(name) > 40 {
		name = name[:40]
	}
	name += "_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		defer func() { _ = admin.Close(ctx) }()
		if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})
	ext, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatalf("connect new database: %v", err)
	}
	_, err = ext.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS timescaledb`)
	_ = ext.Close(ctx)
	if err != nil {
		t.Fatalf("create extension: %v", err)
	}
	for _, f := range []string{"0002_series.sql", "0003_ops_ledger.sql", "0007_quote_snapshot.sql"} {
		applyMigration(t, connURL(t, name, ""), f)
	}
	for _, r := range []string{"ot_runner", "ot_ro_pages", "ot_w_quote_snapshot"} {
		if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO `+r); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

func openQuoteAs(t *testing.T, name, user string) quoteStoreForTest {
	t.Helper()
	s, err := OpenQuoteSnapshot(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

type quoteStoreForTest interface {
	InsertQuoteSnapshots(context.Context, []contracts.QuoteSnapshot) error
	QuoteSnapshots(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.QuoteSnapshot, error)
}

func loadQuoteFixture(t *testing.T) []contracts.QuoteSnapshot {
	t.Helper()
	raw, err := os.ReadFile("../../../contracts/fixtures/obs.quote_snapshot.r1.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []contracts.QuoteSnapshot
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("empty fixture")
	}
	return rows
}

func quoteKey(r contracts.QuoteSnapshot) string {
	return fmt.Sprintf("%s|%s|%s|%d", r.SessionDay.Format("2006-01-02"), r.CaptureID, r.Slot, r.InstrumentID)
}

// readAllQuotes reads every instrument of rows through the reader and returns the JSON of each row by key.
func readAllQuotes(t *testing.T, r quoteStoreForTest, rows []contracts.QuoteSnapshot) map[string]string {
	t.Helper()
	seen := map[int]bool{}
	out := map[string]string{}
	for _, row := range rows {
		if seen[row.InstrumentID] {
			continue
		}
		seen[row.InstrumentID] = true
		got, err := r.QuoteSnapshots(context.Background(), int64(row.InstrumentID), "2000-01-01", "2100-01-01")
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			b, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			out[quoteKey(g)] = string(b)
		}
	}
	return out
}

func assertEqualQuotes(t *testing.T, want []contracts.QuoteSnapshot, got map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("read %d rows, want %d", len(got), len(want))
	}
	for _, w := range want {
		b, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		if got[quoteKey(w)] != string(b) {
			t.Fatalf("row %s\n got %s\nwant %s", quoteKey(w), got[quoteKey(w)], b)
		}
	}
}

func TestQuoteSnapshotRoundTripPG(t *testing.T) {
	t.Parallel()
	name := newQuoteDatabase(t)
	rows := loadQuoteFixture(t)
	if err := openQuoteAs(t, name, "ot_w_quote_snapshot").InsertQuoteSnapshots(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	assertEqualQuotes(t, rows, readAllQuotes(t, openQuoteAs(t, name, "ot_runner"), rows))
}

func TestQuoteSnapshotRetryPG(t *testing.T) {
	t.Parallel()
	name := newQuoteDatabase(t)
	w := openQuoteAs(t, name, "ot_w_quote_snapshot")
	r := openQuoteAs(t, name, "ot_runner")
	ctx := context.Background()
	first := loadQuoteFixture(t)[0]
	if err := w.InsertQuoteSnapshots(ctx, []contracts.QuoteSnapshot{first}); err != nil {
		t.Fatal(err)
	}
	other := first
	b, a := 9.5, 9.75
	other.Bid, other.Ask = &b, &a
	if err := w.InsertQuoteSnapshots(ctx, []contracts.QuoteSnapshot{other}); err != nil {
		t.Fatal(err)
	}
	assertEqualQuotes(t, []contracts.QuoteSnapshot{first}, readAllQuotes(t, r, []contracts.QuoteSnapshot{first}))

	// the same instrument and capture in another slot, and in another capture: both kept
	slot := other
	slot.Slot = "1500"
	capture := other
	capture.CaptureID = "00000000-0000-4000-8000-0000000000ff"
	if err := w.InsertQuoteSnapshots(ctx, []contracts.QuoteSnapshot{slot, capture}); err != nil {
		t.Fatal(err)
	}
	got := readAllQuotes(t, r, []contracts.QuoteSnapshot{first})
	if len(got) != 3 {
		t.Fatalf("%d rows, want 3", len(got))
	}
	assertEqualQuotes(t, []contracts.QuoteSnapshot{first, slot, capture}, got)
}

func TestQuoteSnapshotCompressPG(t *testing.T) {
	t.Parallel()
	name := newQuoteDatabase(t)
	rows := loadQuoteFixture(t)
	if err := openQuoteAs(t, name, "ot_w_quote_snapshot").InsertQuoteSnapshots(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := connectAs(t, name, "")
	var chunks []string
	rs, err := admin.Query(ctx, `SELECT c::text FROM show_chunks('obs.quote_snapshot') AS c`)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var c string
		if err := rs.Scan(&c); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, c)
	}
	rs.Close()
	if len(chunks) == 0 {
		t.Fatal("no chunk")
	}
	sort.Strings(chunks)
	for _, c := range chunks {
		if _, err := admin.Exec(ctx, `SELECT compress_chunk($1::regclass)`, c); err != nil {
			t.Fatalf("compress %s: %v", c, err)
		}
	}
	var compressed int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM timescaledb_information.chunks `+
		`WHERE hypertable_name = 'quote_snapshot' AND is_compressed`).Scan(&compressed); err != nil {
		t.Fatal(err)
	}
	if compressed == 0 {
		t.Fatal("no chunk was compressed")
	}
	assertEqualQuotes(t, rows, readAllQuotes(t, openQuoteAs(t, name, "ot_runner"), rows))
}

func TestQuoteSnapshotGrantsPG(t *testing.T) {
	t.Parallel()
	name := newQuoteDatabase(t)
	rows := loadQuoteFixture(t)
	if err := openQuoteAs(t, name, "ot_w_quote_snapshot").InsertQuoteSnapshots(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	w := connectAs(t, name, "ot_w_quote_snapshot")
	mustFail(t, w, `UPDATE obs.quote_snapshot SET bid = 1`)
	mustFail(t, w, `DELETE FROM obs.quote_snapshot`)
	mustFail(t, w, `SELECT count(*) FROM obs.quote_snapshot`)
	mustFail(t, w, `TRUNCATE obs.quote_snapshot`)

	p := connectAs(t, name, "ot_ro_pages")
	mustFail(t, p, `SELECT count(*) FROM obs.quote_snapshot`)
	var n int
	if err := p.QueryRow(context.Background(), `SELECT count(*) FROM rd.quote_snapshot_r1`).Scan(&n); err != nil {
		t.Fatalf("view not readable: %v", err)
	}
	if n != len(rows) {
		t.Fatalf("view has %d rows, want %d", n, len(rows))
	}
	rn := connectAs(t, name, "ot_runner")
	mustFail(t, rn, `UPDATE obs.quote_snapshot SET bid = 1`)
	mustFail(t, rn, `DELETE FROM obs.quote_snapshot`)
	mustFail(t, rn, `TRUNCATE obs.quote_snapshot`)
}
