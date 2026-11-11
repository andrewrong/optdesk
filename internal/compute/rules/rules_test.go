package rules

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/compute"
)

var _ compute.NewRulesFunc = New

type figureEntry struct {
	ID            string   `toml:"id"`
	Version       int      `toml:"version"`
	Formula       string   `toml:"formula"`
	Currency      string   `toml:"currency"`
	ImplementedBy []string `toml:"implemented_by"`
	SourceHash    string   `toml:"source_hash"`
}

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
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func readRegister(t *testing.T) ([]figureEntry, []map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "conventions", "register.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var typed struct {
		Figure []figureEntry `toml:"figure"`
	}
	var loose struct {
		Figure []map[string]any `toml:"figure"`
	}
	if err := toml.Unmarshal(raw, &typed); err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(raw, &loose); err != nil {
		t.Fatal(err)
	}
	return typed.Figure, loose.Figure
}

func readEntries(t *testing.T) []entry {
	t.Helper()
	var es []entry
	if err := json.Unmarshal(tableJSON, &es); err != nil {
		t.Fatal(err)
	}
	return es
}

func findFigure(figs []figureEntry, id string, version int) (figureEntry, bool) {
	for _, f := range figs {
		if f.ID == id && f.Version == version {
			return f, true
		}
	}
	return figureEntry{}, false
}

// splitName cuts an implemented_by name such as "internal/compute/figures.(*calc).Iv30" or
// "internal/compute/figures.strikeStar" into the package path and the key "calc.Iv30" or
// ".strikeStar" (receiver, dot, name).
func splitName(name string) (pathPart, key string) {
	idx := strings.LastIndex(name, "/")
	pkg, rest, _ := strings.Cut(name[idx+1:], ".")
	pathPart = name[:idx+1] + pkg
	recv := ""
	if strings.HasPrefix(rest, "(*") {
		r, fn, _ := strings.Cut(rest, ").")
		recv, rest = strings.TrimPrefix(r, "(*"), fn
	}
	return pathPart, recv + "." + rest
}

// pkgFuncs parses the non-test files of one package directory and keys each function by
// "recv.name".
func pkgFuncs(root, pathPart string) (map[string]*ast.FuncDecl, error) {
	dir := filepath.Join(root, filepath.FromSlash(pathPart))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	out := map[string]*ast.FuncDecl{}
	for _, de := range ents {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".go") || strings.HasSuffix(de.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, de.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				out[recvName(fd)+"."+fd.Name.Name] = fd
			}
		}
	}
	return out, nil
}

// callees lists the keys of the functions of the package that fd calls: a plain call of a
// package function, or a selector call whose name is a method of the package.
func callees(fd *ast.FuncDecl, funcs map[string]*ast.FuncDecl) []string {
	var keys []string
	ast.Inspect(fd, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if _, ok := funcs["."+fun.Name]; ok {
				keys = append(keys, "."+fun.Name)
			}
		case *ast.SelectorExpr:
			for k := range funcs {
				if recv, name, _ := strings.Cut(k, "."); recv != "" && name == fun.Sel.Name {
					keys = append(keys, k)
				}
			}
		}
		return true
	})
	return keys
}

// hashedSources gives the normalized source of the listed functions in order, then of the
// package functions they call, directly or through each other, sorted by key. The register
// lists the callees by hand (Decision 5) and may miss one; a figure's number depends on every
// helper it reaches, so the hash follows the calls itself.
func hashedSources(root string, impl []string) ([]string, error) {
	cache := map[string]map[string]*ast.FuncDecl{}
	type item struct {
		pkg, key string
	}
	var order, queue []item
	seen := map[item]bool{}
	for _, name := range impl {
		pkg, key := splitName(name)
		it := item{pkg, key}
		if !seen[it] {
			seen[it] = true
			order = append(order, it)
			queue = append(queue, it)
		}
	}
	var extra []item
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		funcs, ok := cache[it.pkg]
		if !ok {
			var err error
			if funcs, err = pkgFuncs(root, it.pkg); err != nil {
				return nil, err
			}
			cache[it.pkg] = funcs
		}
		fd, ok := funcs[it.key]
		if !ok {
			return nil, fmt.Errorf("%s %s not found", it.pkg, it.key)
		}
		for _, k := range callees(fd, funcs) {
			c := item{it.pkg, k}
			if !seen[c] {
				seen[c] = true
				extra = append(extra, c)
				queue = append(queue, c)
			}
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].key < extra[j].key })
	var out []string
	for _, it := range append(order, extra...) {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, token.NewFileSet(), cache[it.pkg][it.key]); err != nil {
			return nil, err
		}
		out = append(out, buf.String())
	}
	return out, nil
}

