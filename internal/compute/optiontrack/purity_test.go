package optiontrack

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNoFloatNoClockNoIO(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob: %v %v", files, err)
	}
	banned := map[string]bool{
		"os": true, "net/http": true, "database/sql": true,
	}
	bannedPrefix := []string{
		"github.com/andrewrong/optdesk/internal/store", "github.com/andrewrong/optdesk/internal/notify",
		"github.com/andrewrong/optdesk/internal/jobs", "github.com/andrewrong/optdesk/internal/pages",
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			bad := banned[path]
			for _, pre := range bannedPrefix {
				bad = bad || path == pre || strings.HasPrefix(path, pre+"/")
			}
			if bad {
				t.Errorf("%s imports %s", f, path)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "float32" || x.Name == "float64" {
					t.Errorf("%s: %s", f, x.Name)
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "time" && (x.Sel.Name == "Now" || x.Sel.Name == "Sleep") {
					t.Errorf("%s: time.%s", f, x.Sel.Name)
				}
			}
			return true
		})
	}
}
