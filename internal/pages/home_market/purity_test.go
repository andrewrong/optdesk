package home_market_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestImportsStayPure: the member builder reads through its Config only. It imports no other
// page (not market_page), no notifier, no job, no database and no file access, and reads no clock.
func TestImportsStayPure(t *testing.T) {
	t.Parallel()
	forbidden := []string{"internal/pages/market_page", "database/sql", "os", "internal/notify", "internal/jobs"}
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
		if strings.Contains(string(src), "time.Now") {
			t.Errorf("%s reads the clock", f)
		}
		for _, bad := range []string{"math.Sqrt", "math.Log"} {
			if strings.Contains(string(src), bad) {
				t.Errorf("%s computes with %s: the page shows stored values only", f, bad)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
