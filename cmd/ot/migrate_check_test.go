package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeIn(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("-- x "+name), 0o600); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", msg)
	return gitIn(t, dir, "rev-parse", "HEAD")
}

func newRepo(t *testing.T) (dir, base string) {
	t.Helper()
	dir = t.TempDir()
	gitIn(t, dir, "init", "-q")
	writeIn(t, dir, "internal/store/setup/a.go")
	writeIn(t, dir, "migrations/0001_a.sql")
	return dir, commitAll(t, dir, "base")
}

func buildInfo(rev, modified string) *debug.BuildInfo {
	bi := &debug.BuildInfo{}
	if rev != "" {
		bi.Settings = append(bi.Settings, debug.BuildSetting{Key: "vcs.revision", Value: rev})
	}
	if modified != "" {
		bi.Settings = append(bi.Settings, debug.BuildSetting{Key: "vcs.modified", Value: modified})
	}
	return bi
}

func migCheckApp(t *testing.T, applied []int64, dsn string, mErr error) (*app, *bytes.Buffer, *[]int64) {
	t.Helper()
	var out, errb bytes.Buffer
	a, err := newApp(func(k string) string {
		switch k {
		case "OT_HOME":
			return "/home-x"
		case "OT_ZONE":
			return "UTC"
		}
		return ""
	}, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	var want []int64
	a.deps = deps{
		readEnv: func(string) (map[string]string, error) {
			if dsn == "" {
				return map[string]string{}, nil
			}
			return map[string]string{store.DSNEnvRunner: dsn}, nil
		},
		migrateMissing: func(_ context.Context, _ string, w []int64) (missing, extra []int64, err error) {
			want = w
			if mErr != nil {
				return nil, nil, mErr
			}
			have := map[int64]bool{}
			for _, v := range applied {
				have[v] = true
			}
			for _, v := range w {
				if !have[v] {
					missing = append(missing, v)
				}
				delete(have, v)
			}
			for _, v := range applied {
				if have[v] {
					extra = append(extra, v)
				}
			}
			return missing, extra, nil
		},
	}
	return a, &out, &want
}

func TestMigrateCheckNoCommit(t *testing.T) {
	t.Parallel()
	a, _, _ := migCheckApp(t, nil, "dsn", nil)
	if err := migrateCheckCmd(a, []string{"--repo", t.TempDir()}); err == nil {
		t.Fatal("exit 0 without a commit")
	}
}

func TestMigrateCheckVersionsFromTree(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	writeIn(t, dir, "migrations/0065_x.sql")
	head := commitAll(t, dir, "two")
	writeIn(t, dir, "migrations/0099_wt.sql") // working tree only
	a, _, want := migCheckApp(t, []int64{1, 65}, "dsn", nil)
	if err := migrateCheck(a, dir, head, buildInfo(base, "")); err != nil {
		t.Fatal(err)
	}
	if len(*want) != 2 || (*want)[0] != 1 || (*want)[1] != 65 {
		t.Fatalf("versions %v", *want)
	}
}

func TestMigrateCheckNoDSNOrConnectError(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	const secret = "postgres://u:s3cret@h/db"
	cases := map[string]struct {
		dsn string
		err error
	}{
		"no dsn":            {"", nil},
		"connect":           {secret, errors.New("setup: check: connect failed")},
		"permission denied": {secret, errors.New("setup: check: permission denied")},
		"absent table":      {secret, errors.New("setup: check: version table absent")},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _, _ := migCheckApp(t, nil, c.dsn, c.err)
			err := migrateCheck(a, dir, base, buildInfo(base, ""))
			if err == nil {
				t.Fatal("exit 0")
			}
			if strings.Contains(err.Error(), "s3cret") {
				t.Fatalf("DSN in %q", err)
			}
		})
	}
}

func TestMigrateCheckZeroFiles(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	gitIn(t, dir, "rm", "-q", "-r", "migrations")
	head := commitAll(t, dir, "none")
	a, _, _ := migCheckApp(t, nil, "dsn", nil)
	if err := migrateCheck(a, dir, head, buildInfo(base, "")); err == nil {
		t.Fatal("exit 0 with no migration file")
	}
}

func TestMigrateCheckOneMissing(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	writeIn(t, dir, "migrations/0066_y.sql")
	head := commitAll(t, dir, "two")
	a, out, _ := migCheckApp(t, []int64{1}, "dsn", nil)
	if err := migrateCheck(a, dir, head, buildInfo(base, "")); err == nil {
		t.Fatal("exit 0 with a missing version")
	}
	if !strings.Contains(out.String(), "missing 66") {
		t.Fatalf("stdout %q", out)
	}
}

func TestMigrateCheckAllApplied(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	a, _, _ := migCheckApp(t, []int64{1}, "dsn", nil)
	if err := migrateCheck(a, dir, base, buildInfo(base, "")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateCheckExtraTableVersions(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	a, out, _ := migCheckApp(t, []int64{1, 70}, "dsn", nil)
	if err := migrateCheck(a, dir, base, buildInfo(base, "")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "extra 70") {
		t.Fatalf("stdout %q", out)
	}
}

func TestMigrateCheckNoWrite(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	a, _, _ := migCheckApp(t, []int64{1}, "dsn", nil)
	// every other constructor is nil: a call of one would panic
	m := a.deps.migrateMissing
	r := a.deps.readEnv
	a.deps = deps{migrateMissing: m, readEnv: r}
	if err := migrateCheck(a, dir, base, buildInfo(base, "")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateCheckSetupChanged(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	writeIn(t, dir, "migrations/0002_b.sql")
	same := commitAll(t, dir, "migration only")
	a, _, _ := migCheckApp(t, []int64{1, 2}, "dsn", nil)
	if err := migrateCheck(a, dir, same, buildInfo(base, "")); err != nil {
		t.Fatalf("no setup change: %v", err)
	}
	writeIn(t, dir, "internal/store/setup/b.go")
	changed := commitAll(t, dir, "setup")
	if err := migrateCheck(a, dir, changed, buildInfo(base, "")); err == nil {
		t.Fatal("exit 0 though setup changed")
	}
}

func TestMigrateCheckBuildInfo(t *testing.T) {
	t.Parallel()
	dir, base := newRepo(t)
	a, _, _ := migCheckApp(t, []int64{1}, "dsn", nil)
	if err := migrateCheck(a, dir, base, buildInfo("", "")); err == nil {
		t.Fatal("exit 0 without revision")
	}
	if err := migrateCheck(a, dir, base, buildInfo(base, "true")); err == nil {
		t.Fatal("exit 0 with vcs.modified")
	}
	if err := migrateCheck(a, dir, base, nil); err == nil {
		t.Fatal("exit 0 without build info")
	}
}

func TestMigrateCheckStderrShort(t *testing.T) {
	t.Parallel()
	err := shortCheckErr(errors.New(strings.Repeat("x", 500)))
	if n := len([]rune(err.Error())); n != checkErrMax {
		t.Fatalf("length %d", n)
	}
}
