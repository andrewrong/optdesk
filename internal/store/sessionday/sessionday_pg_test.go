//go:build pg

package sessionday

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

// libpqVars are the connection variables the pg tests read; none has a default.
var libpqVars = []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"}

// adminURL builds the connection URL of database dbname from the libpq environment.
func adminURL(t *testing.T, dbname string) string {
	t.Helper()
	var missing []string
	for _, v := range libpqVars {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(os.Getenv("PGUSER"), os.Getenv("PGPASSWORD")),
		Host:   os.Getenv("PGHOST") + ":" + os.Getenv("PGPORT"),
		Path:   "/" + dbname,
	}
	if m := os.Getenv("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase creates a database named from the test name and a random suffix, applies the migration there
// and drops it in t.Cleanup. It returns the connection URL of the new database.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminURL(t, os.Getenv("PGDATABASE")))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
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
	dsn := adminURL(t, name)
	applyMigration(t, dsn)
	return dsn
}

// applyMigration runs the Up section of migrations/0001_master_session_day.sql on the database at dsn.
func applyMigration(t *testing.T, dsn string) {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/0001_master_session_day.sql")
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
		t.Fatalf("apply migration: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := Open(ctx, newDatabase(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	rows := []contracts.SessionDay{
		{CalendarID: "XNYS", SessionDay: day(t, "2026-10-30"), IsSession: true, OpenAt: utc(2026, 10, 30, 13, 30), CloseAt: utc(2026, 10, 30, 20, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-02"), IsSession: true, OpenAt: utc(2026, 11, 2, 14, 30), CloseAt: utc(2026, 11, 2, 21, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-26"), IsSession: false, OpenAt: utc(2026, 11, 26, 14, 30), CloseAt: utc(2026, 11, 26, 21, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-27"), IsSession: true, EarlyClose: true,
			OpenAt: utc(2026, 11, 27, 14, 30), CloseAt: utc(2026, 11, 27, 18, 0)},
		{CalendarID: "XTKS", SessionDay: day(t, "2026-11-02"), IsSession: true, OpenAt: utc(2026, 11, 2, 0, 0), CloseAt: utc(2026, 11, 2, 6, 0)},
	}
	if err := s.UpsertSessionDays(ctx, rows); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.SessionDays(ctx, "XNYS", "2026-10-30", "2026-11-27")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	assertRows(t, got, rows[:4])

	// Both ends are inclusive and other calendars are excluded.
	got, err = s.SessionDays(ctx, "XNYS", "2026-11-02", "2026-11-26")
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, got, rows[1:3])
}

func TestUpsertReplacesRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := Open(ctx, newDatabase(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	first := contracts.SessionDay{CalendarID: "XNYS", SessionDay: day(t, "2026-11-27"), IsSession: true,
		OpenAt: utc(2026, 11, 27, 14, 30), CloseAt: utc(2026, 11, 27, 21, 0)}
	second := first
	second.EarlyClose, second.CloseAt = true, utc(2026, 11, 27, 18, 0)
	if err := s.UpsertSessionDays(ctx, []contracts.SessionDay{first}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSessionDays(ctx, []contracts.SessionDay{second}); err != nil {
		t.Fatal(err)
	}
	got, err := s.SessionDays(ctx, "XNYS", "2026-11-27", "2026-11-27")
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, got, []contracts.SessionDay{second})
}

func assertRows(t *testing.T, got, want []contracts.SessionDay) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.CalendarID != w.CalendarID || !g.SessionDay.Equal(w.SessionDay.Time) || g.IsSession != w.IsSession ||
			g.EarlyClose != w.EarlyClose || !g.OpenAt.Equal(w.OpenAt) || !g.CloseAt.Equal(w.CloseAt) {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
}
