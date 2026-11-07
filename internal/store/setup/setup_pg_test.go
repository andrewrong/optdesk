//go:build pg

package setup

import (
	"bytes"
	"context"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/store"
)

// testPassword is the password the other pg tests of the repository use for the week-1 roles. The .env the test
// starts from carries it, so Setup keeps it and does not change the roles under tests that run in parallel.
const testPassword = "pg-test-only"

func adminDSN(t *testing.T, dbname string) string {
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
	if m := os.Getenv("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

func TestSetupPG(t *testing.T) {
	ctx := context.Background()
	dsn := adminDSN(t, os.Getenv("PGDATABASE"))
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close(ctx) }()
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = c.Close(ctx) }()
		if _, err := c.Exec(ctx, `DROP DATABASE IF EXISTS `+ident(Database)+` WITH (FORCE)`); err != nil {
			t.Errorf("drop database: %v", err)
		}
	})

	ensureRoles(ctx, t, admin)
	// ot_loader is shared with TestLoaderGrantsPG, which connects with testPassword: hold lock 54 while Setup
	// changes its password and put testPassword back before the lock goes.
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, r := range addedRoles {
			_, _ = admin.Exec(ctx, "ALTER ROLE "+ident(r)+" PASSWORD "+literal(scramVerifier(testPassword, newSalt())))
		}
		_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
	}()

	home := t.TempDir()
	seed := seedEnv(t, dsn)
	envPath := filepath.Join(home, ".env")
	if err := os.WriteFile(envPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := store.SetupConfig{AdminDSN: dsn, Home: home}
	var logs bytes.Buffer
	if err := run(ctx, cfg, extensions, slog.New(slog.NewTextHandler(&logs, nil))); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	first, _ := os.ReadFile(envPath)
	loaderPW := checkAddedLines(t, seed, string(first), logs.String())
	if err := Setup(ctx, cfg); err != nil {
		t.Fatalf("second Setup: %v", err)
	}
	if second, _ := os.ReadFile(envPath); string(second) != string(first) {
		t.Error("the second run changed .env")
	}

	env, err := readEnv(envPath)
	if err != nil || env["FRED_API_KEY"] != "kept" {
		t.Fatalf("env = %v, %v", env, err)
	}
	if again := passwordOf(env[store.DSNEnvLoader], roleLoader); again != loaderPW {
		t.Error("the second run changed the loader password")
	}
	for _, key := range []string{store.DSNEnvRunner, store.DSNEnvSeriesFred, store.DSNEnvPages, store.DSNEnvLoader, store.DSNEnvOptionEod, store.DSNEnvQuoteSnapshot} {
		c, err := pgx.Connect(ctx, env[key])
		if err != nil {
			t.Fatalf("connect with %s: %v", key, err)
		}
		var ext bool
		if err := c.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb')`).Scan(&ext); err != nil || !ext {
			t.Errorf("%s: timescaledb extension: %v, %v", key, ext, err)
		}
		_ = c.Close(ctx)
	}
	var owner string
	if err := admin.QueryRow(ctx, `SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = $1`, Database).Scan(&owner); err != nil || owner != roleOwner {
		t.Errorf("database owner = %q, %v", owner, err)
	}
}

// ensureRoles creates the week-1 roles with the shared test password when missing; the roles are shared with
// the other pg tests, which hold advisory lock 54 while they do the same.
func ensureRoles(ctx context.Context, t *testing.T, admin *pgx.Conn) {
	t.Helper()
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range roles {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r.name).Scan(&have); err != nil {
			t.Fatal(err)
		}
		if !have {
			if _, err := admin.Exec(ctx, "CREATE ROLE "+ident(r.name)+" LOGIN PASSWORD "+literal(testPassword)); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
}

// addedRoles are the roles whose lines the seed lacks: Setup adds them, with the password it generates, and the
// test puts testPassword back under lock 54 before it ends.
var addedRoles = []string{roleLoader, roleOptionEod, roleQuoteSnapshot}

func isAddedRole(name string) bool {
	for _, r := range addedRoles {
		if r == name {
			return true
		}
	}
	return false
}

// seedEnv returns a .env whose DSNs carry testPassword, so that Setup keeps the passwords of the roles.
func seedEnv(t *testing.T, dsn string) string {
	t.Helper()
	var seed strings.Builder
	seed.WriteString("FRED_API_KEY=kept\n")
	for _, r := range roles {
		if r.envKey == "" || isAddedRole(r.name) {
			continue
		}
		d, err := roleDSN(dsn, r.name, testPassword)
		if err != nil {
			t.Fatal(err)
		}
		seed.WriteString(r.envKey + "=" + d + "\n")
	}
	return seed.String()
}

// TestSetupLogsNoSecret: run with a failing admin DSN holds no password, verifier or DSN in its error.
func TestSetupLogsNoSecret(t *testing.T) {
	t.Parallel()
	const secret = "sentinel-admin-pw"
	var logs bytes.Buffer
	dsn := "postgres://admin:" + secret + "@127.0.0.1:1/postgres?connect_timeout=1"
	err := run(context.Background(), store.SetupConfig{AdminDSN: dsn, Home: t.TempDir()}, extensions, slog.New(slog.NewTextHandler(&logs, nil)))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, s := range []string{secret, "SCRAM-SHA-256", dsn} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("the error holds %q: %v", s, err)
		}
		if strings.Contains(logs.String(), s) {
			t.Errorf("the log holds %q", s)
		}
	}
}

// checkAddedLines asserts that Setup added exactly the lines of addedRoles to the seed, each with the user of its
// role and the database optdesk, and kept every secret out of the log; it returns the loader password.
func checkAddedLines(t *testing.T, seed, first, logs string) string {
	t.Helper()
	if !strings.HasPrefix(first, seed) {
		t.Errorf("the seeded lines changed:\n%s", first)
	}
	added := strings.TrimPrefix(first, seed)
	if n := strings.Count(added, "\n"); n != len(addedRoles) {
		t.Fatalf("Setup added %d lines (%q), want %d", n, added, len(addedRoles))
	}
	var loaderPW string
	for _, r := range roles {
		if !isAddedRole(r.name) {
			continue
		}
		var line string
		for _, l := range strings.Split(added, "\n") {
			if strings.HasPrefix(l, r.envKey+"=") {
				line = l
			}
		}
		if line == "" {
			t.Fatalf("Setup added no %s line: %q", r.envKey, added)
		}
		u, err := url.Parse(strings.TrimPrefix(line, r.envKey+"="))
		if err != nil || u.User.Username() != r.name || u.Path != "/"+Database {
			t.Fatalf("%s DSN user/database wrong: %v", r.envKey, err)
		}
		pw, _ := u.User.Password()
		if pw == "" || strings.Contains(logs, pw) || strings.Contains(logs, line) {
			t.Errorf("the log holds a secret of %s: %s", r.name, logs)
		}
		if r.name == roleLoader {
			loaderPW = pw
		}
	}
	if strings.Contains(logs, "SCRAM-SHA-256") {
		t.Errorf("the log holds a verifier: %s", logs)
	}
	return loaderPW
}
