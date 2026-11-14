package market_state_page

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSourceStaysPure: the page copies figures and computes none. A non-test file imports no
// figure computation, job, database or file access.
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
	forbidden := []string{"internal/compute/marketstate", "internal/compute/liquidity", "internal/compute/figures",
		"internal/jobs", "database/sql", "os"}
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
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
