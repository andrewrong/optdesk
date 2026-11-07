package contracts_test

import (
	"fmt"
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
		{"option_data_page.v1.ts", "./option_data_page.v1", "OptionDataPage"},
		{"indicator_detail.v1.ts", "./indicator_detail.v1", "IndicatorDetail"},
		{"option_result_page.v1.ts", "./option_result_page.v1", "OptionResultPage"},
		{"household_risk.v1.ts", "./household_risk.v1", "HouseholdRisk"},
		{"ledger_page.v1.ts", "./ledger_page.v1", "LedgerPage"},
		{"structure_detail.v3.ts", "./structure_detail.v3", "StructureDetail"},
		{"stock_option_page.v1.ts", "./stock_option_page.v1", "StockOptionPage"},
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

// TestGeneratedV3Types reads each v3 schema and its committed .ts: the banner, the exported top-level name, every
// property name, const and enum value of the schema, and no null. It does not regenerate (no node, no network).
func TestGeneratedV3Types(t *testing.T) {
	t.Parallel()
	pairs := []struct{ schema, file, name string }{
		{"page.market_summary.v3.json", "market_summary.v3.ts", "MarketSummaryV3"},
		{"page.options_summary.v3.json", "options_summary.v3.ts", "OptionsSummaryV3"},
	}
	nullWord := regexp.MustCompile(`\bnull\b`)
	for _, p := range pairs {
		t.Run(p.file, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(generatedDir, p.file))
			if err != nil {
				t.Fatal(err)
			}
			src := string(b)
			if line, _, _ := strings.Cut(src, "\n"); line != generatedBanner {
				t.Errorf("line 1 = %q", line)
			}
			if !regexp.MustCompile(`(?m)^export interface ` + p.name + ` `).MatchString(src) {
				t.Errorf("no export interface %s", p.name)
			}
			if nullWord.MatchString(src) {
				t.Error("null occurs")
			}
			// The names to find: the keys of every "properties" map, const and enum values of the schema and of
			// the defs of common.v2.json it reaches by $ref (the generator inlines only those).
			page := readRaw(t, schemasDir, p.schema)
			commonDefs := readRaw(t, schemasDir, "common.v2.json")["$defs"].(map[string]any)
			used := map[string]bool{}
			// prefix is the form of a ref that points into common.v2.json from the visited document.
			var collect func(v any, prefix string)
			collect = func(v any, prefix string) {
				walk(v, func(k string, x any) {
					ref, _ := x.(string)
					if def, ok := strings.CutPrefix(ref, prefix); k == "$ref" && ok && !used[def] {
						used[def] = true
						collect(commonDefs[def], "#/$defs/")
					}
				})
			}
			collect(page, "common.v2.json#/$defs/")
			schemas := []any{page}
			for d := range used {
				schemas = append(schemas, commonDefs[d])
			}
			want := map[string]bool{}
			for _, doc := range schemas {
				walk(doc, func(k string, v any) {
					switch k {
					case "properties":
						if m, ok := v.(map[string]any); ok {
							for n := range m {
								want[n] = true
							}
						}
					case "const":
						want[fmt.Sprint(v)] = true
					case "enum":
						if l, ok := v.([]any); ok {
							for _, e := range l {
								want[fmt.Sprint(e)] = true
							}
						}
					}
				})
			}
			for n := range want {
				if !strings.Contains(src, n) {
					t.Errorf("%q of the schema does not occur in %s", n, p.file)
				}
			}
		})
	}
}