func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// ruleHash is Decision 12: sha256 over the normalized source of each listed function in
// order, then of the helpers they call, then the canonical JSON of the numbers.
func ruleHash(root string, impl []string, n numbers) (string, error) {
	h := sha256.New()
	srcs, err := hashedSources(root, impl)
	if err != nil {
		return "", err
	}
	for _, src := range srcs {
		h.Write([]byte(src))
		h.Write([]byte("\n"))
	}
	b, err := json.Marshal(n)
	if err != nil {
		return "", err
	}
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func TestHashesMatchSource(t *testing.T) {
	t.Parallel()
	t.Run("edits", func(t *testing.T) { t.Parallel(); checkHashSensitivity(t) })
	root := repoRoot(t)
	figs, _ := readRegister(t)
	for _, e := range readEntries(t) {
		f, ok := findFigure(figs, e.ID, e.Version)
		if !ok {
			t.Fatalf("%s v%d not in register", e.ID, e.Version)
		}
		got, err := ruleHash(root, f.ImplementedBy, e.numbers)
		if err != nil {
			t.Fatal(err)
		}
		if e.Hash != got {
			t.Errorf("%s: rules.json hash %q, source gives %q", e.ID, e.Hash, got)
		}
		if f.SourceHash != got {
			t.Errorf("%s: register source_hash %q, source gives %q", e.ID, f.SourceHash, got)
		}
	}
}

// checkHashSensitivity runs the edit cases of TestHashesMatchSource on a copy of the
// figures package.
func checkHashSensitivity(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	figs, _ := readRegister(t)
	var e entry
	for _, c := range readEntries(t) {
		if c.ID == "market.iv30_own" {
			e = c
		}
	}
	f, _ := findFigure(figs, e.ID, e.Version)
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "internal", "compute", "figures")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "internal", "compute", "figures")
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ents {
		b, err := os.ReadFile(filepath.Join(src, d.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, d.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base, err := ruleHash(tmp, f.ImplementedBy, e.numbers)
	if err != nil {
		t.Fatal(err)
	}
	edit := func(file, from, to string) {
		t.Helper()
		p := filepath.Join(dst, file)
		b, _ := os.ReadFile(p)
		if !strings.Contains(string(b), from) {
			t.Fatalf("%s lacks %q", file, from)
		}
		if err := os.WriteFile(p, []byte(strings.Replace(string(b), from, to, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hash := func() string {
		h, err := ruleHash(tmp, f.ImplementedBy, e.numbers)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	// whitespace only: extra blank line and a trailing space-free reindent.
	edit("iv30.go", "func (c calc) Iv30(", "\n\n\nfunc (c calc) Iv30(")
	if hash() != base {
		t.Error("a whitespace edit changed the hash")
	}
	edit("iv30.go", "func strikeStar(", "func strikeStar(/* x */")
	if hash() != base {
		t.Error("a comment edit changed the hash")
	}
	edit("iv30.go", "func strikeStar(/* x */", "func strikeStar(")
	// a body edit of the method.
	b, _ := os.ReadFile(filepath.Join(dst, "iv30.go"))
	i := strings.Index(string(b), "func (c calc) Iv30(")
	j := strings.Index(string(b)[i:], "{\n") + i + 2
	if err := os.WriteFile(filepath.Join(dst, "iv30.go"),
		[]byte(string(b)[:j]+"\t_ = 1\n"+string(b)[j:]), 0o644); err != nil {
		t.Fatal(err)
	}
	if hash() == base {
		t.Error("a body edit did not change the hash")
	}
	// a body edit of the helper alone.
	edit("iv30.go", "\t_ = 1\n", "")
	if hash() != base {
		t.Fatal("restore failed")
	}
	b, _ = os.ReadFile(filepath.Join(dst, "iv30.go"))
	i = strings.Index(string(b), "func strikeStar(")
	j = strings.Index(string(b)[i:], "{\n") + i + 2
	if err := os.WriteFile(filepath.Join(dst, "iv30.go"),
		[]byte(string(b)[:j]+"\t_ = 2\n"+string(b)[j:]), 0o644); err != nil {
		t.Fatal(err)
	}
	if hash() == base {
		t.Error("a change of a listed helper did not change the hash")
	}
}

func TestRulesMatchRegister(t *testing.T) {
	t.Parallel()
	figs, _ := readRegister(t)
	unitRe := regexp.MustCompile(`^Unit (\w+), stored as decimal text, rounded half-even to (\d+) places\.`)
	rules := New()
	for _, e := range readEntries(t) {
		f, ok := findFigure(figs, e.ID, e.Version)
		if !ok {
			t.Errorf("%s v%d: no register entry", e.ID, e.Version)
			continue
		}
		r, ok := rules.Rule(e.ID)
		if !ok || r.Version != e.Version || r.Unit != e.Unit || r.Scale != e.Scale {
			t.Errorf("%s: Rules gives %+v", e.ID, r)
		}
		m := unitRe.FindStringSubmatch(f.Currency)
		if m == nil {
			t.Errorf("%s: currency %q has no unit and half-even sentence", e.ID, f.Currency)
			continue
		}
		if m[1] != e.Unit || m[2] != fmt.Sprint(e.Scale) {
			t.Errorf("%s: currency says %s/%s, rule %s/%d", e.ID, m[1], m[2], e.Unit, e.Scale)
		}
		switch e.Compare {
		case "at_or_below":
			if !strings.Contains(f.Formula, "at or below") {
				t.Errorf("%s: formula does not say at or below", e.ID)
			}
		case "below":
			if !strings.Contains(f.Formula, "strictly below") {
				t.Errorf("%s: formula does not say strictly below", e.ID)
			}
		}
	}
	if got, want := len(rules.IDs()), len(readEntries(t)); got != want {
		t.Errorf("IDs: %d, rules.json ids: %d", got, want)
	}
}

func TestCompareFixedPerID(t *testing.T) {
	t.Parallel()
	rules := New()
	want := map[string]compute.Compare{
		"market.hv30_own":        compute.Compare(""),
		"market.ivp_own":         compute.AtOrBelow,
		"market.vix_pctile_3y":   compute.Below,
		"market.vxn_pctile_3y":   compute.Below,
		"market.rvx_pctile_3y":   compute.Below,
		"market.sp500_pctile_3y": compute.Below,
	}
	for id, c := range want {
		r, ok := rules.Rule(id)
		if !ok || r.Compare != c {
			t.Errorf("%s: compare %q, want %q", id, r.Compare, c)
		}
	}
}

func TestMarketRv20Numbers(t *testing.T) {
	t.Parallel()
	r, ok := New().Rule("market.rv20")
	if !ok {
		t.Fatal("market.rv20 not found")
	}
	if r.Version != 1 || r.Convention != "rv20_v1" || r.Unit != "decimal" || r.Scale != 6 ||
		r.N != 20 || r.TradingDays != 252 || r.MaxAgeDays != 4 || r.Compare != "" {
		t.Errorf("market.rv20: %+v", r)
	}
}

func TestNoPendingForGoIds(t *testing.T) {
	t.Parallel()
	figs, _ := readRegister(t)
	hexRe := regexp.MustCompile(`^[0-9a-f]{64}$`)
	goIDs := map[[2]string]bool{}
	for _, e := range readEntries(t) {
		goIDs[[2]string{e.ID, fmt.Sprint(e.Version)}] = true
	}
	for _, f := range figs {
		if goIDs[[2]string{f.ID, fmt.Sprint(f.Version)}] {
			if !hexRe.MatchString(f.SourceHash) {
				t.Errorf("%s v%d: source_hash %q is not 64 hex", f.ID, f.Version, f.SourceHash)
			}
		} else if f.SourceHash != "pending-recompute" {
			t.Errorf("%s v%d: source_hash %q, want pending-recompute", f.ID, f.Version, f.SourceHash)
		}
	}
}

func TestRegisterEntriesComplete(t *testing.T) {
	t.Parallel()
	_, loose := readRegister(t)
	keys := []string{"id", "version", "title_zh", "pages", "formula", "inputs", "period_date",
		"currency", "excludes", "gap_text", "implemented_by", "source_hash", "agreed_on",
		"agreed_in", "history"}
	seen := map[string]bool{}
	for _, f := range loose {
		for _, k := range keys {
			if _, ok := f[k]; !ok {
				t.Errorf("%v: key %q missing", f["id"], k)
			}
		}
		id := fmt.Sprintf("%v/%v", f["id"], f["version"])
		if seen[id] {
			t.Errorf("duplicate entry %s", id)
		}
		seen[id] = true
	}
}

func TestHashCoversIvWindow(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	figs, _ := readRegister(t)
	var e entry
	for _, c := range readEntries(t) {
		if c.ID == "market.ivp_own" {
			e = c
		}
	}
	f, _ := findFigure(figs, e.ID, e.Version)
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "internal", "compute", "figures")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "internal", "compute", "figures")
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ents {
		b, err := os.ReadFile(filepath.Join(src, d.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, d.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base, err := ruleHash(tmp, f.ImplementedBy, e.numbers)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dst, "ivp.go")
	b, _ := os.ReadFile(p)
	s := string(b)
	i := strings.Index(s, "func (c calc) ivWindow(")
	j := strings.Index(s[i:], "{\n") + i + 2
	if err := os.WriteFile(p, []byte(s[:j]+"\t_ = 3\n"+s[j:]), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := ruleHash(tmp, f.ImplementedBy, e.numbers); got == base {
		t.Fatal("an edit of ivWindow, which IvPercentile calls, leaves the ivp_own rule hash unchanged")
	}
}

func TestHashCoversEaseHelpers(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	figs, _ := readRegister(t)
	var e entry
	for _, c := range readEntries(t) {
		if c.ID == "macro.ease" {
			e = c
		}
	}
	f, ok := findFigure(figs, e.ID, e.Version)
	if !ok {
		t.Fatal("macro.ease not in the register")
	}
	tmp := t.TempDir()
	dst := copyPackage(t, root, tmp, "ease")
	hash := func() string {
		h, err := ruleHash(tmp, f.ImplementedBy, e.numbers)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := hash()
	if base != e.Hash {
		t.Fatalf("the copy hashes to %q, rules.json says %q", base, e.Hash)
	}
	// edit replaces the first match of from in a file of the copy.
	edit := func(file, from, to string) {
		t.Helper()
		p := filepath.Join(dst, file)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), from) {
			t.Fatalf("%s lacks %q", file, from)
		}
		if err := os.WriteFile(p, []byte(strings.Replace(string(b), from, to, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	helpers := map[string]string{
		"params":        "params.go",
		"round2":        "ease.go",
		"ratOfFloat":    "ease.go",
		"wholeHalfUp":   "ease.go",
		"weekdaysAfter": "ease.go",
		"usChange":      "ease.go",
		"hyLevel":       "ease.go",
		"age":           "ease.go",
		"ratsByDay":     "netliquidity.go",
		"noFigure":      "netliquidity.go",
		"sortedDays":    "netliquidity.go",
	}
	// whitespace only: blank lines before the function change nothing.
	for name, file := range helpers {
		edit(file, "func "+name+"(", "\n\n\nfunc "+name+"(")
	}
	if hash() != base {
		t.Error("a whitespace-only edit changed the hash")
	}
	for name, file := range helpers {
		b, _ := os.ReadFile(filepath.Join(dst, file))
		s := string(b)
		i := strings.Index(s, "func "+name+"(")
		j := strings.Index(s[i:], "{\n") + i + 2
		if err := os.WriteFile(filepath.Join(dst, file), []byte(s[:j]+"\t_ = 1\n"+s[j:]), 0o644); err != nil {
			t.Fatal(err)
		}
		if hash() == base {
			t.Errorf("a body edit of %s did not change the hash", name)
		}
		edit(file, "\t_ = 1\n", "")
		if hash() != base {
			t.Fatalf("restoring %s failed", name)
		}
	}
}

// copyPackage copies the files of internal/compute/<pkg> under root into the same path under
// tmp and gives the new directory.
func copyPackage(t *testing.T, root, tmp, pkg string) string {
	t.Helper()
	dst := filepath.Join(tmp, "internal", "compute", pkg)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "internal", "compute", pkg)
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ents {
		if d.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, d.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, d.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func TestHashCoversEaseScoreLogic(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	figs, _ := readRegister(t)
	var e entry
	for _, c := range readEntries(t) {
		if c.ID == "macro.ease" {
			e = c
		}
	}
	f, ok := findFigure(figs, e.ID, e.Version)
	if !ok {
		t.Fatal("macro.ease not in the register")
	}
	tmp := t.TempDir()
	dst := copyPackage(t, root, tmp, "ease")
	p := filepath.Join(dst, "ease.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	from := "obs[days[n-1-p.usLookback]]"
	if !strings.Contains(string(b), from) {
		t.Fatalf("ease.go lacks %q", from)
	}
	s := strings.Replace(string(b), from, "obs[days[n-p.usLookback]]", 1)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := ruleHash(tmp, f.ImplementedBy, e.numbers)
	if err != nil {
		t.Fatal(err)
	}
	if h == e.Hash {
		t.Fatal("changing which DGS10 observation usChange subtracts leaves the macro.ease hash unchanged")
	}
}
