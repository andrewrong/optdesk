//go:build pg

package markpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/store/mark"
)

const rolePassword = "pg-test-only"

func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		t.Fatalf("read libpq environment: %v", err)
	}
	if cfg.Host == "" || cfg.User == "" {
		t.Fatal("PGHOST and PGUSER name no server")
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	mode := "disable"
	if cfg.TLSConfig != nil {
		mode = "prefer"
	}
	u.RawQuery = url.Values{"sslmode": {mode}}.Encode()
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles, the schemas account and rd and migration 0026.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	cfg, err := pgx.ParseConfig("")
	if err != nil || cfg.Database == "" {
		t.Fatalf("PGDATABASE names no database: %v", err)
	}
	admin, err := pgx.Connect(ctx, connURL(t, cfg.Database, ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(962)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_runner", "ot_w_posmark"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(962)`)
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
	raw, err := os.ReadFile("../../../migrations/0026_position_mark.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c := pgxAs(t, name)
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS rd; CREATE SCHEMA IF NOT EXISTS account`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0026: %v", err)
	}
	// Earlier migrations (0006, 0007) grant USAGE on rd to ot_ro_pages; this database holds only 0026.
	if _, err := c.Exec(ctx, `GRANT USAGE ON SCHEMA rd TO ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_posmark, ot_ro_pages, ot_runner`); err != nil {
		t.Fatal(err)
	}
	return name
}

func pgxAs(t *testing.T, name string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, ""))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func mk(capture string, con int64, attempt int, basis, price string, at time.Time) mark.PositionMark {
	return mark.PositionMark{CaptureID: capture, Alias: "main", MarkedAt: at, SessionDay: "2026-11-09", ConID: con,
		Attempt: attempt, Currency: "USD", Basis: basis, Price: price, Status: "ok"}
}

func TestStoreWriteAndReadBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_posmark"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	t0 := time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC)

	if got, err := st.Marks(ctx, "main", "2026-11-09"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty: %#v %v", got, err)
	}
	if err := st.WriteMarks(ctx, []mark.PositionMark{mk("old", 5, 1, mark.BasisMid, "9", t0.Add(-time.Hour)),
		mk("c1", 20, 1, mark.BasisNone, "", t0), mk("c1", 10, 1, mark.BasisMid, "1.25", t0)}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteMarks(ctx, []mark.PositionMark{mk("c1", 10, 1, mark.BasisLast, "9", t0),
		mk("c1", 20, 2, mark.BasisClose, "3.5", t0.Add(time.Minute))}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Marks(ctx, "main", "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ConID != 10 || got[1].ConID != 20 || got[0].CaptureID != "c1" {
		t.Fatalf("want c1 rows 10, 20: %#v", got)
	}
	if got[0].Basis != mark.BasisMid || got[0].Price != "1.25" {
		t.Fatalf("existing key must stay: %#v", got[0])
	}
	if got[1].Attempt != 2 || got[1].Basis != mark.BasisClose || got[1].Price != "3.5" {
		t.Fatalf("highest attempt must win: %#v", got[1])
	}
	if got[0].Bid != "" || !got[0].MarkedAt.Equal(t0) || got[0].SessionDay != "2026-11-09" {
		t.Fatalf("mapping: %#v", got[0])
	}
	if other, _ := st.Marks(ctx, "spouse", "2026-11-09"); len(other) != 0 {
		t.Fatalf("accounts are never pooled: %#v", other)
	}
}

func TestRolesCannotUpdateAndPagesReadTheViewOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_posmark"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.WriteMarks(ctx, []mark.PositionMark{mk("c1", 1, 1, mark.BasisMid, "1", time.Now().UTC())}); err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"ot_w_posmark", "ot_ro_pages", "ot_runner"} {
		c, err := pgx.Connect(ctx, connURL(t, name, user))
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{`UPDATE account.position_mark SET price = 2`, `DELETE FROM account.position_mark`,
			`TRUNCATE account.position_mark`} {
			if _, err := c.Exec(ctx, q); err == nil {
				t.Errorf("%s: %s succeeded", user, q)
			}
		}
		_, tableErr := c.Exec(ctx, `SELECT count(*) FROM account.position_mark`)
		_, viewErr := c.Exec(ctx, `SELECT count(*) FROM rd.account_position_mark_r1`)
		switch user {
		case "ot_w_posmark":
			if tableErr != nil || viewErr != nil {
				t.Errorf("writer: table %v view %v", tableErr, viewErr)
			}
		case "ot_ro_pages":
			if tableErr == nil || viewErr != nil {
				t.Errorf("pages: table %v view %v", tableErr, viewErr)
			}
		case "ot_runner":
			if tableErr == nil || viewErr == nil {
				t.Errorf("runner: table %v view %v", tableErr, viewErr)
			}
		}
		_ = c.Close(ctx)
	}
}
