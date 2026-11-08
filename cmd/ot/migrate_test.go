package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
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
