package setup

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const sentinelPassword = "sentinel-pw-one"

// recorder keeps the statements of a fake connection in order.
type recorder struct {
	mu         sync.Mutex
	stmts      []string
	rejectRole bool
	upCalls    int
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stmts = append(r.stmts, s)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.stmts...)
}

// fakeExec is the execer the after-connect step sees; it rejects SET ROLE when asked to.
type fakeExec struct{ r *recorder }

func (f fakeExec) Exec(_ context.Context, q string, _ ...any) (pgconn.CommandTag, error) {
	f.r.add(q)
	if f.r.rejectRole && strings.HasPrefix(q, "SET ROLE") {
		return pgconn.CommandTag{}, errors.New("permission denied " + sentinelPassword)
	}
	return pgconn.CommandTag{}, nil
}

type fakeConnector struct {
	r     *recorder
	after afterConnect
}

func (c fakeConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := c.after(ctx, fakeExec{c.r}); err != nil {
		return nil, err
	}
	return fakeSQLConn{c.r}, nil
}
func (c fakeConnector) Driver() driver.Driver { return nil }

type fakeSQLConn struct{ r *recorder }

func (fakeSQLConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (fakeSQLConn) Close() error                        { return nil }
func (fakeSQLConn) Begin() (driver.Tx, error)           { return nil, errors.New("not used") }
func (c fakeSQLConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.r.add(q)
	return driver.RowsAffected(0), nil
}

func (r *recorder) open(_ context.Context, after afterConnect) (*sql.DB, error) {
	db := sql.OpenDB(fakeConnector{r, after})
	db.SetMaxOpenConns(1)
	return db, nil
}

func (r *recorder) up(context.Context, *sql.DB, fs.FS) error {
	r.mu.Lock()
	r.upCalls++
	r.mu.Unlock()
	return nil
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)) }

func TestSetRoleFailureRunsNothing(t *testing.T) {
	t.Parallel()
	r := &recorder{rejectRole: true}
	err := migrateWith(context.Background(), r.open, r.up, fstest.MapFS{}, quietLog())
	if err == nil || !strings.Contains(err.Error(), "set role") {
		t.Fatalf("err = %v, want stage set role", err)
	}
	if got := r.list(); len(got) != 1 || got[0] != "SET ROLE ot_owner" {
		t.Errorf("statements = %q, want only the role switch", got)
	}
	if r.upCalls != 0 {
		t.Error("migrations ran after the role was refused")
	}
	if strings.Contains(err.Error(), sentinelPassword) {
		t.Errorf("error holds the server text: %v", err)
	}
}

func TestSetRoleIsFirst(t *testing.T) {
	t.Parallel()
	r := &recorder{}
	if err := migrateWith(context.Background(), r.open, r.up, fstest.MapFS{}, quietLog()); err != nil {
		t.Fatal(err)
	}
	got := r.list()
	if len(got) < 2 || got[0] != "SET ROLE ot_owner" || got[1] != "SET search_path TO meta" {
		t.Errorf("statements = %q", got)
	}
	if r.upCalls != 1 {
		t.Errorf("up calls = %d, want 1", r.upCalls)
	}
}

func TestSchemaAfterRole(t *testing.T) {
	t.Parallel()
	r := &recorder{}
	if err := migrateWith(context.Background(), r.open, r.up, fstest.MapFS{}, quietLog()); err != nil {
		t.Fatal(err)
	}
	role, schema := -1, -1
	for i, s := range r.list() {
		if s == "SET ROLE ot_owner" && role < 0 {
			role = i
		}
		if s == "CREATE SCHEMA IF NOT EXISTS meta" {
			schema = i
		}
		if strings.Contains(s, "public") {
			t.Errorf("statement mentions public: %q", s)
		}
	}
	if role < 0 || schema < 0 || schema < role {
		t.Errorf("role at %d, schema at %d in %q", role, schema, r.list())
	}
}

func assertNoSecret(t *testing.T, err error, logs *bytes.Buffer, secrets ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("want an error")
	}
	for _, s := range secrets {
		if strings.Contains(err.Error(), s) || strings.Contains(logs.String(), s) {
			t.Errorf("secret %q in error %q or log %q", s, err, logs)
		}
	}
}

func TestMigrateUpMalformedDSNHoldsNoSecret(t *testing.T) {
	t.Parallel()
	dsn := "postgres://admin:" + sentinelPassword + "@h:5432/%zz"
	var logs bytes.Buffer
	err := migrateUp(context.Background(), dsn, fstest.MapFS{}, slog.New(slog.NewTextHandler(&logs, nil)))
	assertNoSecret(t, err, &logs, sentinelPassword, dsn)
}

func TestMigrateUpUnreachableHoldsNoSecret(t *testing.T) {
	t.Parallel()
	dsn := "postgres://nobody:" + sentinelPassword + "@127.0.0.1:0/postgres?sslmode=disable"
	var logs bytes.Buffer
	err := migrateUp(context.Background(), dsn, fstest.MapFS{}, slog.New(slog.NewTextHandler(&logs, nil)))
	assertNoSecret(t, err, &logs, sentinelPassword, dsn, "nobody")
}

func TestOwnerDSNMalformedHoldsNoSecret(t *testing.T) {
	t.Parallel()
	for _, dsn := range []string{
		"postgres://admin:" + sentinelPassword + "@h:5432/%zz",
		"mysql://admin:" + sentinelPassword + "@h:3306/x",
	} {
		out, err := OwnerDSN(dsn)
		assertNoSecret(t, err, &bytes.Buffer{}, sentinelPassword, dsn)
		if out != "" {
			t.Errorf("DSN = %q on error", out)
		}
	}
}

func TestOwnerDSNSetsRole(t *testing.T) {
	t.Parallel()
	got, err := OwnerDSN("postgres://admin:pw@db.example:5433/postgres")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(got)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "db.example" || cfg.Port != 5433 || cfg.User != "admin" || cfg.Password != "pw" || cfg.Database != "optdesk" {
		t.Errorf("config = %+v", cfg.Config)
	}
	if cfg.RuntimeParams["options"] != "-c role=ot_owner" {
		t.Errorf("options = %q", cfg.RuntimeParams["options"])
	}
	if !strings.HasPrefix(got, "postgres://") {
		t.Errorf("DSN scheme: %q", got)
	}
}

func TestOwnerDSNKeepsSSLMode(t *testing.T) {
	t.Parallel()
	got, err := OwnerDSN("postgresql://admin:pw@h:5432/postgres?sslmode=require&application_name=x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "sslmode=require") || strings.Contains(got, "application_name") {
		t.Errorf("DSN = %q", got)
	}
}
