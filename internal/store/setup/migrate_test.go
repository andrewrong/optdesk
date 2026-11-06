package setup

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"
)

const sentinel = "S3NTINEL-pw"

// fakeConn records the statements it is given and rejects those that contain reject.
type fakeConn struct {
	stmts  []string
	reject string
}

func (f *fakeConn) Exec(_ context.Context, q string, _ ...any) (pgconn.CommandTag, error) {
	f.stmts = append(f.stmts, q)
	if f.reject != "" && strings.Contains(q, f.reject) {
		return pgconn.CommandTag{}, errors.New("boom " + sentinel)
	}
	return pgconn.CommandTag{}, nil
}

// driveMigrate drives migrateWith with an open that runs the after-connect step on conn and an up that counts its calls.
func driveMigrate(t *testing.T, conn *fakeConn) (upCalls int, err error) {
	t.Helper()
	open := func(ctx context.Context, after func(context.Context, execer) error) (*sql.DB, error) {
		if err := after(ctx, conn); err != nil {
			return nil, err
		}
		return nil, nil
	}
	up := func(context.Context, *sql.DB, fs.FS) error { upCalls++; return nil }
	err = migrateWith(context.Background(), open, up, fstest.MapFS{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return upCalls, err
}

func TestSetRoleFailureRunsNothing(t *testing.T) {
	t.Parallel()
	conn := &fakeConn{reject: "SET ROLE"}
	calls, err := driveMigrate(t, conn)
	if err == nil || !strings.Contains(err.Error(), "set role") {
		t.Fatalf("want error of stage set role, got %v", err)
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("error carries the driver text: %v", err)
	}
	if len(conn.stmts) != 1 || conn.stmts[0] != "SET ROLE ot_owner" {
		t.Fatalf("statements = %q, want only SET ROLE", conn.stmts)
	}
	if calls != 0 {
		t.Fatalf("migrations ran %d times", calls)
	}
}

func TestSetRoleIsFirst(t *testing.T) {
	t.Parallel()
	conn := &fakeConn{}
	calls, err := driveMigrate(t, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.stmts) < 2 || conn.stmts[0] != "SET ROLE ot_owner" || conn.stmts[1] != "SET search_path TO meta" {
		t.Fatalf("statements = %q", conn.stmts)
	}
	if calls != 1 {
		t.Fatalf("up called %d times", calls)
	}
}

func TestSchemaAfterRole(t *testing.T) {
	t.Parallel()
	conn := &fakeConn{}
	if _, err := driveMigrate(t, conn); err != nil {
		t.Fatal(err)
	}
	role, schema := -1, -1
	for i, s := range conn.stmts {
		if s == "SET ROLE ot_owner" {
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
		t.Fatalf("role at %d, schema at %d in %q", role, schema, conn.stmts)
	}
}

func TestSchemaFailureIsOfStageSchema(t *testing.T) {
	t.Parallel()
	calls, err := driveMigrate(t, &fakeConn{reject: "CREATE SCHEMA"})
	if err == nil || !strings.Contains(err.Error(), "schema") || strings.Contains(err.Error(), sentinel) {
		t.Fatalf("got %v", err)
	}
	if calls != 0 {
		t.Fatal("migrations ran")
	}
}

func holdsNoSecret(t *testing.T, dsn string, err error, logs *bytes.Buffer) {
	t.Helper()
	if err == nil {
		t.Fatal("want an error")
	}
	for _, s := range []string{sentinel, dsn} {
		if strings.Contains(err.Error(), s) || strings.Contains(logs.String(), s) {
			t.Errorf("%q leaks into error %q or log %q", s, err, logs)
		}
	}
}

func TestMigrateUpMalformedDSNHoldsNoSecret(t *testing.T) {
	t.Parallel()
	for _, dsn := range []string{
		"postgres://ot:" + sentinel + "@127.0.0.1:notaport/x",
		"not a url " + sentinel,
		"mysql://ot:" + sentinel + "@h/x",
	} {
		var logs bytes.Buffer
		err := migrateUp(context.Background(), dsn, fstest.MapFS{}, slog.New(slog.NewTextHandler(&logs, nil)))
		holdsNoSecret(t, dsn, err, &logs)
		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("want stage parse, got %v", err)
		}
	}
}

func TestMigrateUpUnreachableHoldsNoSecret(t *testing.T) {
	t.Parallel()
	// pgx rejects port 0 at parse time, so take a loopback port that was free a moment ago: nothing listens there.
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	dsn := "postgres://nobody:" + sentinel + "@" + addr + "/postgres?sslmode=disable"
	var logs bytes.Buffer
	err = migrateUp(context.Background(), dsn, fstest.MapFS{}, slog.New(slog.NewTextHandler(&logs, nil)))
	holdsNoSecret(t, dsn, err, &logs)
	if !strings.Contains(err.Error(), "connect") {
		t.Errorf("want stage connect, got %v", err)
	}
}

func TestOwnerDSNMalformedHoldsNoSecret(t *testing.T) {
	t.Parallel()
	dsn := "postgres://ot:" + sentinel + "@127.0.0.1:notaport/x"
	got, err := OwnerDSN(dsn)
	holdsNoSecret(t, dsn, err, &bytes.Buffer{})
	if got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestOwnerDSNSetsRole(t *testing.T) {
	t.Parallel()
	got, err := OwnerDSN("postgres://adm:pw"+"@db.example:5433/other")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	pw, _ := u.User.Password()
	if u.Scheme != "postgres" || u.Host != "db.example:5433" || u.Path != "/optdesk" || u.User.Username() != "adm" || pw != "pw" {
		t.Fatalf("got %s", got)
	}
	if o := u.Query().Get("options"); o != "-c role=ot_owner" {
		t.Fatalf("options = %q", o)
	}
}

func TestOwnerDSNKeepsSSLMode(t *testing.T) {
	t.Parallel()
	got, err := OwnerDSN("postgresql://adm:pw"+"@h:5432/x?sslmode=require")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("sslmode") != "require" || u.Query().Get("options") == "" {
		t.Fatalf("got %s", got)
	}
}
