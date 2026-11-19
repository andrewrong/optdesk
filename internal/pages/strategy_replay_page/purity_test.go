package strategy_replay_page

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSourceStaysPure: the package imports the standard library and contracts only, no store, strategy, compute,
// database, network, clock or decimal package.
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
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
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "os" || p == "time" || p == "math" || p == "net/http" || p == "database/sql" {
				t.Errorf("%s imports %s", f, p)
				continue
			}
			if strings.Contains(strings.SplitN(p, "/", 2)[0], ".") && p != "github.com/andrewrong/optdesk/internal/contracts" {
				t.Errorf("%s imports %s", f, p)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
