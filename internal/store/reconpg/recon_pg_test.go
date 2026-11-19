//go:build pg

package reconpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"

	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
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

// newDatabase makes a throwaway database with the roles and migration 0060 and returns its name.
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
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_w_recon"} {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r).Scan(&have); err != nil {
			t.Fatal(err)
		}
		verb := "ALTER"
		if !have {
			verb = "CREATE"
		}
		if _, err := admin.Exec(ctx, verb+" ROLE "+r+" LOGIN PASSWORD '"+rolePassword+"'"); err != nil {
			t.Fatalf("%s role %s: %v", verb, r, err)
		}
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
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	raw, err := os.ReadFile("../../../migrations/0060_recon.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0060: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_recon, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

// open returns the writer as ot_w_recon and the reader as ot_ro_pages on a fresh database.
func open2(t *testing.T) (reconstore.Writer, reconstore.Reader, string) {
	t.Helper()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := OpenWriter(ctx, connURL(t, name, "ot_w_recon"))
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

func run(pair, day string, finished time.Time, items ...reconstore.Item) reconstore.Run {
	return reconstore.Run{PairID: pair, SessionDay: contracts.Day(day), Status: "ok", StartedAt: finished.Add(-time.Minute),
		FinishedAt: finished, N: 10, Matched: 9, Diff: 1, PairsAsked: 1, PairsCompared: 1, ToleranceText: "tol", SourceID: "yfinance", Items: items}
}

func TestRerunNewRunLatestNewest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, r, _ := open2(t)
	at := time.Date(2026, 11, 18, 22, 0, 0, 0, time.UTC)
	first, err := w.InsertRun(ctx, run("R6", "2026-11-18", at, reconstore.Item{Key: "a", Company: "AAPL", Kind: "diff"}))
	if err != nil {
		t.Fatal(err)
	}
	second, err := w.InsertRun(ctx, run("R6", "2026-11-18", at.Add(time.Hour)))
	if err != nil || second == first {
		t.Fatalf("second run id %d (first %d), %v", second, first, err)
	}
	if _, err := w.InsertRun(ctx, run("R6", "2026-11-17", at)); err != nil {
		t.Fatal(err)
	}
	got, err := r.Latest(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("Latest = %d runs, %v", len(got), err)
	}
	if got[0].SessionDay != "2026-11-17" || got[1].SessionDay != "2026-11-18" || got[1].PairID != "R6" {
		t.Errorf("runs = %+v", got)
	}
	if got[1].Items == nil || len(got[1].Items) != 0 {
		t.Errorf("the newest run has the items %v, want none and non-nil", got[1].Items)
	}
	if !got[1].FinishedAt.Equal(at.Add(time.Hour)) || got[1].Skipped != 0 || got[1].SourceID != "yfinance" {
		t.Errorf("newest run = %+v", got[1])
	}
}

func TestItemsCapAtTwenty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, r, _ := open2(t)
	var items []reconstore.Item
	for i := 0; i < 25; i++ {
		items = append(items, reconstore.Item{Key: fmt.Sprintf("k%02d", i), Company: "C", Kind: "diff"})
	}
	if _, err := w.InsertRun(ctx, run("R6", "2026-11-18", time.Now(), items...)); err != nil {
		t.Fatal(err)
	}
	got, err := r.Latest(ctx)
	if err != nil || len(got) != 1 {
		t.Fatalf("Latest = %v, %v", got, err)
	}
	if len(got[0].Items) != reconstore.LatestItemsMax {
		t.Fatalf("%d items, want %d", len(got[0].Items), reconstore.LatestItemsMax)
	}
}

func TestItemsOrderedBySeverity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, r, _ := open2(t)
	_, err := w.InsertRun(ctx, run("R1", "2026-11-18", time.Now(),
		reconstore.Item{Key: "k1", Company: "A", Kind: "skipped", Reason: "no_data"},
		reconstore.Item{Key: "k2", Company: "B", Kind: "missing_other"},
		reconstore.Item{Key: "k3", Company: "B", Kind: "diff", Ours: "1", Other: "2", Diff: "-1"},
		reconstore.Item{Key: "k4", Company: "A", Kind: "missing_ours"},
		reconstore.Item{Key: "k5", Company: "A", Kind: "diff"}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Latest(ctx)
	if err != nil || len(got) != 1 {
		t.Fatalf("Latest = %v, %v", got, err)
	}
	var keys []string
	for _, it := range got[0].Items {
		keys = append(keys, it.Key)
	}
	if want := "k5 k3 k4 k2 k1"; strings.Join(keys, " ") != want {
		t.Errorf("order = %v, want %s", keys, want)
	}
	if it := got[0].Items[1]; it.Ours != "1" || it.Other != "2" || it.Diff != "-1" {
		t.Errorf("item = %+v", it)
	}
	if got[0].Items[4].Reason != "no_data" {
		t.Errorf("reason = %q", got[0].Items[4].Reason)
	}
}

func TestWriterRoleCannotUpdateOrDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, _, name := open2(t)
	if _, err := w.InsertRun(ctx, run("R6", "2026-11-18", time.Now(), reconstore.Item{Key: "k", Kind: "diff"})); err != nil {
		t.Fatal(err)
	}
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_w_recon"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, sql := range []string{`UPDATE recon.run SET status = 'failed'`, `DELETE FROM recon.run`, `UPDATE recon.item SET kind = 'skipped'`, `DELETE FROM recon.item`} {
		if _, err := c.Exec(ctx, sql); err == nil {
			t.Errorf("%s was allowed", sql)
		}
	}
}

func TestPageRoleReadsTheViewOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, _, name := open2(t)
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, sql := range []string{`SELECT 1 FROM recon.run`, `SELECT 1 FROM recon.item`} {
		if _, err := c.Exec(ctx, sql); err == nil {
			t.Errorf("%s was allowed", sql)
		}
	}
	if _, err := c.Exec(ctx, `SELECT 1 FROM rd.recon_latest_r1`); err != nil {
		t.Errorf("view: %v", err)
	}
}
