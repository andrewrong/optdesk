package company_data_page

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestSourceStaysPure: no non-test file imports a math or decimal package, the compute implementation, a
// database or file package, reads a clock, or converts to a float.
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"builder.go", "rows.go", "strings.go"} {
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "math" || strings.HasPrefix(p, "math/") || strings.Contains(p, "decimal") || p == "os" || p == "database/sql" ||
				strings.HasSuffix(p, "internal/compute/companydata") {
				t.Errorf("%s imports %s", f, p)
			}
		}
		ast.Inspect(af, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "time" && (x.Sel.Name == "Now" || x.Sel.Name == "Since" || x.Sel.Name == "Until") {
					t.Errorf("%s reads the clock", f)
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "float64" || id.Name == "float32") {
					t.Errorf("%s converts to %s", f, id.Name)
				}
			}
			return true
		})
	}
}
