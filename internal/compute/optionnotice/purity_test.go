package optionnotice

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var (
	bannedImports = []string{"os", "net/http", "database/sql", "time"}
	bannedModules = []string{"internal/store", "internal/notify"}
)

func TestNoFloatNoClockNoIO(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob: %v %d", err, len(files))
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, b := range bannedImports {
				if path == b {
					t.Errorf("%s imports %s", f, path)
				}
			}
			for _, b := range bannedModules {
				if strings.HasSuffix(path, b) || strings.Contains(path, b+"/") {
					t.Errorf("%s imports %s", f, path)
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "float32" || x.Name == "float64" {
					t.Errorf("%s: identifier %s", fset.Position(x.Pos()), x.Name)
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "time" && x.Sel.Name == "Now" {
					t.Errorf("%s: time.Now", fset.Position(x.Pos()))
				}
			}
			return true
		})
	}
}
