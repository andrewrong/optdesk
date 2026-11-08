package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/pressly/goose/v3"
)

type upCall struct {
	dsn   string
	fsys  fs.FS
	calls int
}

func migrateApp(t *testing.T, env map[string]string, profileName string, up func(*upCall) error) (*app, *bytes.Buffer, *upCall) {
	t.Helper()
	var out, errb bytes.Buffer
	a, err := newApp(func(k string) string { return env[k] }, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	a.profile.Name = profileName
	rec := &upCall{}
	a.deps = deps{migrateUp: func(_ context.Context, dsn string, fsys fs.FS) error {
		rec.dsn, rec.fsys = dsn, fsys
		rec.calls++
		return up(rec)
	}}
	return a, &errb, rec
}

func okUp(*upCall) error { return nil }

func TestMigrateCallsUp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0001_x.sql"), []byte("-- x"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, errb, rec := migrateApp(t, map[string]string{store.AdminDSNEnv: "dsn-x"}, "real", okUp)
	if err := runMigrate(a, []string{"--dir", dir}); err != nil {
		t.Fatal(err)
	}
	if rec.calls != 1 || rec.dsn != "dsn-x" {
		t.Fatalf("calls %d dsn %q", rec.calls, rec.dsn)
	}
	ents, err := fs.ReadDir(rec.fsys, ".")
	if err != nil || len(ents) != 1 || ents[0].Name() != "0001_x.sql" {
		t.Fatalf("entries %v err %v", ents, err)
	}
	if !strings.Contains(errb.String(), dir) || strings.Contains(errb.String(), "dsn-x") {
		t.Fatalf("log %q", errb.String())
	}
}

// sqlDir returns a temporary directory holding one .sql file.
func sqlDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0001_x.sql"), []byte("-- x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestMigrateDefaultDir: the default --dir is "migrations" relative to the working directory
// (cmd/ot has none), so the check names that path and nothing is called.
func TestMigrateDefaultDir(t *testing.T) {
	t.Parallel()
	a, _, rec := migrateApp(t, map[string]string{store.AdminDSNEnv: "d"}, "real", okUp)
	err := runMigrate(a, nil)
	if err == nil || !strings.Contains(err.Error(), `--dir "migrations"`) || rec.calls != 0 {
		t.Fatalf("err %v calls %d", err, rec.calls)
	}
}

func checkDirRefused(t *testing.T, dir string) {
	t.Helper()
	const dsn = "postgres://u:secret-pw@h/db"
	a, _, rec := migrateApp(t, map[string]string{store.AdminDSNEnv: dsn}, "real", okUp)
	err := runMigrate(a, []string{"--dir", dir})
	if err == nil || !strings.Contains(err.Error(), "--dir "+strconv.Quote(dir)+" is not a directory with .sql files; pass --dir <repository>/migrations") {
		t.Fatalf("err %v", err)
	}
	if rec.calls != 0 {
		t.Fatalf("migrateUp called %d times", rec.calls)
	}
	if strings.Contains(err.Error(), dsn) || strings.Contains(err.Error(), "secret-pw") {
		t.Fatalf("DSN in %v", err)
	}
}

func TestMigrateDirMissing(t *testing.T) {
	t.Parallel()
	checkDirRefused(t, filepath.Join(t.TempDir(), "absent"))
}

func TestMigrateDirEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	checkDirRefused(t, dir)
}

// A subdirectory named like a migration is not a migration file.
func TestMigrateDirOnlySubdirNamedSQL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "x.sql"), 0o700); err != nil {
		t.Fatal(err)
	}
	checkDirRefused(t, dir)
}

func TestMigrateDirWithSQL(t *testing.T) {
	t.Parallel()
	a, _, rec := migrateApp(t, map[string]string{store.AdminDSNEnv: "d"}, "real", okUp)
	if err := runMigrate(a, []string{"--dir", sqlDir(t)}); err != nil {
		t.Fatal(err)
	}
	if rec.calls != 1 {
		t.Fatalf("calls %d", rec.calls)
	}
}

