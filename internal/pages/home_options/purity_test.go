package home_options_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestImportsStayInsideTheRules: the member reads the ledger through store.OptionLedgerReader and compute.OptionLedger and
// imports neither the replay implementation nor the tracking page, no escape (os, sort, database).
func TestImportsStayInsideTheRules(t *testing.T) {
	t.Parallel()
	forbidden := []string{"internal/pages/option_tracking_page", "database/sql", "os", "sort", "internal/jobs", "internal/notify", "internal/compute/optionledger"}
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
