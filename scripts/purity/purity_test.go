package purity

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// repoRoot is the repository root seen from this package.
const repoRoot = "../.."

// floatForm says how a row finds a float.
type floatForm int

const (
	floatNone   floatForm = iota
	floatIdent            // the identifier float32 or float64 anywhere in the syntax tree
	floatText             // the text float32 or float64 anywhere in the source, comments included
	floatText64           // the text float64 only
	floatConv             // a conversion call float32(...) or float64(...)
)

// clockForm says which time functions a row bans.
type clockForm int

const (
	clockNone          clockForm = iota
	clockNowSleep                // time.Now, time.Sleep
	clockNow                     // time.Now
	clockNowSinceUntil           // time.Now, time.Since, time.Until
)

// selBan bans a selector Pkg.Name. An empty Pkg matches any operand, an empty Name any selector, and Prefix
// makes Name a prefix. Pkg is the package name or the local name the file gives its import.
type selBan struct {
	Pkg, Name string
	Prefix    bool
}

// rule is the purity rule of one package; the zero value bans nothing.
type rule struct {
	Files        []string                                 // the files scanned; empty means every non-test .go file
	Select       func(root, dir string) ([]string, error) // replaces the default file list (the whole-tree walk)
	Require      []string                                 // a file name prefix that must match at least one file
	ClockOnly    []string                                 // files scanned for the clock rule only
	Match        matchForm
	Banned       []string                                // banned import paths, see importBanned
	BannedSubstr []string                                // banned import path substrings
	Float        floatForm                               // float rule
	Clock        clockForm                               // clock rule
	Text         []string                                // text banned anywhere in the source
	Idents       []string                                // identifiers banned anywhere
	Sels         []selBan                                // selectors banned
	Extra        func(name string, f *ast.File) []string // a rule that is not data, see rules_test.go
}

// row is one package of the table.
type row struct {
	Dir  string
	Rule rule
}

// matchForm is the import matcher of a row.
type matchForm int

const (
	matchMerged matchForm = iota // the path equals bad, starts with bad/ or contains /bad
	matchEnds                    // the path equals bad, starts with bad/ or ends with /bad
)

// importBanned is the matcher: the merged form (default) catches everything each old matcher caught. A row
// that the merged form over-reports on main (/internal/compute/companydataapi contains /internal/compute/companydata)
// keeps matchEnds, the closest old form.
func importBanned(m matchForm, path, bad string) bool {
	if path == bad || strings.HasPrefix(path, bad+"/") {
		return true
	}
	if m == matchEnds {
		return strings.HasSuffix(path, "/"+bad)
	}
	return strings.Contains(path, "/"+bad)
}

func importPath(imp *ast.ImportSpec) string {
	p, _ := strconv.Unquote(imp.Path.Value)
	return p
}

// names returns the names under which f can write the package pkg: pkg itself and each local import name.
func names(f *ast.File, pkg string) map[string]bool {
	out := map[string]bool{pkg: true}
	for _, imp := range f.Imports {
		p := importPath(imp)
		if imp.Name != nil && (p == pkg || strings.HasSuffix(p, "/"+pkg)) {
			out[imp.Name.Name] = true
		}
	}
	return out
}

func (c clockForm) names() []string {
	switch c {
	case clockNone:
		return nil
	case clockNowSleep:
		return []string{"Now", "Sleep"}
	case clockNow:
		return []string{"Now"}
	case clockNowSinceUntil:
		return []string{"Now", "Since", "Until"}
	}
	return nil
}

// scanSource applies r to one source text and returns its violations. full false applies the clock rule only.
func scanSource(r rule, name, src string, full bool) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	add := func(pos token.Pos, format string, a ...any) {
		out = append(out, fmt.Sprintf("%s: %s", fset.Position(pos), fmt.Sprintf(format, a...)))
	}
	if full {
		out = append(out, scanImports(r, f, fset)...)
		out = append(out, scanText(r, name, src)...)
		if r.Extra != nil {
			for _, v := range r.Extra(name, f) {
				out = append(out, fmt.Sprintf("%s: %s", name, v))
			}
		}
	}
	timeNames := names(f, "time")
	clock := r.Clock.names()
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			scanSelector(r, f, x, timeNames, clock, full, add)
		case *ast.Ident:
			if full {
				scanIdent(r, x, add)
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && full && r.Float == floatConv && isFloatName(id.Name) {
				add(x.Pos(), "converts to %s", id.Name)
			}
		}
		return true
	})
	return out, nil
}

func isFloatName(s string) bool { return s == "float32" || s == "float64" }

func scanImports(r rule, f *ast.File, fset *token.FileSet) []string {
	var out []string
	for _, imp := range f.Imports {
		p := importPath(imp)
		for _, b := range r.Banned {
			if importBanned(r.Match, p, b) {
				out = append(out, fmt.Sprintf("%s: imports %s (banned %s)", fset.Position(imp.Pos()), p, b))
			}
		}
		for _, s := range r.BannedSubstr {
			if strings.Contains(p, s) {
				out = append(out, fmt.Sprintf("%s: imports %s (banned text %s)", fset.Position(imp.Pos()), p, s))
			}
		}
	}
	return out
}

