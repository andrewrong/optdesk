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
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/compute"
)

var _ compute.NewRulesFunc = New

// numbers is the canonical JSON of a rule without its hash (Decision 12): the struct order
// is the key order.
type numbers struct {
	ID          string `json:"id"`
	Version     int    `json:"version"`
	Convention  string `json:"convention"`
	Unit        string `json:"unit"`
	Scale       int    `json:"scale"`
	Compare     string `json:"compare"`
	N           int    `json:"n"`
	Window      int    `json:"window"`
	MinValid    int    `json:"min_valid"`
	Years       int    `json:"years"`
	TradingDays int    `json:"trading_days"`
	MaxAgeDays  int    `json:"max_age_days"`
}

func (e entry) numbers() numbers {
	return numbers{e.ID, e.Version, e.Convention, e.Unit, e.Scale, e.Compare, e.N, e.Window,
		e.MinValid, e.Years, e.TradingDays, e.MaxAgeDays}
}

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

// funcSource prints the normalized source of one implemented_by name under root, such as
// "internal/compute/figures.(*calc).Iv30" or "internal/compute/figures.strikeStar".
func funcSource(root, name string) (string, error) {
	idx := strings.LastIndex(name, "/")
	pathPart, rest := name[:idx+1], name[idx+1:]
	pkg, rest, _ := strings.Cut(rest, ".")
	pathPart += pkg
	recv := ""
	if strings.HasPrefix(rest, "(*") {
		r, fn, _ := strings.Cut(rest, ").")
		recv, rest = strings.TrimPrefix(r, "(*"), fn
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, filepath.Join(root, filepath.FromSlash(pathPart)),
		func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		return "", err
	}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Name.Name != rest || recvName(fd) != recv {
					continue
				}
				var buf bytes.Buffer
				if err := printer.Fprint(&buf, token.NewFileSet(), fd); err != nil {
					return "", err
				}
				return buf.String(), nil
			}
		}
	}
	return "", fmt.Errorf("%s not found", name)
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
// order, then the canonical JSON of the numbers.
func ruleHash(root string, impl []string, n numbers) (string, error) {
	h := sha256.New()
	for _, name := range impl {
		src, err := funcSource(root, name)
		if err != nil {
			return "", err
		}
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
		got, err := ruleHash(root, f.ImplementedBy, e.numbers())
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
	base, err := ruleHash(tmp, f.ImplementedBy, e.numbers())
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
		h, err := ruleHash(tmp, f.ImplementedBy, e.numbers())
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
