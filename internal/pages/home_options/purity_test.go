package home_options_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestImportsStayInsideTheRules: the member reads the ledger through store.OptionLedgerReader and compute.OptionLedger and
// imports neither the replay implementation nor the tracking page, no big number (the builder
// computes no figure) and no escape (os, sort, database).
func TestImportsStayInsideTheRules(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"math/big", "internal/pages/option_tracking_page", "database/sql", "os", "sort",
		"internal/jobs", "internal/notify", "internal/compute/optionledger",
	}
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

// TestNoFloat: the builder copies decimal text; it declares no float.
func TestNoFloat(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && (id.Name == "float32" || id.Name == "float64") {
				t.Errorf("%s declares %s", f, id.Name)
			}
			return true
		})
	}
}