// scanText applies the rules that read the source text, comments included.
func scanText(r rule, name, src string) []string {
	words := append([]string{}, r.Text...)
	switch r.Float {
	case floatText:
		words = append(words, "float32", "float64")
	case floatText64:
		words = append(words, "float64")
	case floatNone, floatIdent, floatConv: // the other forms read the syntax tree
	}
	var out []string
	for _, w := range words {
		if strings.Contains(src, w) {
			out = append(out, fmt.Sprintf("%s: mentions %s", name, w))
		}
	}
	return out
}

func scanSelector(r rule, f *ast.File, x *ast.SelectorExpr, timeNames map[string]bool, clock []string, full bool,
	add func(token.Pos, string, ...any)) {
	id, _ := x.X.(*ast.Ident)
	if id == nil {
		return
	}
	if timeNames[id.Name] && contains(clock, x.Sel.Name) {
		add(x.Pos(), "reads the clock with %s.%s", id.Name, x.Sel.Name)
	}
	if !full {
		return
	}
	for _, s := range r.Sels {
		if (s.Pkg == "" || names(f, s.Pkg)[id.Name]) &&
			(s.Name == "" || x.Sel.Name == s.Name || (s.Prefix && strings.HasPrefix(x.Sel.Name, s.Name))) {
			add(x.Pos(), "uses %s.%s", id.Name, x.Sel.Name)
		}
	}
}

func scanIdent(r rule, x *ast.Ident, add func(token.Pos, string, ...any)) {
	if r.Float == floatIdent && isFloatName(x.Name) || contains(r.Idents, x.Name) {
		add(x.Pos(), "identifier %s", x.Name)
	}
}

// nonTestFiles lists the non-test .go files of dir, by name relative to dir.
func nonTestFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}

func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}

// scanDir applies the row to the package under root and returns its violations. It is an error when the
// directory is missing, a named file is missing or no non-test file is found.
func scanDir(root string, r row) ([]string, error) {
	dir := filepath.Join(root, r.Dir)
	var files []string
	var err error
	switch {
	case r.Rule.Select != nil:
		files, err = r.Rule.Select(root, r.Dir)
	case len(r.Rule.Files) > 0:
		files = r.Rule.Files
	default:
		files, err = nonTestFiles(dir)
	}
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s holds no non-test .go file", r.Dir)
	}
	for _, prefix := range r.Rule.Require {
		found := false
		for _, f := range files {
			found = found || strings.HasPrefix(filepath.Base(f), prefix)
		}
		if !found {
			return nil, fmt.Errorf("%s holds no file starting %s", r.Dir, prefix)
		}
	}
	var out []string
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return nil, err
		}
		v, err := scanSource(r.Rule, f, string(src), !contains(r.Rule.ClockOnly, filepath.Base(f)))
		if err != nil {
			return nil, err
		}
		for _, s := range v {
			out = append(out, r.Dir+"/"+s)
		}
	}
	return out, nil
}

// TestPurity runs every row of the table as a parallel subtest.
func TestPurity(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, r := range table {
		if seen[r.Dir] {
			t.Fatalf("two rows for %s", r.Dir)
		}
		seen[r.Dir] = true
		t.Run(r.Dir, func(t *testing.T) {
			t.Parallel()
			violations, err := scanDir(repoRoot, r)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range violations {
				t.Error(v)
			}
		})
	}
}

// TestScannerFailsOnBadSource: each kind of bad source makes the scanner report, and good source does not.
func TestScannerFailsOnBadSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		r    rule
		src  string
		want int
	}{
		{"banned import", rule{Banned: []string{"os"}}, "package p\nimport \"os\"\nvar _ = os.Args\n", 1},
		{"banned import by suffix", rule{Banned: []string{"internal/jobs"}}, "package p\nimport _ \"x.com/m/internal/jobs\"\n", 1},
		{"float name", rule{Float: floatIdent}, "package p\nvar x float64\n", 1},
		{"float text in a comment", rule{Float: floatText}, "package p\n// float32\n", 1},
		{"float conversion", rule{Float: floatConv}, "package p\nvar x = float64(1)\n", 1},
		{"clock call", rule{Clock: clockNow}, "package p\nimport \"time\"\nvar _ = time.Now()\n", 1},
		{"clock call under a local name", rule{Clock: clockNow}, "package p\nimport t \"time\"\nvar _ = t.Now()\n", 1},
		{"good source", rule{Banned: []string{"os"}, Float: floatIdent, Clock: clockNow},
			"package p\nimport \"strings\"\nvar _ = strings.ToUpper(\"a\")\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := scanSource(c.r, "x.go", c.src, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != c.want {
				t.Fatalf("got %d violations %v, want %d", len(got), got, c.want)
			}
		})
	}
}

// TestScanDirFailsOnMissingOrEmptyDir: a row whose directory is missing or holds no non-test file is an error.
func TestScanDirFailsOnMissingOrEmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty", "a_test.go"), []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"missing", "empty"} {
		if _, err := scanDir(root, row{Dir: dir}); err == nil {
			t.Errorf("%s: no error", dir)
		}
	}
}

// TestNotifyRowBansOptionnoticeSubstring: the notify row bans any import whose path contains optionnotice.
func TestNotifyRowBansOptionnoticeSubstring(t *testing.T) {
	t.Parallel()
	var r rule
	for _, x := range table {
		if x.Dir == "internal/notify" {
			r = x.Rule
		}
	}
	src := "package p\nimport _ \"github.com/andrewrong/optdesk/internal/compute/legacyoptionnotice\"\n"
	got, err := scanSource(r, "a.go", src, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("import containing optionnotice not reported; the old notify test reported it")
	}
}
