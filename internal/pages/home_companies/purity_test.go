package home_companies_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPurity: the builder copies stored text; it uses no float, no sort, no notifier, no job, no
// database, no file access and no implementation page.
func TestPurity(t *testing.T) {
	t.Parallel()
	forbiddenImports := []string{"math/big", "sort", "database/sql", "os", "internal/notify", "internal/jobs", "internal/pages/companies_page"}
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
			for _, bad := range forbiddenImports {
				if path == bad || strings.HasSuffix(path, "/"+bad) {
					t.Errorf("%s imports %s", f, path)
				}
			}
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []string{"float32", "float64", "time.Now"} {
			if strings.Contains(string(src), w) {
				t.Errorf("%s mentions %s", f, w)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
