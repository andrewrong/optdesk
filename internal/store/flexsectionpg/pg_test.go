//go:build pg

package flexsectionpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

// env reads one process environment variable (the pg tests take their connection from PG*).
func env(k string) string {
	v, _ := os.LookupEnv(k)
	return v
}

const rolePassword = "pg-test-only"

func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	for _, v := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"} {
		if env(v) == "" {
			t.Fatalf("missing environment variable %s", v)
		}
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(env("PGUSER"), env("PGPASSWORD")),
		Host:   env("PGHOST") + ":" + env("PGPORT"),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	if m := env("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles and migrations 0022 and 0024, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, env("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_w_optionledger", "ot_ro_pages"} {
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
	for _, f := range []string{"0022_optionledger.sql", "0024_flex_section.sql"} {
		raw, err := os.ReadFile("../../../migrations/" + f)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(raw), "-- +goose Down")
		if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_optionledger, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestPGRecordAndRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_optionledger"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	at := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	r := func(k, h string) flexsection.Row {
		return flexsection.Row{Alias: "a", Section: "CashTransaction", RowKey: k, Attributes: `{"v":"` + h + `"}`, AttributesSHA256: h, RecordedAt: at}
	}
	status := []flexsection.Status{{Alias: "a", Section: "CashTransaction", State: "present", RowCount: 2, RecordedAt: at}}
	if got, err := st.RecordSections(ctx, "f1", []flexsection.Row{r("k1", "h1"), r("k2", "h2")}, status); err != nil || got.New != 2 {
		t.Fatalf("first = %+v, %v", got, err)
	}
	if got, err := st.RecordSections(ctx, "f2", []flexsection.Row{r("k1", "h1"), r("k2", "h3")}, nil); err != nil || got.Unchanged != 1 || got.Revised != 1 {
		t.Fatalf("second = %+v, %v", got, err)
	}
	rows, err := st.Rows(ctx, "a", "CashTransaction")
	if err != nil || len(rows) != 2 || rows[0].Revision != 1 || rows[1].Revision != 2 {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	if sts, err := st.Statuses(ctx, "a"); err != nil || len(sts) != 1 || sts[0].State != "present" {
		t.Fatalf("statuses = %+v, %v", sts, err)
	}
}

// TestWriterRoleCannotUpdate shows the grants: the role inserts and reads, and an UPDATE, DELETE or TRUNCATE
// fails with SQLSTATE 42501.
func TestWriterRoleCannotUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_optionledger"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	row := flexsection.Row{Alias: "a", Section: "S", RowKey: "k", Attributes: `{}`, AttributesSHA256: "h"}
	status := []flexsection.Status{{Alias: "a", Section: "S", State: "absent"}}
	if _, err := st.RecordSections(ctx, "f1", []flexsection.Row{row}, status); err != nil {
		t.Fatal(err)
	}
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_w_optionledger"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, sql := range []string{
		`UPDATE optionledger.flex_section_row SET attributes = '{}'`,
		`DELETE FROM optionledger.flex_section_row`,
		`TRUNCATE optionledger.flex_section_row`,
		`UPDATE optionledger.flex_section_status SET state = 'present'`,
		`DELETE FROM optionledger.flex_section_status`,
		`TRUNCATE optionledger.flex_section_status`,
	} {
		_, err := c.Exec(ctx, sql)
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || pe.Code != "42501" {
			t.Errorf("%q: err = %v, want SQLSTATE 42501", sql, err)
		}
	}
}
