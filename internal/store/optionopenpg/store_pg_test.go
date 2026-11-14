//go:build pg

package optionopenpg

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

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
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

// newDatabase makes a throwaway database with the roles, the Timescale extension, a master.instrument table with
// ids 1 to 3 (the columns the foreign keys need) and migrations 0029 to 0032.
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
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_w_option_open"} {
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
	if _, err := c.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS timescaledb; CREATE SCHEMA master; `+
		`CREATE TABLE master.instrument (instrument_id bigint PRIMARY KEY); `+
		`INSERT INTO master.instrument VALUES (1), (2), (3), (4)`); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	for _, f := range []string{"0029_option_open.sql", "0030_option_open_plan.sql", "0031_option_not_listed.sql", "0032_option_contract.sql"} {
		raw, err := os.ReadFile("../../../migrations/" + f)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(raw), "-- +goose Down")
		if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_option_open, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

func day(s string) types.SerializableDate {
	d, _ := time.Parse(time.DateOnly, s)
	return types.SerializableDate{Time: d}
}

func f(v float64) *float64 { return &v }

func openRow(slot string, inst int, at time.Time, bid float64) contracts.OptionOpen {
	return contracts.OptionOpen{SessionDay: day("2026-11-11"), CaptureID: "00000000-0000-4000-8000-00000000000" + strconv.Itoa(len(slot)), Slot: slot,
		InstrumentID: inst, UnderlyingID: 1, PlanID: "00000000-0000-4000-8000-0000000000aa", Generation: 1, MarketDataType: 1,
		RequestedAt: at, ReceivedAt: at, ConnectionGeneration: 1, Status: 1, Currency: "USD", UnderlyingCcy: "USD",
		Bid: f(bid), Delta: f(-1), IvDecimal: f(0.3)}
}

// mustFail asserts that every statement is refused to the connection.
func mustFail(t *testing.T, c *pgx.Conn, sqls ...string) {
	t.Helper()
	for _, q := range sqls {
		if _, err := c.Exec(context.Background(), q); err == nil {
			t.Errorf("statement was allowed: %s", q)
		}
	}
}

func TestOptionOpenRoundTripAndGrants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := Open(ctx, connURL(t, name, "ot_w_option_open"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	t0 := time.Date(2026, 11, 11, 14, 45, 0, 0, time.UTC)
	rows := []contracts.OptionOpen{openRow("0945", 2, t0, 1.1), openRow("0945", 3, t0, 2.2), openRow("1000", 2, t0.Add(time.Minute), 1.5)}
	if err := w.InsertOptionOpen(ctx, rows); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertOptionOpen(ctx, rows); err != nil {
		t.Fatalf("a retry must add no row: %v", err)
	}
	// The pages role reads the same rows through the view and cannot touch the base table.
	r, err := Open(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	got, err := r.OptionOpen(ctx, "2026-11-11")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d rows, %v", len(got), err)
	}
	if got[0].InstrumentID != 2 || got[0].Slot != "1000" || *got[0].Bid != 1.5 || got[0].Delta != nil {
		t.Errorf("newest of instrument 2 = %+v", got[0])
	}
	if !got[0].ReceivedAt.Equal(t0.Add(time.Minute)) || got[0].ReceivedAt.Location() != time.UTC {
		t.Errorf("received_at = %v", got[0].ReceivedAt)
	}
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	mustFail(t, c, `SELECT 1 FROM obs.option_open`, `SELECT 1 FROM plan.option_open_item`,
		`SELECT 1 FROM master.option_contract`, `SELECT 1 FROM master.option_not_listed`)
	if _, err := r.Plan(ctx, "2026-11-11", "0945"); err != nil {
		t.Errorf("the pages role must read the plan view: %v", err)
	}
	// A writer cannot delete, truncate or update a key column (migration 0047 grants UPDATE of the non-key columns and
	// SELECT of the compared ones).
	wc, err := pgx.Connect(ctx, connURL(t, name, "ot_w_option_open"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wc.Close(ctx) }()
	mustFail(t, wc, `UPDATE obs.option_open SET slot = '0000'`, `UPDATE obs.option_open SET capture_id = gen_random_uuid()`,
		`DELETE FROM obs.option_open`, `TRUNCATE obs.option_open`,
		`DELETE FROM plan.option_open_item`, `DELETE FROM master.option_contract`)
	if _, err := wc.Exec(ctx, `UPDATE obs.option_open SET skew = skew WHERE false`); err != nil {
		t.Errorf("UPDATE of a non-key column must be granted: %v", err)
	}
	var wn int
	if err := wc.QueryRow(ctx, `SELECT count(session_day) FROM obs.option_open`).Scan(&wn); err != nil {
		t.Errorf("SELECT of a compared column must be granted: %v", err)
	}
	bad := openRow("refresh", 2, t0, 1)
	if err := w.InsertOptionOpen(ctx, []contracts.OptionOpen{bad}); err == nil {
		t.Error("a refresh row was stored")
	}
}

func TestPlanLatestPerUnderlying(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	s, err := Open(ctx, connURL(t, name, "ot_w_option_open"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	t0 := time.Date(2026, 11, 11, 13, 45, 0, 0, time.UTC)
	it := func(plan string, und, inst, rank int, at time.Time) contracts.OptionOpenItem {
		return contracts.OptionOpenItem{SessionDay: day("2026-11-11"), Slot: "0945", PlanID: plan, UnderlyingID: und, InstrumentID: inst,
			PlannedAt: at, Class: "puts", Rank: rank, PlannedSpot: 100.5, Sigma: 0.3, SigmaSource: "default"}
	}
	const old, cur, other = "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003"
	if err := s.PutPlan(ctx, []contracts.OptionOpenItem{
		it(old, 1, 2, 0, t0), it(cur, 1, 3, 1, t0.Add(time.Minute)), it(cur, 1, 4, 0, t0.Add(time.Minute)), it(other, 2, 3, 0, t0),
	}); err != nil {
		t.Fatal(err)
	}
	all, err := s.Plan(ctx, "2026-11-11", "0945")
	if err != nil || len(all) != 3 || all[0].InstrumentID != 4 || all[1].InstrumentID != 3 || all[2].UnderlyingID != 2 {
		t.Fatalf("all = %+v, %v", all, err)
	}
	if all[0].PlannedSpot != 100.5 || all[0].Class != "puts" {
		t.Errorf("item = %+v", all[0])
	}
	one, err := s.Plan(ctx, "2026-11-11", "0945", 2)
	if err != nil || len(one) != 1 || one[0].PlanID != other {
		t.Fatalf("one = %+v, %v", one, err)
	}
}

func TestNotListedLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	s, err := Open(ctx, connURL(t, name, "ot_w_option_open"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	now := time.Date(2026, 11, 11, 1, 0, 0, 0, time.UTC)
	m := func(und int, exp, strike string, retry time.Time) contracts.OptionNotListed {
		return contracts.OptionNotListed{UnderlyingID: und, Expiry: day(exp), Strike: strike, Right: "P",
			TradingClass: "AAPL", CheckedAt: now.Add(-24 * time.Hour), RetryAt: retry}
	}
	if err := s.AddNotListed(ctx, []contracts.OptionNotListed{
		m(1, "2026-11-20", "100", now.Add(time.Hour)), m(1, "2026-12-18", "95.5", now.Add(-time.Hour)),
		m(2, "2026-12-18", "9", now.Add(time.Hour)),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.NotListed(ctx, now)
	if err != nil || len(got) != 2 || got[0].Strike != "100" {
		t.Fatalf("not yet due = %+v, %v", got, err)
	}
	// The same contract again replaces the mark.
	if err := s.AddNotListed(ctx, []contracts.OptionNotListed{m(1, "2026-12-18", "95.5", now.Add(48*time.Hour))}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.NotListed(ctx, now); len(got) != 3 {
		t.Fatalf("after replace = %d, want 3", len(got))
	}
	if n, err := s.RemoveNotListedExpired(ctx, "2026-12-01"); err != nil || n != 1 {
		t.Fatalf("expired = %d, %v", n, err)
	}
	if n, err := s.RemoveNotListedByUnderlying(ctx, 2); err != nil || n != 1 {
		t.Fatalf("by underlying = %d, %v", n, err)
	}
	k := openquote.NotListedKey{UnderlyingID: 1, Expiry: "2026-12-18", Strike: "95.5", Right: "P", TradingClass: "AAPL"}
	if ok, err := s.RemoveNotListed(ctx, k); err != nil || !ok {
		t.Fatalf("remove = %v, %v", ok, err)
	}
	if ok, _ := s.RemoveNotListed(ctx, k); ok {
		t.Error("removed twice")
	}
}

func TestOptionContractPutAndList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	s, err := Open(ctx, connURL(t, name, "ot_w_option_open"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ref := func(exp, strike, right string, inst int64) openquote.OptionRef {
		return openquote.OptionRef{UnderlyingID: 1, Currency: "USD", Exchange: "SMART", TradingClass: "AAPL", Multiplier: "100",
			Expiry: contracts.Day(exp), Strike: strike, Right: right, InstrumentID: inst}
	}
	for _, r := range []openquote.OptionRef{ref("2026-12-18", "100", "P", 2), ref("2026-12-18", "95", "P", 3), ref("2026-11-20", "100", "C", 4)} {
		if err := s.PutOption(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	// The contract again, with a multiplier written another way, is the same contract with a new instrument id.
	again := ref("2026-12-18", "95", "P", 2)
	again.Multiplier = "100.0"
	if err := s.PutOption(ctx, again); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListOptions(ctx, 1)
	if err != nil || len(got) != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].Right != "C" || got[1].Strike != "95" || got[1].InstrumentID != 2 || got[2].Strike != "100" {
		t.Errorf("order or update wrong: %+v", got)
	}
	if none, err := s.ListOptions(ctx, 3); err != nil || none == nil || len(none) != 0 {
		t.Errorf("none = %#v, %v", none, err)
	}
}
