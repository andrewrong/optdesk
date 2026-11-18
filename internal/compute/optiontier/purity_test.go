package optiontier

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoFloatNoClockNoIO: the non-test files import no math, os, net/http, database/sql or time (nor
// strategy, optionevidence or bsm) and name no float64.
func TestNoFloatNoClockNoIO(t *testing.T) {
	t.Parallel()
	banned := []string{"math", "os", "net/http", "database/sql", "time",
		"github.com/andrewrong/optdesk/internal/strategy", "github.com/andrewrong/optdesk/internal/compute/optionevidence"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, b := range banned {
				if path == b || strings.HasPrefix(path, b+"/") || strings.Contains(path, "/bsm") {
					t.Errorf("%s imports %s", name, path)
				}
			}
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "float"+"64") {
			t.Errorf("%s names the float type", name)
		}
	}
}
