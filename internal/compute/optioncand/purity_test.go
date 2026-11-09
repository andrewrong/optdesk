package optioncand

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
	bannedImports = []string{"os", "net/http", "database/sql"}
	bannedModules = []string{"internal/store", "internal/notify", "internal/jobs"}
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
		timeName := checkImports(t, f, file)
		checkIdents(t, fset, file, timeName)
	}
}

// checkImports fails on a banned import and returns the local name of "time", or "".
func checkImports(t *testing.T, f string, file *ast.File) string {
	t.Helper()
	timeName := ""
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
		if path == "time" {
			timeName = "time"
			if imp.Name != nil {
				timeName = imp.Name.Name
			}
		}
	}
	return timeName
}

// checkIdents fails on float32, float64 and on time.Now or time.Sleep.
func checkIdents(t *testing.T, fset *token.FileSet, file *ast.File, timeName string) {
	t.Helper()
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if x.Name == "float32" || x.Name == "float64" {
				t.Errorf("%s: identifier %s", fset.Position(x.Pos()), x.Name)
			}
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			if ok && timeName != "" && id.Name == timeName && (x.Sel.Name == "Now" || x.Sel.Name == "Sleep") {
				t.Errorf("%s: time.%s", fset.Position(x.Pos()), x.Sel.Name)
			}
		}
		return true
	})
}
