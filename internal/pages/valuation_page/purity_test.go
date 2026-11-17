package valuation_page

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func nonTestFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatal("no source file checked")
	}
	return out
}

func importsOf(t *testing.T, f string) []string {
	t.Helper()
	af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, im := range af.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		out = append(out, p)
	}
	return out
}

// TestSourceStaysPure: the package imports no database, file, network, store implementation or clock package.
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
	forbidden := []string{"database/sql", "os", "net/http", "internal/clock", "internal/store/valuationpg", "internal/store/underlyingpg",
		"github.com/jackc/pgx", "internal/compute/valuelines"}
	for _, f := range nonTestFiles(t) {
		for _, p := range importsOf(t, f) {
			for _, bad := range forbidden {
				if p == bad || strings.HasPrefix(p, bad+"/") || strings.HasSuffix(p, "/"+bad) {
					t.Errorf("%s imports %s", f, p)
				}
			}
		}
	}
}

// TestNoArithmeticInBuilders: no builder multiplies, divides, rounds or positions a figure; the files import no
// decimal or math package and never parse a float.
func TestNoArithmeticInBuilders(t *testing.T) {
	t.Parallel()
	for _, f := range nonTestFiles(t) {
		for _, p := range importsOf(t, f) {
			if p == "math" || strings.HasPrefix(p, "math/") || strings.Contains(p, "decimal") {
				t.Errorf("%s imports %s", f, p)
			}
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "strconv" && strings.HasPrefix(sel.Sel.Name, "ParseFloat") {
					t.Errorf("%s calls strconv.%s", f, sel.Sel.Name)
				}
			}
			return true
		})
	}
}
