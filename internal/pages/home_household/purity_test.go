package home_household_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoFloatNoClockNoSort: the builder copies decimal text; it uses no float, no big number, no
// math, no sort, no clock reading, no notifier, no job, no database and no file access.
func TestNoFloatNoClockNoSort(t *testing.T) {
	t.Parallel()
	forbiddenImports := []string{"math", "sort", "database/sql", "os", "internal/notify", "internal/jobs"}
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
				if path == bad || strings.HasPrefix(path, bad+"/") || strings.Contains(path, "/"+bad) {
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
