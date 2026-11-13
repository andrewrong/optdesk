package optionfig

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPurity: the non-test files import no store, jobs, notify or os and never call time.Now.
func TestPurity(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"/internal/store", "/internal/jobs", "/internal/notify"}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == "os" {
				t.Errorf("%s imports os", f)
			}
			for _, b := range banned {
				if strings.Contains(p, b) {
					t.Errorf("%s imports %s", f, p)
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && id.Name == "time" && s.Sel.Name == "Now" {
					t.Errorf("%s uses time.Now", f)
				}
			}
			return true
		})
	}
}
