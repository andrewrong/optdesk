//go:build pg

package candidate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
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

// newDatabase makes a throwaway database with the roles and migration 0018, dropped in t.Cleanup.
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
	for _, r := range []string{"ot_owner", "ot_w_candidate", "ot_ro_pages"} {
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
	for _, file := range []string{"0018_candidates.sql", "0027_candidate_settings_version.sql"} {
		raw, err := os.ReadFile("../../../migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(raw), "-- +goose Down")
		if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_candidate`); err != nil {
		t.Fatal(err)
	}
	return name
}

func pgSample(runID string, at time.Time, instrument int, rank *int) store.Candidate {
	return store.Candidate{
		SessionDay: "2026-11-06", RunID: runID, Kind: "short_put", InstrumentID: instrument, Alias: "",
		RuleVersion: "v1", Issuer: "AAPL", UnderlyingID: 7, Expiry: "2026-12-18", DTE: 42, Right: "put",
		Strike: "180", Multiplier: 100, BidEod: "2.15", Ask: ptr("2.2"), BreakEven: "177.85", Status: "ok",
		Gates: `[]`, Marks: `[]`, Fits: `{"A":"fit"}`, EarningsState: "listed", Rank: rank, ComputedAt: at,
	}
}

func TestCandidateRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_candidate"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	in := pgSample("r1", time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC), 42, ptr(1))
	in.NextEarnings = ptr(contracts.Day("2027-01-28"))
	if n, err := st.InsertCandidates(ctx, []store.Candidate{in}); err != nil || n != 1 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	if _, err := st.InsertCandidates(ctx, []store.Candidate{in}); err == nil {
		t.Error("a second row of the same key was accepted")
	}
	got, err := st.Candidates(ctx, "2026-11-06", "short_put")
	if err != nil || len(got) != 1 {
		t.Fatalf("read = %v, %v", got, err)
	}
	if !got[0].ComputedAt.Equal(in.ComputedAt) {
		t.Errorf("computed_at = %v, want %v", got[0].ComputedAt, in.ComputedAt)
	}
	got[0].ComputedAt = in.ComputedAt
	if !reflect.DeepEqual(got[0], in) {
		t.Errorf("row = %+v\nwant  %+v", got[0], in)
	}
	empty, err := st.Candidates(ctx, "2026-11-05", "short_put")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty day = %#v, %v", empty, err)
	}
}

func TestCandidateNewestRunWins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_candidate"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	t0 := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	rows := []store.Candidate{
		pgSample("old", t0, 42, ptr(1)),
		pgSample("new", t0.Add(time.Hour), 43, nil),
		pgSample("new", t0.Add(time.Hour), 42, ptr(1)),
	}
	if n, err := st.InsertCandidates(ctx, rows); err != nil || n != 3 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	got, err := st.Candidates(ctx, "2026-11-06", "short_put")
	if err != nil || len(got) != 2 {
		t.Fatalf("read = %d rows, %v", len(got), err)
	}
	if got[0].RunID != "new" || got[1].RunID != "new" || got[0].Rank == nil || got[1].Rank != nil {
		t.Errorf("rows = %+v", got)
	}
}

func TestSettingsVersionRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_candidate"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	t0 := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	old := pgSample("old", t0, 42, ptr(1))
	cur := pgSample("new", t0.Add(time.Hour), 42, ptr(1))
	cur.SettingsVersion = ptr(int64(57))
	if _, err := st.InsertCandidates(ctx, []store.Candidate{old}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Candidates(ctx, "2026-11-06", "short_put")
	if err != nil || len(got) != 1 || got[0].SettingsVersion != nil {
		t.Fatalf("old run = %+v, %v; want a nil settings version", got, err)
	}
	if _, err := st.InsertCandidates(ctx, []store.Candidate{cur}); err != nil {
		t.Fatal(err)
	}
	got, err = st.Candidates(ctx, "2026-11-06", "short_put")
	if err != nil || len(got) != 1 || got[0].SettingsVersion == nil || *got[0].SettingsVersion != 57 {
		t.Fatalf("new run = %+v, %v; want settings version 57", got, err)
	}
}
