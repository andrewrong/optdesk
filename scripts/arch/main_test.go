package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

func modulesText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "design", "modules.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runArch(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String() + errb.String()
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestArchCheckCommittedFiles(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	code, out := runArch(t, "--check", "--modules", filepath.Join(root, "docs/design/modules.md"), "--root", root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
}

// fixtureModules is a small modules text of its own: three modules and the implementation list that generate reads.
func fixtureModules(gammaDepends string) string {
	return "# Fixture\n\n## Modules\n\n" +
		"| module | paths | entry | tests | depends |\n" +
		"| --- | --- | --- | --- | --- |\n" +
		"| alpha | internal/alpha/** | api.go | internal/alpha/api_test.go | |\n" +
		"| beta | internal/beta/** | api.go | internal/beta/api_test.go | alpha |\n" +
		"| gamma | internal/gamma/** | api.go | internal/gamma/api_test.go | " + gammaDepends + " |\n\n" +
		"The composition root imports one constructor each: `internal/alpha/impl`. Any other import is refused.\n"
}

func TestArchCheckFailsOnNewEdge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	modules := filepath.Join(dir, "modules.md")
	write(t, modules, fixtureModules("alpha"))
	if code, out := runArch(t, "--modules", modules, "--root", dir); code != 0 {
		t.Fatalf("generate: exit %d: %s", code, out)
	}
	if code, out := runArch(t, "--check", "--modules", modules, "--root", dir); code != 0 {
		t.Fatalf("fresh files: exit %d: %s", code, out)
	}
	write(t, modules, fixtureModules("alpha beta"))
	code, out := runArch(t, "--check", "--modules", modules, "--root", dir)
	if code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, out)
	}
}

