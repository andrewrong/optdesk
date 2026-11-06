//go:build pg

package setup

import (
	"context"
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

	home := t.TempDir()
	seed := seedEnv(t, dsn)
	envPath := filepath.Join(home, ".env")
	if err := os.WriteFile(envPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := store.SetupConfig{AdminDSN: dsn, Home: home}
	if err := Setup(ctx, cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	first, _ := os.ReadFile(envPath)
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
	for _, key := range []string{store.DSNEnvRunner, store.DSNEnvSeriesFred, store.DSNEnvPages} {
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

// seedEnv returns a .env whose DSNs carry testPassword, so that Setup keeps the passwords of the roles.
func seedEnv(t *testing.T, dsn string) string {
	t.Helper()
	var seed strings.Builder
	seed.WriteString("FRED_API_KEY=kept\n")
	for _, r := range roles {
		if r.envKey == "" {
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
