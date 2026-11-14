//go:build pg

package snapshot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// libpqVars are the connection variables the pg tests read; none has a default.
var libpqVars = []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"}

// rolePassword is the password of the roles in the throwaway database of the tests.
const rolePassword = "pg-test-only"

// connURL builds the connection URL of database dbname for user from the libpq environment; user "" is the admin.
func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	var missing []string
	for _, v := range libpqVars {
		if pgEnv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(pgEnv("PGUSER"), pgEnv("PGPASSWORD")),
		Host:   pgEnv("PGHOST") + ":" + pgEnv("PGPORT"),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	if m := pgEnv("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// ensureRoles creates the roles of 0002, 0003 and 0006 when missing and puts rolePassword on them. It takes advisory
// lock 54 and keeps it until the admin connection closes (t.Cleanup of newDatabase): the roles are shared by every
// database of the server, and TestSetupPG changes their passwords under the same lock, so a test that connects as a
// role must hold the lock for its whole run.
func ensureRoles(ctx context.Context, t *testing.T, admin *pgx.Conn) {
	t.Helper()
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_runner", "ot_w_series_fred", "ot_ro_pages", "ot_w_option_eod"} {
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
}

// newDatabase creates a database with a random name, the Timescale extension and the roles, applies 0002, 0003
// and 0006 there, and drops the database in t.Cleanup. It returns its name.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, pgEnv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	ensureRoles(ctx, t, admin)
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
	for _, f := range []string{"0002_series.sql", "0003_ops_ledger.sql", "0006_option_eod.sql"} {
		applyMigration(t, connURL(t, name, ""), f)
	}
	for _, r := range []string{"ot_runner", "ot_ro_pages", "ot_w_option_eod"} {
		if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO `+r); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

// applyMigration runs the Up section of migrations/<file> on the database at dsn.
func applyMigration(t *testing.T, dsn, file string) {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/" + file)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	up = strings.Replace(up, "-- +goose Up", "", 1)
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, up); err != nil {
		t.Fatalf("apply %s: %v", file, err)
	}
}

func connectAs(t *testing.T, name, user string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatalf("connect as %s: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func mustFail(t *testing.T, c *pgx.Conn, sql string) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql); err == nil {
		t.Errorf("statement was allowed: %s", sql)
	}
}

func openAs(t *testing.T, name, user string) interface {
	InsertOptionEod(context.Context, []contracts.OptionEod) error
	OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error)
} {
	t.Helper()
	s, err := OpenOptionEod(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func loadFixture(t *testing.T) []contracts.OptionEod {
	t.Helper()
	raw, err := os.ReadFile("../../../contracts/fixtures/obs.option_eod.r1.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []contracts.OptionEod
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("empty fixture")
	}
	return rows
}

func rowKey(r contracts.OptionEod) string {
	return fmt.Sprintf("%s|%s|%d", r.SessionDay.Format("2006-01-02"), r.CaptureID, r.InstrumentID)
}

// readAll reads every underlying of rows through the reader and returns the JSON of each row by key.
func readAll(t *testing.T, r interface {
	OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error)
}, rows []contracts.OptionEod) map[string]string {
	t.Helper()
	seen := map[int]bool{}
	out := map[string]string{}
	for _, row := range rows {
		if seen[row.UnderlyingID] {
			continue
		}
		seen[row.UnderlyingID] = true
		got, err := r.OptionEod(context.Background(), int64(row.UnderlyingID), "2000-01-01", "2100-01-01")
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			b, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			out[rowKey(g)] = string(b)
		}
	}
	return out
}

func assertEqualRows(t *testing.T, want []contracts.OptionEod, got map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("read %d rows, want %d", len(got), len(want))
	}
	for _, w := range want {
		b, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		if got[rowKey(w)] != string(b) {
			t.Fatalf("row %s\n got %s\nwant %s", rowKey(w), got[rowKey(w)], b)
		}
	}
}

// pickedFixture drops the fixture row that the read hides: the first row shares its key and received_at with the
// fourth, which has the greater capture_id.
func pickedFixture(rows []contracts.OptionEod) []contracts.OptionEod {
	return append(append([]contracts.OptionEod{}, rows[1:3]...), rows[3:]...)
}

func TestOptionEodPickPG(t *testing.T) {
	t.Parallel()
	for _, c := range pickCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			name := newDatabase(t)
			var rows []contracts.OptionEod
			capture := map[string]string{}
			for _, p := range c.in {
				id := fmt.Sprintf("00000000-0000-4000-8000-%012x", p.capture[0])
				capture[id] = p.capture
				r := baseRow(t)
				r.SessionDay = day(t, p.day)
				r.CaptureID, r.InstrumentID, r.Status, r.SnapshotKind = id, p.inst, p.status, p.kind
				r.ReceivedAt = r.ReceivedAt.Add(time.Duration(p.minute) * time.Minute)
				r.ErrorCode, r.TermsVersionID = nil, nil
				rows = append(rows, r)
			}
			if err := openAs(t, name, "ot_w_option_eod").InsertOptionEod(context.Background(), rows); err != nil {
				t.Fatal(err)
			}
			got, err := openAs(t, name, "ot_runner").OptionEod(context.Background(), 8, "2000-01-01", "2100-01-01")
			if err != nil {
				t.Fatal(err)
			}
			var have []string
			for _, r := range got {
				have = append(have, fmt.Sprintf("%s %d %s", r.SessionDay.Format(time.DateOnly), r.InstrumentID, capture[r.CaptureID]))
			}
			if !reflect.DeepEqual(have, c.want) {
				t.Fatalf("got %v, want %v", have, c.want)
			}
		})
	}
}

func TestOptionEodRoundTripPG(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	rows := loadFixture(t)
	if err := openAs(t, name, "ot_w_option_eod").InsertOptionEod(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	assertEqualRows(t, pickedFixture(rows), readAll(t, openAs(t, name, "ot_runner"), rows))
}

func TestOptionEodRetryPG(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	w := openAs(t, name, "ot_w_option_eod")
	r := openAs(t, name, "ot_runner")
	ctx := context.Background()
	first := loadFixture(t)[0]
	if err := w.InsertOptionEod(ctx, []contracts.OptionEod{first}); err != nil {
		t.Fatal(err)
	}
	other := first
	b, a := 9.5, 9.75
	other.Bid, other.Ask = &b, &a
	if err := w.InsertOptionEod(ctx, []contracts.OptionEod{other}); err != nil {
		t.Fatal(err)
	}
	got := readAll(t, r, []contracts.OptionEod{first})
	assertEqualRows(t, []contracts.OptionEod{first}, got)

	second := other
	second.CaptureID = "00000000-0000-4000-8000-0000000000ff"
	if err := w.InsertOptionEod(ctx, []contracts.OptionEod{second}); err != nil {
		t.Fatal(err)
	}
	got = readAll(t, r, []contracts.OptionEod{first})
	assertEqualRows(t, []contracts.OptionEod{second}, got)
}

func TestOptionEodCompressPG(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	rows := loadFixture(t)
	if err := openAs(t, name, "ot_w_option_eod").InsertOptionEod(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin := connectAs(t, name, "")
	var chunks []string
	rs, err := admin.Query(ctx, `SELECT c::text FROM show_chunks('obs.option_eod') AS c`)
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
		`WHERE hypertable_name = 'option_eod' AND is_compressed`).Scan(&compressed); err != nil {
		t.Fatal(err)
	}
	if compressed == 0 {
		t.Fatal("no chunk was compressed")
	}
	assertEqualRows(t, pickedFixture(rows), readAll(t, openAs(t, name, "ot_runner"), rows))
}

func TestOptionEodGrantsPG(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	rows := loadFixture(t)
	if err := openAs(t, name, "ot_w_option_eod").InsertOptionEod(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	w := connectAs(t, name, "ot_w_option_eod")
	// DELETE, TRUNCATE and an UPDATE of a key column are refused (migration 0047 widens only UPDATE of non-key columns and
	// SELECT of compared ones; this database applies 0006 alone, so the widened grants are checked by the setup tests).
	mustFail(t, w, `UPDATE obs.option_eod SET capture_id = gen_random_uuid()`)
	mustFail(t, w, `UPDATE obs.option_eod SET instrument_id = instrument_id + 1`)
	mustFail(t, w, `DELETE FROM obs.option_eod`)
	mustFail(t, w, `TRUNCATE obs.option_eod`)

	p := connectAs(t, name, "ot_ro_pages")
	mustFail(t, p, `SELECT count(*) FROM obs.option_eod`)
	var n int
	if err := p.QueryRow(context.Background(), `SELECT count(*) FROM rd.option_eod_r1`).Scan(&n); err != nil {
		t.Fatalf("view not readable: %v", err)
	}
	if n != len(rows) {
		t.Fatalf("view has %d rows, want %d", n, len(rows))
	}
	rn := connectAs(t, name, "ot_runner")
	mustFail(t, rn, `UPDATE obs.option_eod SET bid = 1`)
	mustFail(t, rn, `DELETE FROM obs.option_eod`)
}

func TestOptionEodChecksPG(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	admin := connectAs(t, name, "")
	ins := func(col, val string) string {
		return `INSERT INTO obs.option_eod (session_day, capture_id, instrument_id, underlying_id, generation, ` +
			`market_data_type, received_at, status, currency, snapshot_kind, underlying_ccy, ` + col + `) ` +
			`VALUES ('2026-11-06', gen_random_uuid(), 1, 1, 1, 1, now(), 1, 'USD', 'eod', 'USD', ` + val + `)`
	}
	mustFail(t, admin, ins("bid", "-1"))
	mustFail(t, admin, ins("delta", "'NaN'::numeric"))
	if _, err := admin.Exec(context.Background(), ins("delta", "-1")); err != nil {
		t.Fatalf("-1 in delta must meet the CHECK: %v", err)
	}
}

// pgEnv reads one libpq variable; only this test file reads the environment.
func pgEnv(key string) string {
	v, _ := os.LookupEnv(key)
	return v
}
