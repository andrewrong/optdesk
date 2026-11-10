package jobs_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var sharedPending = regexp.MustCompile(`shared-pending: (docs/design/[A-Za-z0-9._-]+\.md)`)

// apiTypeProblems returns one line for each exported type declared in internal/jobs/api.go of the module at
// root that fewer than two directories use, other than cmd/ot and the root internal/jobs, as
// <name of the jobs import>.<Type>. A type whose comment carries shared-pending: docs/design/<name>.md naming
// an existing file is not a problem, whatever its number of users. Reads files only.
func apiTypeProblems(root string) ([]string, error) {
	mod, err := modulePath(root)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	apiFile, err := parser.ParseFile(fset, filepath.Join(root, "internal", "jobs", "api.go"), nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	users, pending := declaredTypes(root, apiFile)
	err = filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return skipDir(root, path, e.Name())
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		rel = filepath.ToSlash(rel)
		if rel == "cmd/ot" || rel == "internal/jobs" {
			return nil
		}
		return countUses(path, rel, mod+"/internal/jobs", users)
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for t, dirs := range users {
		if len(dirs) < 2 && !pending[t] {
			list := make([]string, 0, len(dirs))
			for d := range dirs {
				list = append(list, d)
			}
			sort.Strings(list)
			out = append(out, fmt.Sprintf("%s has %d user packages %v; move it to its package or mark it shared-pending: docs/design/<name>.md", t, len(list), list))
		}
	}
	sort.Strings(out)
	return out, nil
}

func skipDir(root, path, name string) error {
	if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "web" || name == "pyside") {
		return filepath.SkipDir
	}
	return nil
}

// declaredTypes returns the exported types of api.go, each with an empty set of user directories, and the
// ones marked shared-pending with a design file that exists.
func declaredTypes(root string, apiFile *ast.File) (users map[string]map[string]bool, pending map[string]bool) {
	users, pending = map[string]map[string]bool{}, map[string]bool{}
	for _, d := range apiFile.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, sp := range gd.Specs {
			ts := sp.(*ast.TypeSpec)
			if !ts.Name.IsExported() {
				continue
			}
			users[ts.Name.Name] = map[string]bool{}
			doc := ts.Doc
			if doc == nil {
				doc = gd.Doc
			}
			if m := sharedPending.FindStringSubmatch(doc.Text()); m != nil {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(m[1]))); err == nil {
					pending[ts.Name.Name] = true
				}
			}
		}
	}
	return users, pending
}

// countUses records dir as a user of each type of users that the file at path names as <jobs import>.<Type>.
func countUses(path, dir, jobsPath string, users map[string]map[string]bool) error {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	name := ""
	for _, im := range f.Imports {
		if p, _ := strconv.Unquote(im.Path.Value); p == jobsPath {
			name = "jobs"
			if im.Name != nil {
				name = im.Name.Name
			}
		}
	}
	if name == "" {
		return nil
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if se, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := se.X.(*ast.Ident); ok && id.Name == name && users[se.Sel.Name] != nil {
				users[se.Sel.Name][dir] = true
			}
		}
		return true
	})
	return nil
}

func modulePath(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if m, ok := strings.CutPrefix(strings.TrimSpace(l), "module "); ok {
			return strings.TrimSpace(m), nil
		}
	}
	return "", fmt.Errorf("no module line in go.mod")
}

// TestAPITypeHasTwoUserPkgs: every exported type of api.go is used by two packages, or marked shared-pending.
func TestAPITypeHasTwoUserPkgs(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	problems, err := apiTypeProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func userFile(pkg, imp string, types ...string) string {
	s := "package " + pkg + "\n\nimport " + imp + " \"example.com/m/internal/jobs\"\n\nvar _ = []any{"
	for _, ty := range types {
		s += "new(" + imp + "." + ty + "), "
	}
	return s + "}\n"
}

// TestUsersCounterOnTempTree: the counter counts distinct directories, leaves out cmd/ot, the root of
// internal/jobs and test files, follows the import name, and lets the mark pass a type with a design file.
func TestUsersCounterOnTempTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod": "module example.com/m\n",
		"internal/jobs/api.go": `package jobs

type Shared struct{}
type OneUser struct{}
type OnlyOt struct{}
type OnlyRoot struct{}
type OnlyTests struct{}
type Unused struct{}

// Pending has no user yet.
// shared-pending: docs/design/exists.md
type Pending struct{}

// Ghost names a design file that does not exist.
// shared-pending: docs/design/missing.md
type Ghost struct{}

type hidden struct{}
`,
		"docs/design/exists.md":     "# design\n",
		"internal/jobs/root_use.go": userFile("jobs", "jobs", "OnlyRoot"),
		"cmd/ot/use.go":             userFile("main", "jobs", "Shared", "OneUser", "OnlyOt"),
		"internal/a/a.go":           userFile("a", "jobs", "Shared", "OneUser"),
		"internal/a/b.go":           userFile("a", "jobs", "Shared"),
		"internal/b/b.go":           userFile("b", "jj", "Shared"),
		"internal/c/c_test.go":      userFile("c", "jobs", "Shared", "OnlyTests"),
	})
	got, err := apiTypeProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Ghost has 0 user packages []",
		"OneUser has 1 user packages [internal/a]",
		"OnlyOt has 0 user packages []",
		"OnlyRoot has 0 user packages []",
		"OnlyTests has 0 user packages []",
		"Unused has 0 user packages []",
	}
	if len(got) != len(want) {
		t.Fatalf("problems %q, want one each for %q", got, want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Fatalf("problem %d is %q, want it to start %q", i, got[i], want[i])
		}
	}

	// a second user package for OneUser clears it; the mark is never a problem with users either
	writeTree(t, root, map[string]string{"internal/d/d.go": userFile("d", "jobs", "OneUser", "Pending")})
	got, err = apiTypeProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if strings.HasPrefix(p, "OneUser") || strings.HasPrefix(p, "Pending") {
			t.Fatalf("unexpected problem %q", p)
		}
	}
}
