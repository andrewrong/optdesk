package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tree makes a temporary git repository holding the files, all tracked.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q", "--template="}, {"add", "-A"}} {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func check(t *testing.T, files map[string]string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run([]string{"--root", tree(t, files)}, &out, &errb)
	return code, errb.String() + out.String()
}

func mustFind(t *testing.T, files map[string]string, needle string) {
	t.Helper()
	code, out := check(t, files)
	if code != 1 || !strings.Contains(out, needle) {
		t.Fatalf("exit %d output %q, want exit 1 containing %q", code, out, needle)
	}
}

func mustPass(t *testing.T, files map[string]string) {
	t.Helper()
	if code, out := check(t, files); code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
}

func TestHygieneAccountID(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"U" + "1234567", "DU" + "123456", "D" + "12345678"} {
		mustFind(t, map[string]string{"a.txt": "acct " + id + "\n"}, "a.txt:1: account id")
	}
	mustPass(t, map[string]string{"a.txt": "U" + "12345 and X1234567 and U" + "123456789\n"})
}

func TestHygieneUsersPath(t *testing.T) {
	t.Parallel()
	mustFind(t, map[string]string{"a.go": "x\n// " + "/Us" + "ers/someone/x\n"}, "a.go:2: /Users/ path")
}

func TestHygieneVolumesPath(t *testing.T) {
	t.Parallel()
	mustFind(t, map[string]string{"a.sh": "cd " + "/Vol" + "umes/disk\n"}, "a.sh:1: /Volumes/ path")
}

func TestHygieneEmail(t *testing.T) {
	t.Parallel()
	mustFind(t, map[string]string{"a.txt": "mail " + "someone" + "@" + "example.org\n"}, "a.txt:1: e-mail")
	mustPass(t, map[string]string{"a.txt": "noreply" + "@" + "anthropic.com 1+x" + "@" + "users.noreply.github.com golang.org/x/text" + "@" + "v0.14.0\n"})
}

func TestHygieneManifestRule(t *testing.T) {
	t.Parallel()
	manifest := `{"marker":"synthetic-v1","files":["a.json",{"path":"sub/b.json"}]}`
	mustPass(t, map[string]string{
		"x/fixtures/MANIFEST.synthetic-v1.json": manifest, "x/fixtures/a.json": "{}\n", "x/fixtures/sub/b.json": "{}\n",
	})
	mustFind(t, map[string]string{
		"x/fixtures/MANIFEST.synthetic-v1.json": manifest, "x/fixtures/a.json": "{}\n", "x/fixtures/c.json": "{}\n",
	}, "x/fixtures/c.json: fixture not listed")
	mustFind(t, map[string]string{"y/fixtures/a.json": "{}\n"}, "y/fixtures/a.json: fixture not listed")
}

func TestHygieneExemptSkipped(t *testing.T) {
	t.Parallel()
	id := "U" + "1234567"
	mustPass(t, map[string]string{
		"docs/design/a.md":                    id + "\n",
		"scripts/hygiene/x_test.go":           id + "\n",
		"go.sum":                              id + "\n",
		"pyside/uv.lock":                      id + "\n",
		"web/pnpm-lock.yaml":                  id + "\n",
		"fixtures/MANIFEST.synthetic-v1.json": `{"files":[]}` + id,
	})
}
