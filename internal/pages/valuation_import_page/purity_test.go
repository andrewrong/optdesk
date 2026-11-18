package valuation_import_page

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func nonTestFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatal("no source file checked")
	}
	return out
}

// TestSourceStaysPure: the package imports no database, file, network, clock, store implementation, decimal or
// math package.
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
	forbidden := []string{"database/sql", "os", "net/http", "time", "math", "internal/clock",
		"internal/store/valuationpg", "internal/store/underlyingpg", "github.com/jackc/pgx", "internal/compute/valueimport"}
	for _, f := range nonTestFiles(t) {
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if strings.Contains(p, "decimal") {
				t.Errorf("%s imports %s", f, p)
			}
			for _, bad := range forbidden {
				if p == bad || strings.HasPrefix(p, bad+"/") || strings.HasSuffix(p, "/"+bad) {
					t.Errorf("%s imports %s", f, p)
				}
			}
		}
	}
}
