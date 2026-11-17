package data_health_page

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSourceStaysPure: a non-test file imports no database, file, network or store implementation, and reads no clock.
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
	forbidden := []string{"database/sql", "os", "net/http", "internal/store/ops", "github.com/jackc/pgx"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		checked++
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			path, _ := strconv.Unquote(im.Path.Value)
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") || strings.HasSuffix(path, "/"+bad) {
					t.Errorf("%s imports %s", f, path)
				}
			}
		}
		ast.Inspect(af, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "time" && (sel.Sel.Name == "Now" || sel.Sel.Name == "Since" || sel.Sel.Name == "Until") {
				t.Errorf("%s reads the clock", f)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
