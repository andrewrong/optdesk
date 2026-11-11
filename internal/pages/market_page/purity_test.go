package market_page

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSourceStaysPure: the page renders stored readings and computes nothing. A non-test
// file imports no bar store, job, figure computation, database or file access, and names
// no square root or logarithm (the realised volatility is computed by compute_figures).
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
	forbidden := []string{"internal/store/bar", "internal/jobs", "internal/compute/figures", "database/sql", "os"}
	banned := []string{"math.Sqrt", "math.Log"}
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
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			path, _ := strconv.Unquote(im.Path.Value)
			for _, bad := range forbidden {
				if path == bad || strings.HasSuffix(path, "/"+bad) {
					t.Errorf("%s imports %s", f, path)
				}
			}
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range banned {
			if strings.Contains(string(src), b) {
				t.Errorf("%s computes with %s", f, b)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