func TestMigrateDemoRefused(t *testing.T) {
	t.Parallel()
	a, _, rec := migrateApp(t, map[string]string{store.AdminDSNEnv: "d"}, "demo", okUp)
	if err := runMigrate(a, nil); err == nil || rec.calls != 0 {
		t.Fatalf("err %v calls %d", err, rec.calls)
	}
}

func TestMigrateUnsetDSN(t *testing.T) {
	t.Parallel()
	a, _, rec := migrateApp(t, map[string]string{}, "real", okUp)
	err := runMigrate(a, nil)
	if err == nil || !strings.Contains(err.Error(), store.AdminDSNEnv) || rec.calls != 0 {
		t.Fatalf("err %v calls %d", err, rec.calls)
	}
}

func TestMigrateFailureStopsLoad(t *testing.T) {
	t.Parallel()
	a, errb, _ := migrateApp(t, map[string]string{store.AdminDSNEnv: "d"}, "real",
		func(*upCall) error { return errors.New("boom-text") })
	err := runMigrate(a, []string{"--dir", sqlDir(t)})
	if err == nil || !strings.Contains(err.Error(), store.AdminDSNEnv) || strings.Contains(err.Error(), "boom-text") {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(errb.String(), "boom-text") || strings.Contains(errb.String(), "status=ok") {
		t.Fatalf("log %q", errb.String())
	}

	// the real dispatcher with the real migrateUp against a refused loopback port (no server is
	// contacted): the one error line is the line main.run writes.
	{
		const dsn = "postgres://sentinel-user:sentinel-pw@127.0.0.1:0/optdesk"
		env := map[string]string{"OT_HOME": t.TempDir(), "OT_ZONE": "UTC", store.AdminDSNEnv: dsn}
		var out, errb bytes.Buffer
		code := run([]string{"migrate", "--dir", sqlDir(t)}, func(k string) string { return env[k] }, &out, &errb)
		if code != 1 {
			t.Fatalf("exit code %d, log %q", code, errb.String())
		}
		if n := strings.Count(errb.String(), "level=ERROR"); n != 1 {
			t.Fatalf("%d error lines in %q", n, errb.String())
		}
		for _, s := range []string{dsn, "sentinel-pw", "sentinel-user"} {
			if strings.Contains(errb.String(), s) {
				t.Fatalf("secret %q in log %q", s, errb.String())
			}
		}
	}
}

func migrateSentinel(t *testing.T, dsn string) {
	t.Helper()
	a, errb, _ := migrateApp(t, map[string]string{store.AdminDSNEnv: dsn}, "real",
		func(*upCall) error { return errors.New("dial " + dsn + " password sentinel-pw") })
	err := runMigrate(a, []string{"--dir", sqlDir(t)})
	// run reports a command error through the log, as main does
	a.log.Error("command failed", "component", "ot", "err", err.Error())
	for _, s := range []string{dsn, "sentinel-pw"} {
		if strings.Contains(err.Error(), s) || strings.Contains(errb.String(), s) {
			t.Fatalf("secret %q leaked: %v / %q", s, err, errb.String())
		}
	}
}

func TestMigrateMalformedDSN(t *testing.T) {
	t.Parallel()
	migrateSentinel(t, "postgres://sentinel-user:sentinel-pw@%zz/db")
}

func TestMigrateWrongPasswordDSN(t *testing.T) {
	t.Parallel()
	migrateSentinel(t, "postgres://sentinel-user:sentinel-pw@127.0.0.1:0/db")
}

// migrateFail runs ot migrate with a migrateUp that returns up, and returns the log and the
// returned error after the error was logged the way main does.
func migrateFail(t *testing.T, up error) (string, error) {
	t.Helper()
	a, errb, _ := migrateApp(t, map[string]string{store.AdminDSNEnv: "postgres://u:sentinel-pw@h/db"}, "real",
		func(*upCall) error { return up })
	err := runMigrate(a, []string{"--dir", sqlDir(t)})
	if err == nil {
		t.Fatal("want an error")
	}
	a.log.Error("command failed", "component", "ot", "err", err.Error())
	return errb.String(), err
}

// fakePgError stands for a *pgconn.PgError: an error with a SQLSTATE and a fixed message.
type fakePgError struct{ code, msg string }

func (e *fakePgError) Error() string    { return e.msg }
func (e *fakePgError) SQLState() string { return e.code }

func partialWrapping(version int64, pg *fakePgError) error {
	return &goose.PartialError{
		Failed: &goose.MigrationResult{Source: &goose.Source{Version: version}},
		Err:    fmt.Errorf("WRAPPEDMARKER: %w", pg),
	}
}

func TestMigrateErrorStage(t *testing.T) {
	t.Parallel()
	const text = "setup: migrate: connect: the server refused or could not be reached"
	log, err := migrateFail(t, errors.New(text))
	if !strings.Contains(err.Error(), text) || !strings.Contains(log, text) {
		t.Fatalf("err %v log %q", err, log)
	}
}

func TestMigrateErrorSQLState(t *testing.T) {
	t.Parallel()
	pg := &fakePgError{code: "23505", msg: "duplicate key value"}
	log, err := migrateFail(t, partialWrapping(13, pg))
	for _, s := range []string{"13", "23505", "duplicate key value"} {
		if !strings.Contains(err.Error(), s) || !strings.Contains(log, s) {
			t.Fatalf("%q missing: err %v log %q", s, err, log)
		}
	}
	for _, s := range []string{"WRAPPEDMARKER", "sentinel-pw", "ot setup pg"} {
		if strings.Contains(err.Error(), s) || strings.Contains(log, s) {
			t.Fatalf("%q present: err %v log %q", s, err, log)
		}
	}
}

func TestMigrateErrorRoleHint(t *testing.T) {
	t.Parallel()
	pg := &fakePgError{code: "42704", msg: `role "ot_w_readings" does not exist`}
	log, err := migrateFail(t, partialWrapping(13, pg))
	// the log quotes the message, so the log is checked for its unquoted part
	for _, s := range []string{"13", "42704", "ot_w_readings", "does not exist", "ot setup pg"} {
		if !strings.Contains(err.Error(), s) || !strings.Contains(log, s) {
			t.Fatalf("%q missing: err %v log %q", s, err, log)
		}
	}
	if !strings.Contains(err.Error(), pg.msg) {
		t.Fatalf("message missing: %v", err)
	}
	if strings.Contains(err.Error(), "WRAPPEDMARKER") || strings.Contains(log, "WRAPPEDMARKER") {
		t.Fatalf("wrapped text leaked: %v", err)
	}
	// 42704 without a role in the message gets no hint
	_, err = migrateFail(t, partialWrapping(5, &fakePgError{code: "42704", msg: `type "x" does not exist`}))
	if strings.Contains(err.Error(), "ot setup pg") {
		t.Fatalf("hint on a non-role message: %v", err)
	}
}

func TestMigrateErrorUnknownWithheld(t *testing.T) {
	t.Parallel()
	const dsn = "postgres://sentinel-user:sentinel-pw@127.0.0.1:0/db"
	log, err := migrateFail(t, errors.New("dial "+dsn+" password sentinel-pw"))
	if !strings.Contains(err.Error(), "migrate failed; the error text is withheld because it may hold "+store.AdminDSNEnv) {
		t.Fatalf("err %v", err)
	}
	for _, s := range []string{dsn, "sentinel-pw", "sentinel-user"} {
		if strings.Contains(err.Error(), s) || strings.Contains(log, s) {
			t.Fatalf("%q leaked: %v / %q", s, err, log)
		}
	}
}
