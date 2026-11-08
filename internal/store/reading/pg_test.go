//go:build pg

package reading

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const rolePassword = "pg-test-only"

func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	for _, v := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"} {
		if os.Getenv(v) == "" {
			t.Fatalf("missing environment variable %s", v)
		}
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(os.Getenv("PGUSER"), os.Getenv("PGPASSWORD")),
		Host:   os.Getenv("PGHOST") + ":" + os.Getenv("PGPORT"),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	if m := os.Getenv("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles and migrations 0013 and 0016, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, os.Getenv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_runner", "ot_ro_pages", "ot_w_readings"} {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r).Scan(&have); err != nil {
			t.Fatal(err)
		}
		if !have {
			if _, err := admin.Exec(ctx, "CREATE ROLE "+r+" LOGIN PASSWORD '"+rolePassword+"'"); err != nil {
				t.Fatalf("create role %s: %v", r, err)
			}
		}
	}
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
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
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, file := range []string{"0013_readings.sql", "0016_reading_parameter_view.sql"} {
		raw, err := os.ReadFile("../../../migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(raw), "-- +goose Down")
		if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
	// Migration 0009 gives the page role USAGE on schema rd; this database has only 0013.
	if _, err := c.Exec(ctx, `GRANT USAGE ON SCHEMA rd TO ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_runner", "ot_ro_pages", "ot_w_readings"} {
		if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO `+r); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

func TestInsertAndWinnerPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_readings"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	one := rec(1, "ok")
	n, err := st.InsertReadings(ctx, []contracts.ReadingRecord{one})
	if err != nil || n != 1 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	if n, err = st.InsertReadings(ctx, []contracts.ReadingRecord{one}); err != nil || n != 0 {
		t.Fatalf("replay = %d, %v", n, err)
	}
	two := rec(2, "ok")
	two.Value = strPtr("0.750000000000")
	if n, err = st.InsertReadings(ctx, []contracts.ReadingRecord{two}); err != nil || n != 1 {
		t.Fatalf("rev 2 = %d, %v", n, err)
	}
	got, err := st.LatestReadings(ctx, one.Name, one.SubjectKind, one.Subject, "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 || got[0].Rev != 2 || *got[0].Value != "0.750000000000" {
		t.Fatalf("latest = %+v, %v", got, err)
	}
	if len(got[0].Inputs) != 1 || len(got[0].Inputs[0].Keys) != 2 || len(got[0].Parameters) != 1 {
		t.Errorf("children = %+v %+v", got[0].Inputs, got[0].Parameters)
	}

	// A missing_input rev 3 is the latest, but the view keeps the ok rev 2.
	three := rec(3, "missing_input")
	three.Value = nil
	if _, err = st.InsertReadings(ctx, []contracts.ReadingRecord{three}); err != nil {
		t.Fatal(err)
	}
	if got, _ = st.LatestReadings(ctx, one.Name, one.SubjectKind, one.Subject, "2026-11-01", "2026-11-30"); len(got) != 1 || got[0].Rev != 3 {
		t.Errorf("latest after missing = %+v", got)
	}
	admin := pgxAs(t, name, "")
	var rev int
	if err := admin.QueryRow(ctx, `SELECT rev FROM rd.reading_r1 WHERE name = $1`, one.Name).Scan(&rev); err != nil || rev != 2 {
		t.Errorf("winner rev = %d, %v; want 2", rev, err)
	}
}

func pgxAs(t *testing.T, name, user string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatalf("connect as %q: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func TestGrantsPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w := pgxAs(t, name, "ot_w_readings")
	for _, sql := range []string{
		`UPDATE readings.reading SET rev = rev`, `DELETE FROM readings.reading`, `TRUNCATE readings.reading`,
		`SELECT 1 FROM master.session_day`,
	} {
		if _, err := w.Exec(ctx, sql); err == nil {
			t.Errorf("ot_w_readings may run %s", sql)
		}
	}
	if _, err := w.Exec(ctx, `SELECT * FROM rd.reading_r1`); err != nil {
		t.Errorf("ot_w_readings cannot read the view: %v", err)
	}
	pages := pgxAs(t, name, "ot_ro_pages")
	if _, err := pages.Exec(ctx, `SELECT * FROM rd.reading_r1`); err != nil {
		t.Errorf("ot_ro_pages cannot read the view: %v", err)
	}
	for _, sql := range []string{`SELECT 1 FROM readings.reading`, `INSERT INTO readings.reading_parameter (name) VALUES ('x')`} {
		if _, err := pages.Exec(ctx, sql); err == nil {
			t.Errorf("ot_ro_pages may run %s", sql)
		}
	}
	runner := pgxAs(t, name, "ot_runner")
	for _, sql := range []string{`SELECT 1 FROM rd.reading_r1`, `SELECT 1 FROM readings.reading`} {
		if _, err := runner.Exec(ctx, sql); err == nil {
			t.Errorf("ot_runner may run %s", sql)
		}
	}
}
