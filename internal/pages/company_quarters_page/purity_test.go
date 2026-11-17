package company_quarters_page

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// TestPageHasNoArithmetic: format.go and builder.go import no math or decimal package and use no big. or
// decimal. selector and no float64 conversion; they import no compute implementation.
func TestPageHasNoArithmetic(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"format.go", "builder.go"} {
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "math" || strings.HasPrefix(p, "math/") || strings.Contains(p, "decimal") || strings.HasSuffix(p, "internal/compute/companydata") {
				t.Errorf("%s imports %s", f, p)
			}
		}
		ast.Inspect(af, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && (id.Name == "big" || id.Name == "decimal" || id.Name == "math") {
					t.Errorf("%s uses %s.%s", f, id.Name, x.Sel.Name)
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

// TestFreeCashCopiedFromCalculator: the free cash flow cell is the Calculator's FreeCash, not a difference.
func TestFreeCashCopiedFromCalculator(t *testing.T) {
	t.Parallel()
	qs := quarters()
	qs[0].OperatingCashFlow, qs[0].Capex = val("5"), val("3")
	qs[0].FreeCash = val("777")
	i, _ := in("US:AAPL", []underlying.Row{aapl}, qs)
	m := build(t, i)
	c := m["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)["free_cash"].(map[string]any)
	if c["text"] != "777" {
		t.Fatalf("free cash = %v", c)
	}
}
