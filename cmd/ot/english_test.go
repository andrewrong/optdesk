package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// allowedCJK lists the frozen Chinese values a Go file of the guarded trees must hold. Empty today: the
// frozen values of Left out in docs/design/ui-english.md live in schemas and web types. An entry is added
// with a comment naming its line in Left out.
var allowedCJK = []string{}

// guardedTrees are the Go trees the English guard walks, from the repository root.
var guardedTrees = []string{"internal/pages", "internal/compute/pagefig", "cmd/ot"}

var cjkRun = regexp.MustCompile(`[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]+`)

// TestGoTreesAreEnglish fails on any CJK character in a non-test .go file of the guarded trees that is not
// part of an allowedCJK entry.
func TestGoTreesAreEnglish(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	for _, tree := range guardedTrees {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(tree)), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(b)
			for _, ok := range allowedCJK {
				text = strings.ReplaceAll(text, ok, "")
			}
			for i, line := range strings.Split(text, "\n") {
				if m := cjkRun.FindString(line); m != "" {
					t.Errorf("%s: line %d of the text without allowed values holds %q", filepath.ToSlash(path), i+1, m)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", tree, err)
		}
	}
}
