package option_pnl_page

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// sources parses the non-test .go files of the package.
func sources(t *testing.T) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = f
	}
	if len(out) == 0 {
		t.Fatal("no source files")
	}
	return out
}

func TestNoFloatNoSortNoNotifyNoMath(t *testing.T) {
	t.Parallel()
	banned := map[string]bool{`"math/big"`: true, `"sort"`: true, `"os"`: true, `"net/http"`: true,
		`"database/sql"`: true, `"math"`: true}
	arith := map[string]bool{"Add": true, "Sub": true, "Mul": true, "Div": true}
	for name, f := range sources(t) {
		for _, imp := range f.Imports {
			if banned[imp.Path.Value] {
				t.Errorf("%s imports %s", name, imp.Path.Value)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "float32" || x.Name == "float64" {
					t.Errorf("%s uses %s", name, x.Name)
				}
			case *ast.SelectorExpr:
				if arith[x.Sel.Name] {
					t.Errorf("%s calls %s", name, x.Sel.Name)
				}
			}
			return true
		})
	}
}

// marks.go does only sign tests on figures.
func TestMarksDoOnlySignTests(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{"Sign": true, "IsPositive": true, "NewFromString": true}
	f := sources(t)["marks.go"]
	ast.Inspect(f, func(n ast.Node) bool {
		if x, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "decimal" && !allowed[x.Sel.Name] {
				t.Errorf("marks.go uses decimal.%s", x.Sel.Name)
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name != "Sign" && sel.Sel.Name != "IsPositive" && sel.Sel.Name != "NewFromString" {
				t.Errorf("marks.go calls .%s", sel.Sel.Name)
			}
		}
		return true
	})
}
