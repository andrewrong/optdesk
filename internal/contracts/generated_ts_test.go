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

// generatedTypeFiles lists the .ts files of generatedDir by base name.
func generatedTypeFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(generatedDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".ts") {
			names = append(names, e.Name())
		}
	}
	return names
}

// TestGeneratedTypes reads the directory: every .ts file has the banner, a schema page.<name>.json and
// an exported interface named by the title of that schema. It does not detect drift in the bodies.
func TestGeneratedTypes(t *testing.T) {
	t.Parallel()
	files := generatedTypeFiles(t)
	if len(files) == 0 {
		t.Fatal("no .ts file in " + generatedDir)
	}
	for _, f := range files {
		if f == "index.ts" {
			t.Fatal("index.ts exists: a consumer imports the file it needs")
		}
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(generatedDir, f))
			if err != nil {
				t.Fatal(err)
			}
			src := string(b)
			if line, _, _ := strings.Cut(src, "\n"); line != generatedBanner {
				t.Errorf("line 1 = %q", line)
			}
			schema := "page." + strings.TrimSuffix(f, ".ts") + ".json"
			if _, err := os.Stat(filepath.Join(schemasDir, schema)); err != nil {
				t.Fatalf("no schema for %s: %v", f, err)
			}
			title, _ := readRaw(t, schemasDir, schema)["title"].(string)
			if title == "" {
				t.Fatalf("%s has no title", schema)
			}
			if !regexp.MustCompile(`(?m)^export interface ` + regexp.QuoteMeta(title) + ` `).MatchString(src) {
				t.Errorf("%s does not export interface %s", f, title)
			}
		})
	}
}

type v3Pair struct{ schema, file, name string }

// v3Pairs lists the *.v3.ts files of the directory whose schema is not carried, with the schema title.
func v3Pairs(t *testing.T) []v3Pair {
	t.Helper()
	manifest := readManifest(t)
	var pairs []v3Pair
	for _, f := range generatedTypeFiles(t) {
		schema := "page." + strings.TrimSuffix(f, ".ts") + ".json"
		// macro_rates.v3 is version 3 of the v2 line (its points hold null), not of the v3 line.
		if !strings.HasSuffix(f, ".v3.ts") || f == "macro_rates.v3.ts" {
			continue
		}
		if _, carried := manifest[schema]; carried {
			continue
		}
		title, _ := readRaw(t, schemasDir, schema)["title"].(string)
		pairs = append(pairs, v3Pair{schema, f, title})
	}
	return pairs
}

var nullWord = regexp.MustCompile(`\bnull\b`)

// refsV2Page reports whether the schema holds a $ref to a page file of the v2 line (`page.<name>.v2.json`), whose
// nullable members the generator inlines. A ref to common.v2.json does not count: every v3 file has one.
func refsV2Page(doc any) bool {
	found := false
	walk(doc, func(k string, v any) {
		ref, _ := v.(string)
		file, _, _ := strings.Cut(ref, "#")
		if k == "$ref" && strings.HasPrefix(file, "page.") && strings.HasSuffix(file, ".v2.json") {
			found = true
		}
	})
	return found
}

// v3NullProblem returns the message of the null check for the source of a v3 file, or "" when it holds. The check
// is skipped for a schema that refs a v2 page file; no other check depends on it.
func v3NullProblem(src string, schema map[string]any) string {
	if refsV2Page(schema) || !nullWord.MatchString(src) {
		return ""
	}
	return "null occurs"
}

// TestGeneratedV3NullRule: a source holding null fails unless its schema refs a v2 page file.
func TestGeneratedV3NullRule(t *testing.T) {
	t.Parallel()
	withNull := "export type Tone = (\"up\") | null;\n"
	plain := map[string]any{"properties": map[string]any{"a": map[string]any{"$ref": "common.v2.json#/$defs/Id"}}}
	member := map[string]any{"properties": map[string]any{"m": map[string]any{"$ref": "page.household_summary.v2.json"}}}
	for _, c := range []struct {
		name   string
		src    string
		schema map[string]any
		want   string
	}{
		{"null without a v2 page ref", withNull, plain, "null occurs"},
		{"null with a v2 page ref", withNull, member, ""},
		{"no null with a v2 page ref", "export type A = string;\n", member, ""},
		{"no null without a ref", "export type A = string;\n", plain, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := v3NullProblem(c.src, c.schema); got != c.want {
				t.Errorf("problem = %q, want %q", got, c.want)
			}
		})
	}
}

// TestGeneratedV3Types reads each v3 file of the directory whose schema is written here (not carried): the banner,
// the exported top-level name, every property name, const and enum value of the schema, and no null. It does not
// regenerate (no node, no network).
func TestGeneratedV3Types(t *testing.T) {
	t.Parallel()
	pairs := v3Pairs(t)
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
			page := readRaw(t, schemasDir, p.schema)
			if problem := v3NullProblem(src, page); problem != "" {
				t.Error(problem)
			}
			// The names to find: the keys of every "properties" map, const and enum values of the schema and of
			// the defs of common.v2.json it reaches by $ref (the generator inlines only those).
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
