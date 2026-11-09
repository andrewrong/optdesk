//go:build pg

package barpg

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

	"github.com/andrewrong/optdesk/internal/contracts"
)

const rolePassword = "pg-test-only"

// connURL builds the URL of database dbname on the server the libpq variables (PGHOST, PGPORT, PGUSER, PGPASSWORD,
// PGSSLMODE) name; pgx reads them itself, so no variable is read here. user "" is the administrator.
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

// adminDatabase is the database the administrator connects to (PGDATABASE).
func adminDatabase(t *testing.T) string {
	t.Helper()
	cfg, err := pgx.ParseConfig("")
	if err != nil || cfg.Database == "" {
		t.Fatalf("PGDATABASE names no database: %v", err)
	}
	return cfg.Database
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles, the schema rd and migration 0021, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, adminDatabase(t), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(863)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_w_bar"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(863)`)
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
	raw, err := os.ReadFile("../../../migrations/0021_bars.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c := pgxAs(t, name)
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS rd`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0021: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_bar, ot_ro_pages`); err != nil {
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

func count(t *testing.T, c *pgx.Conn, table string) int {
	t.Helper()
	var n int
	if err := c.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func pbar(id int, basis string, epoch int, d, closeText string) contracts.PriceBar {
	r := contracts.PriceBar{
		InstrumentID: id, AdjustEpoch: epoch, Open: closeText, High: closeText, Low: closeText, Close: closeText,
		Currency: "USD", ReceivedAt: time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC),
	}
	r.SessionDay.Time, _ = time.Parse(time.DateOnly, d)
	setText(&r.Basis, basis)
	setText(&r.Source, "ibkr")
	return r
}

func TestBarRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_bar"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	admin := pgxAs(t, name)

	vol := "1200"
	rows := []contracts.PriceBar{pbar(7, "trades", 1, "2026-11-02", "10.50"), pbar(7, "trades", 1, "2026-11-03", "11")}
	rows[0].Volume = &vol
	n, err := st.InsertPriceBars(ctx, rows)
	if err != nil || n != 2 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	// A key that exists adds nothing and keeps its value.
	again := []contracts.PriceBar{pbar(7, "trades", 1, "2026-11-03", "99"), pbar(7, "trades", 1, "2026-11-04", "12")}
	if n, err := st.InsertPriceBars(ctx, again); err != nil || n != 1 {
		t.Fatalf("reinsert = %d, %v", n, err)
	}
	if c := count(t, admin, "obs.price_bar"); c != 3 {
		t.Errorf("rows = %d, want 3", c)
	}
	got, err := st.PriceBars(ctx, 7, "trades", "2026-11-03", "2026-11-04")
	if err != nil || len(got) != 2 || got[0].Close != "11" || got[1].Close != "12" || got[0].Volume != nil {
		t.Fatalf("bars = %+v, %v", got, err)
	}
	all, _ := st.PriceBars(ctx, 7, "trades", "2026-11-01", "2026-11-30")
	if len(all) != 3 || all[0].Volume == nil || *all[0].Volume != "1200" || all[0].SessionDay.Format(time.DateOnly) != "2026-11-02" {
		t.Errorf("all = %+v", all)
	}

	checkFxRoundTrip(ctx, t, st, rows[0].ReceivedAt)
	checkBadRowRollsBack(ctx, t, st)
}

func checkFxRoundTrip(ctx context.Context, t *testing.T, st *Store, at time.Time) {
	t.Helper()
	fx := contracts.FxRate{Pair: "USDJPY", Rate: "151.25", ReceivedAt: at}
	setText(&fx.Source, "ibkr_midpoint")
	fx.RateDay.Time = time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	// The duplicate inside one call adds nothing.
	if n, err := st.InsertFxRates(ctx, []contracts.FxRate{fx, fx}); err != nil || n != 1 {
		t.Fatalf("fx insert = %d, %v", n, err)
	}
	if r, found, err := st.LatestFxRate(ctx, "USDJPY", "2026-11-09"); err != nil || !found || r.Rate != "151.25" {
		t.Errorf("latest = %+v %v %v", r, found, err)
	}
	if r, found, err := st.LatestFxRate(ctx, "USDJPY", "2026-11-05"); err != nil || found || r != (contracts.FxRate{}) {
		t.Errorf("before first = %+v %v %v", r, found, err)
	}
	if rs, err := st.FxRates(ctx, "USDJPY", "2026-11-01", "2026-11-30"); err != nil || len(rs) != 1 {
		t.Errorf("fx rates = %+v, %v", rs, err)
	}

}

// checkBadRowRollsBack: a bad row fails the whole call and leaves none of its rows.
func checkBadRowRollsBack(ctx context.Context, t *testing.T, st *Store) {
	t.Helper()
	bad := []contracts.PriceBar{pbar(8, "trades", 1, "2026-11-02", "5"), pbar(8, "trades", 1, "2026-11-03", "-5")}
	if _, err := st.InsertPriceBars(ctx, bad); err == nil {
		t.Error("negative price accepted")
	}
	if none, _ := st.PriceBars(ctx, 8, "trades", "2026-11-01", "2026-11-30"); len(none) != 0 {
		t.Errorf("half call visible: %+v", none)
	}
}

func TestNewEpochWins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_bar"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	first := []contracts.PriceBar{
		pbar(7, "trades", 1, "2026-11-02", "100"), pbar(7, "trades", 1, "2026-11-03", "101"),
		pbar(7, "adjusted_last", 1, "2026-11-03", "90"),
	}
	if _, err := st.InsertPriceBars(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertPriceBars(ctx, []contracts.PriceBar{pbar(7, "trades", 2, "2026-11-03", "50"), pbar(7, "trades", 2, "2026-11-04", "51")}); err != nil {
		t.Fatal(err)
	}
	got, err := st.PriceBars(ctx, 7, "trades", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 2 || got[0].Close != "50" || got[0].AdjustEpoch != 2 || got[1].Close != "51" {
		t.Fatalf("trades winners = %+v, %v", got, err)
	}
	adj, err := st.PriceBars(ctx, 7, "adjusted_last", "2026-11-01", "2026-11-30")
	if err != nil || len(adj) != 1 || adj[0].AdjustEpoch != 1 {
		t.Errorf("adjusted winners = %+v, %v", adj, err)
	}
}

func TestBarRoleGrants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w := pgxAsUser(t, name, "ot_w_bar")
	denied := func(what, sql string) {
		t.Helper()
		if _, err := w.Exec(ctx, sql); err == nil {
			t.Errorf("%s: allowed", what)
		}
	}
	denied("select table", `SELECT 1 FROM obs.price_bar`)
	denied("update", `UPDATE obs.price_bar SET close = '1'`)
	denied("delete", `DELETE FROM obs.fx_rate`)
	denied("truncate", `TRUNCATE obs.price_bar`)
	for _, s := range []string{`SELECT 1 FROM rd.price_bar_r1`, `SELECT 1 FROM rd.fx_rate_r1`,
		`INSERT INTO obs.price_bar VALUES (1, '2026-11-02', 'trades', 1, '1', '1', '1', '1', NULL, 'USD', 'ibkr', now())`} {
		if _, err := w.Exec(ctx, s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	denied("bad basis", `INSERT INTO obs.price_bar VALUES (1, '2026-11-02', 'other', 1, '1', '1', '1', '1', NULL, 'USD', 'ibkr', now())`)
	denied("epoch 0", `INSERT INTO obs.price_bar VALUES (1, '2026-11-02', 'trades', 0, '1', '1', '1', '1', NULL, 'USD', 'ibkr', now())`)
	denied("bad pair", `INSERT INTO obs.fx_rate VALUES ('usdjpy', '2026-11-02', '1', 'ibkr_midpoint', now())`)
	denied("zero rate", `INSERT INTO obs.fx_rate VALUES ('USDJPY', '2026-11-02', '0', 'ibkr_midpoint', now())`)

	p := pgxAsUser(t, name, "ot_ro_pages")
	for _, v := range []string{"rd.price_bar_r1", "rd.fx_rate_r1"} {
		if _, err := p.Exec(ctx, `SELECT 1 FROM `+v); err != nil {
			t.Errorf("pages read %s: %v", v, err)
		}
	}
	if _, err := p.Exec(ctx, `SELECT 1 FROM obs.price_bar`); err == nil {
		t.Error("pages read the table")
	}
}

func pgxAsUser(t *testing.T, name, user string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatalf("connect %s: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}
