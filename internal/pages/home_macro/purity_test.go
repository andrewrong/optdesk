package home_macro_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestImportsStayInsideTheRules: the member reads macro.ease through compute.ReadingFunc and
// imports neither the ease rule nor the macro page, no clock-free escape (os, sort, database).
func TestImportsStayInsideTheRules(t *testing.T) {
	t.Parallel()
	forbidden := []string{"internal/compute/ease", "internal/pages/macro_page", "database/sql", "os", "sort", "internal/jobs", "internal/notify"}
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
				if path == bad || strings.HasSuffix(path, "/"+bad) || strings.Contains(path, "/"+bad+"/") {
					t.Errorf("%s imports %s", f, path)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
