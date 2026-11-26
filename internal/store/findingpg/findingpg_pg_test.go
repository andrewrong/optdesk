//go:build pg

package findingpg

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

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

const (
	rolePassword  = "pg-test-only"
	migrationFile = "../../../migrations/0073_data_check.sql"
)

// connURL builds the URL of database dbname on the server the libpq variables name; user "" is the administrator.
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

// stubViews are the views the migration grants on and entry 5 reads; empty stand-ins are enough here.
var stubViews = []string{"coverage_day_r1", "session_day_r1", "reading_r1", "option_open_r1", "option_candidate_r1",
	"rule_param_r1", "option_exclusion_r1", "job_gap_r1"}

// newDatabase makes a throwaway database with the roles and migration 0073, dropped in t.Cleanup.
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
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(864)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_w_datacheck"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(864)`)
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
	raw, err := os.ReadFile(migrationFile)
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS rd`); err != nil {
		t.Fatal(err)
	}
	for _, v := range stubViews {
		if _, err := c.Exec(ctx, `CREATE VIEW rd.`+v+` AS SELECT 1 AS stub`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_datacheck, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

func open(t *testing.T) (finding.Writer, finding.Reader, string) {
	t.Helper()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := OpenWriter(ctx, connURL(t, name, "ot_w_datacheck"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	r, err := OpenReader(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return w, r, name
}

func fd(rule, dataset, day string, sev finding.Severity) finding.Finding {
	return finding.Finding{Rule: rule, Dataset: dataset, Day: contracts.Day(day), Severity: sev,
		Evidence: []finding.Count{{Name: "missing", N: 2}}}
}

func run(day string, fs ...finding.Finding) finding.Run {
	return finding.Run{CheckDay: contracts.Day(day), RulesTotal: 6, Findings: fs}
}

func TestIsNewUntilNoticed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, _, _ := open(t)
	f := fd(finding.RuleCaptureGap, "bars.eod", "2026-11-24", finding.SeverityWarn)

	r1, err := w.InsertRun(ctx, run("2026-11-25", f))
	if err != nil || !r1.Findings[0].IsNew {
		t.Fatalf("first run: %+v, %v", r1, err)
	}
	// An earlier run with no notice row: the same key is new again.
	r2, err := w.InsertRun(ctx, run("2026-11-25", f))
	if err != nil || !r2.Findings[0].IsNew {
		t.Fatalf("second run before notice: %+v, %v", r2, err)
	}
	if err := w.MarkNoticed(ctx, r2.RunID); err != nil {
		t.Fatal(err)
	}
	r3, err := w.InsertRun(ctx, run("2026-11-26", f, fd(finding.RulePlanAbsent, "bars.eod", "2026-11-24", finding.SeverityWarn)))
	if err != nil {
		t.Fatal(err)
	}
	if r3.RunID <= r2.RunID || r3.Findings[0].IsNew || !r3.Findings[1].IsNew {
		t.Fatalf("after notice: %+v", r3)
	}
}

func TestMarkNoticedTwice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, _, db := open(t)
	r, err := w.InsertRun(ctx, run("2026-11-25"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := w.MarkNoticed(ctx, r.RunID); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	admin, err := pgx.Connect(ctx, connURL(t, db, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close(ctx) }()
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM datacheck.notice WHERE run_id = $1`, r.RunID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("notice rows = %d, %v", n, err)
	}
}

func TestLatestOrdersAndPicksNewestDay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, rd, _ := open(t)

	empty, err := rd.Latest(ctx)
	if err != nil || empty.RunID != 0 || empty.Findings == nil || len(empty.Findings) != 0 {
		t.Fatalf("empty = %+v, %v", empty, err)
	}
	if _, err := w.InsertRun(ctx, run("2026-11-25",
		fd(finding.RuleGapUnclosed, "z", "2026-11-20", finding.SeverityUnknown),
		fd(finding.RulePlanAbsent, "b", "2026-11-21", finding.SeverityWarn),
		fd(finding.RuleCaptureGap, "a", "2026-11-22", finding.SeverityWarn),
		fd(finding.RuleReadingRepeat, "q", "2026-11-23", finding.SeverityError))); err != nil {
		t.Fatal(err)
	}
	// A forced run of an older day has the higher run_id and must not replace the newest day.
	if _, err := w.InsertRun(ctx, run("2026-11-20")); err != nil {
		t.Fatal(err)
	}
	got, err := rd.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.CheckDay != "2026-11-25" || got.RulesTotal != 6 || len(got.Findings) != 4 {
		t.Fatalf("latest = %+v", got)
	}
	var order []string
	for _, f := range got.Findings {
		order = append(order, f.Dataset)
	}
	if strings.Join(order, "") != "qabz" {
		t.Fatalf("order = %v", order)
	}
	if ev := got.Findings[0].Evidence; len(ev) != 1 || ev[0] != (finding.Count{Name: "missing", N: 2}) {
		t.Fatalf("evidence = %+v", ev)
	}
}
