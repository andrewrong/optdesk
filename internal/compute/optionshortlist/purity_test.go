package optionshortlist

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNoClockNoIO(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob: %v %d", err, len(files))
	}
	banned := []string{"time", "os", "net", "database/sql", "math/rand"}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, b := range banned {
				if p == b || strings.HasPrefix(p, b+"/") {
					t.Errorf("%s imports %s", f, p)
				}
			}
		}
	}
}
