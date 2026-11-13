package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// allowedCJK lists the frozen Chinese values a Go file of the guarded trees may hold. They are the four sentences
// of Decision 9 of docs/design/market-state-tech.md, written in internal/pages/home_market/say.go. The owner's
// word is 2026-11-13, "3. 中文"; it amends docs/design/ui-english.md decision 1 for these four sentences and
// for nothing else. A further entry needs its own owner word and its own design entry.
var allowedCJK = []string{
	"科技近 1 个月、3 个月都跑赢大盘（试行，不是预测）。",
	"科技与大盘大体同步（试行，不是预测）。",
	"科技近 1 个月、3 个月都跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。",
	"科技近 3 个月明显跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。",
}

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
