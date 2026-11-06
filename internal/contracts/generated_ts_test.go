package contracts_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const generatedDir = "../../web/src/generated"

const generatedBanner = "// Code generated from contracts/, DO NOT EDIT."

// TestGeneratedSummaryTypes guards the banner, the exported top-level name and the
// re-exports of the generated v2 summary types. It does not detect drift in the bodies.
func TestGeneratedSummaryTypes(t *testing.T) {
	t.Parallel()
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(generatedDir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	firstLine := func(s string) string { l, _, _ := strings.Cut(s, "\n"); return l }

	files := []struct{ file, module, name string }{
		{"market_summary.v2.ts", "./market_summary.v2", "MarketSummary"},
		{"options_summary.v2.ts", "./options_summary.v2", "OptionsSummary"},
	}
	index := read("index.ts")
	if got := firstLine(index); got != generatedBanner {
		t.Errorf("index.ts line 1 = %q", got)
	}
	if strings.Contains(index, "export {}") {
		t.Error("index.ts still holds the stub export {}")
	}
	var exports []string
	for _, l := range strings.Split(index, "\n") {
		if strings.HasPrefix(l, "export ") {
			exports = append(exports, l)
		}
	}
	if len(exports) != len(files) {
		t.Errorf("index.ts has %d export lines %q, want %d", len(exports), exports, len(files))
	}
	for _, f := range files {
		src := read(f.file)
		if got := firstLine(src); got != generatedBanner {
			t.Errorf("%s line 1 = %q", f.file, got)
		}
		if !regexp.MustCompile(`(?m)^export interface ` + f.name + ` `).MatchString(src) {
			t.Errorf("%s does not export interface %s", f.file, f.name)
		}
		want := "export type { " + f.name + " } from '" + f.module + "';"
		found := 0
		for _, l := range exports {
			if l == want {
				found++
			}
		}
		if found != 1 {
			t.Errorf("index.ts has %d lines %q, want 1", found, want)
		}
	}
}
