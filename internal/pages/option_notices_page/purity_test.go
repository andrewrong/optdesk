package option_notices_page

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

func TestNoFloatNoSortNoNotify(t *testing.T) {
	t.Parallel()
	banned := map[string]bool{
		`"sort"`: true, `"os"`: true, `"net/http"`: true, `"database/sql"`: true,
		`"github.com/andrewrong/optdesk/internal/notify"`: true, `"github.com/andrewrong/optdesk/internal/jobs"`: true,
	}
	for name, f := range sources(t) {
		slices := ""
		for _, imp := range f.Imports {
			if banned[imp.Path.Value] {
				t.Errorf("%s imports %s", name, imp.Path.Value)
			}
			if imp.Path.Value == `"slices"` {
				slices = "slices"
				if imp.Name != nil {
					slices = imp.Name.Name
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "float32" || x.Name == "float64" {
					t.Errorf("%s uses %s", name, x.Name)
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && slices != "" && id.Name == slices && strings.HasPrefix(x.Sel.Name, "Sort") {
					t.Errorf("%s calls slices.%s", name, x.Sel.Name)
				}
			}
			return true
		})
	}
}

func TestNoClockNoIO(t *testing.T) {
	t.Parallel()
	for name, f := range sources(t) {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "time" && (sel.Sel.Name == "Now" || sel.Sel.Name == "Sleep") {
					t.Errorf("%s calls time.%s", name, sel.Sel.Name)
				}
			}
			return true
		})
	}
}