// otImports returns the sub-package imports (those with a slash) of component ot in the generated arch.toml.
func otImports(t *testing.T, text string) ([]string, string) {
	t.Helper()
	toml, _, err := generate(text, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)\[component\."ot"\]\nin = "cmd/ot"\nmay_import = \[([^\]]*)\]`).FindSubmatch(toml)
	if m == nil {
		t.Fatalf("no component ot in:\n%s", toml)
	}
	var got []string
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(m[1], -1) {
		if strings.Contains(string(q[1]), "/") {
			got = append(got, string(q[1]))
		}
	}
	return got, string(m[1])
}

func TestArchCheckOTListsImplPackages(t *testing.T) {
	t.Parallel()
	text := modulesText(t)
	// The list as the text states it: backticked internal/a/b paths after the anchor.
	_, after, _ := strings.Cut(text, "one constructor each")
	list, _, _ := strings.Cut(after, "Any other import")
	var want []string
	for _, m := range regexp.MustCompile("`internal/([a-z0-9_]+/[a-z0-9_]+)`").FindAllStringSubmatch(list, -1) {
		want = append(want, m[1])
	}
	got, all := otImports(t, text)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ot imports %v, text lists %v", got, want)
	}
	for _, root := range []string{"contracts", "clock", "sources", "store", "runner", "jobs", "pages", "profile"} {
		if !strings.Contains(all, `"`+root+`"`) {
			t.Errorf("ot lacks root component %s", root)
		}
	}
}

func TestArchOTListIsReadFromTheText(t *testing.T) {
	t.Parallel()
	text := modulesText(t)
	const anchor = "one constructor each: "
	longer := strings.Replace(text, anchor, anchor+"`internal/a/b`, ", 1)
	if longer == text {
		t.Fatal("the anchor was not found")
	}
	got, _ := otImports(t, longer)
	found := false
	for _, g := range got {
		found = found || g == "a/b"
	}
	if !found {
		t.Fatalf("ot imports %v, want a/b among them", got)
	}
}

func TestArchRunFailsWithoutImplAnchor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	modules := filepath.Join(dir, "modules.md")
	text := modulesText(t)
	cut := strings.Replace(text, "one constructor each", "", 1)
	if cut == text {
		t.Fatal("the anchor was not found")
	}
	write(t, modules, cut)
	if code, out := runArch(t, "--modules", modules, "--root", dir); code != 2 {
		t.Fatalf("exit %d, want 2: %s", code, out)
	}
}

func TestArchCheckProfileHasNoImports(t *testing.T) {
	t.Parallel()
	toml, yml, err := generate(modulesText(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), "[component.\"profile\"]\nin = \"internal/profile\"\nmay_import = []\n") {
		t.Fatalf("profile is not generated with an empty import list:\n%s", toml)
	}
	if strings.Contains(string(yml), "\n  profile:\n    mayDependOn") {
		t.Fatal("profile has a mayDependOn entry")
	}
}

// scratch writes a module tree of interface packages plus the given files.
func scratch(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/s\n\ngo 1.24\n")
	pkgs := map[string]string{
		"internal/contracts": "contracts", "internal/clock": "clock", "internal/sources": "sources",
		"internal/store": "store", "internal/runner": "runner", "internal/jobs": "jobs",
		"internal/pages": "pages", "internal/profile": "profile",
		"internal/store/migrate": "migrate", "internal/store/sessionday": "sessionday",
		"cmd/ot": "main",
	}
	for p, name := range pkgs {
		src := "package " + name + "\n"
		if name == "main" {
			src += "\nfunc main() {}\n"
		}
		write(t, filepath.Join(dir, p, "x.go"), src)
	}
	for p, src := range files {
		write(t, filepath.Join(dir, p), src)
	}
	_, yml, err := generate(modulesText(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, ymlFile), string(yml))
	return dir
}

func imports(pkg, path string) string {
	return "package " + pkg + "\n\nimport _ \"example.com/s/" + path + "\"\n"
}

// lint runs the pinned go-arch-lint on a scratch tree and reports whether it passed.
func lint(t *testing.T, dir string) (bool, string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", "tool", "go-arch-lint", "check", "--project-path", dir, "--output-color=false")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}

func TestArchCheckScratchTree(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		file string
		src  string
		pass bool
	}{
		{"ot imports a store sub-package outside the list", "cmd/ot/main2.go", imports("main", "internal/store/migrate"), false},
		{"ot imports a listed implementation package", "cmd/ot/main2.go", imports("main", "internal/store/sessionday"), true},
		{"a jobs sub-package imports a store sub-package", "internal/jobs/fred/j.go", imports("fred", "internal/store/sessionday"), false},
		{"a jobs sub-package imports the store root", "internal/jobs/fred/j.go", imports("fred", "internal/store"), true},
		{"profile imports another module", "internal/profile/p.go", imports("profile", "internal/store"), false},
		{"clock imports a module that its row does not list", "internal/clock/c.go", imports("clock", "internal/runner"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := scratch(t, map[string]string{tc.file: tc.src})
			ok, out := lint(t, dir)
			if ok != tc.pass {
				t.Fatalf("pass = %v, want %v:\n%s", ok, tc.pass, out)
			}
		})
	}
}

func TestArchDeriveContractsGenpb(t *testing.T) {
	t.Parallel()
	dir := scratch(t, map[string]string{"internal/contracts/genpb/session.pb.go": "package genpb\n"})
	toml, yml, err := generate(modulesText(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), "[component.\"contracts/genpb\"]\nin = \"internal/contracts/genpb\"\nmay_import = []\n") {
		t.Fatalf("contracts/genpb is not derived with an empty import list:\n%s", toml)
	}
	if !strings.Contains(string(yml), "  contracts/genpb:\n    in: internal/contracts/genpb\n") {
		t.Fatalf("no contracts/genpb component in:\n%s", yml)
	}
	_, depsPart, _ := strings.Cut(string(yml), "\ndeps:\n")
	if strings.Contains(depsPart, "  contracts/genpb:\n") {
		t.Fatal("contracts/genpb has a mayDependOn entry")
	}
	if ok, out := lint(t, dir); !ok {
		t.Fatalf("a package under internal/contracts/genpb fails the lint:\n%s", out)
	}
}

func TestArchDeriveContractsGen(t *testing.T) {
	t.Parallel()
	dir := scratch(t, map[string]string{"internal/contracts/gen/g.go": "package gen\n"})
	toml, _, err := generate(modulesText(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), "[component.\"contracts/gen\"]\nin = \"internal/contracts/gen\"\nmay_import = []\n") {
		t.Fatalf("contracts/gen is not derived with an empty import list:\n%s", toml)
	}
	_, rest, _ := strings.Cut(string(toml), "[component.\"contracts\"]\n")
	block, _, _ := strings.Cut(rest, "\n[")
	if !strings.Contains(block, "may_import = [\"contracts/gen\"]") {
		t.Fatalf("contracts does not import contracts/gen:\n%s", block)
	}
}
