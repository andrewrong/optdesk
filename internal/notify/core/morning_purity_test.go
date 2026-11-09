package core_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoAccountTypesNoTelegramLeak: the code that can reach a Sender names no account, candidate
// or notice type and imports neither pages, jobs nor the optionnotice rule (Decision 1).
func TestNoAccountTypesNoTelegramLeak(t *testing.T) {
	t.Parallel()
	banned := map[string]bool{"Position": true, "Candidate": true, "Cushion": true, "ExcessLiquidity": true,
		"AccountReader": true, "AccountValue": true, "OptionLedger": true, "NoticeResult": true, "CashByDate": true}
	root := filepath.Join("..") // internal/notify
	var files []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("walk: %v, %d files", err, len(files))
	}
	fset := token.NewFileSet()
	for _, p := range files {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if strings.Contains(path, "internal/pages") || strings.Contains(path, "internal/jobs") || strings.Contains(path, "optionnotice") {
				t.Errorf("%s imports %s", p, path)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && banned[id.Name] {
				t.Errorf("%s: identifier %s at %s", p, id.Name, fset.Position(id.Pos()))
			}
			return true
		})
	}
}
