package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	homePageV3     = "page.home_page.v3.json"
	heldV3         = "page.held_companies_summary.v3.json"
	optionsV4      = "page.options_summary.v4.json"
	homeV3Pattern  = "^[A-Za-z0-9_-]{1,64}$"
	datePatternRaw = "^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$"
)

// v3Overrides gives the patterns of the new files a value.
var v3Overrides = map[string]string{homeV3Pattern: "id_1", datePatternRaw: "2026-10-28", "^/": "/macro"}

func minimalV3(t *testing.T, name string) map[string]any {
	t.Helper()
	inst, ok := minimal(t, schemasDir, name, readRaw(t, schemasDir, name), v3Overrides).(map[string]any)
	if !ok {
		t.Fatalf("%s: not an object schema", name)
	}
	return inst
}

// hasV3Leaf reports whether validating inst against s fails at location with keyword.
func hasV3Leaf(t *testing.T, s *jsonschema.Schema, inst any, location, keyword string) bool {
	t.Helper()
	ve, ok := s.Validate(toJSONValue(t, inst)).(*jsonschema.ValidationError)
	if !ok {
		return false
	}
	var ls []leaf
	leaves(ve, &ls)
	for _, l := range ls {
		if l.location == location && l.keyword == keyword {
			return true
		}
	}
	return false
}

func TestHomePageV3Valid(t *testing.T) {
	t.Parallel()
	s := compileURL(t, schemasDir, schemaHost+homePageV3)
	inst := minimalV3(t, homePageV3)
	if err := s.Validate(toJSONValue(t, inst)); err != nil {
		t.Fatalf("minimal instance is invalid: %v", err)
	}
	if _, ok := inst["block_links"]; ok {
		t.Error("minimal instance holds block_links")
	}
	if _, ok := inst["overview"].(map[string]any); !ok {
		t.Error("minimal instance holds no overview")
	}
}

func TestHomePageV3Rejects(t *testing.T) {
	t.Parallel()
	s := compileURL(t, schemasDir, schemaHost+homePageV3)
	cases := []struct {
		name, location, keyword string
		edit                    func(inst map[string]any)
	}{
		{"block_links", "", "additionalProperties", func(i map[string]any) { i["block_links"] = []any{} }},
		{"schema_version 2", "/schema_version", "const", func(i map[string]any) { i["schema_version"] = 2 }},
		{"no stale_after", "", "required", func(i map[string]any) { delete(i, "stale_after") }},
		{"no overview", "", "required", func(i map[string]any) { delete(i, "overview") }},
		{"empty overview items", "/overview/items", "minItems", func(i map[string]any) { i["overview"].(map[string]any)["items"] = []any{} }},
		{"href without slash", "/overview/items/0/href", "pattern", func(i map[string]any) {
			i["overview"].(map[string]any)["items"].([]any)[0].(map[string]any)["href"] = "macro"
		}},
		{"21 gaps", "/overview/gaps", "maxItems", func(i map[string]any) {
			g := make([]any, 21)
			for k := range g {
				g[k] = map[string]any{"card": "a", "text": "b"}
			}
			i["overview"].(map[string]any)["gaps"] = g
		}},
		{"companies v2", "/companies/schema_version", "const", func(i map[string]any) { i["companies"].(map[string]any)["schema_version"] = 2 }},
		{"options v3", "/options/schema_version", "const", func(i map[string]any) { i["options"].(map[string]any)["schema_version"] = 3 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			inst := minimalV3(t, homePageV3)
			inst["overview"] = map[string]any{
				"items": []any{map[string]any{"label": "a", "text": "b", "href": "/macro"}},
				"gaps":  []any{},
			}
			c.edit(inst)
			if !hasV3Leaf(t, s, inst, c.location, c.keyword) {
				t.Errorf("want a failure at %q (%s)", c.location, c.keyword)
			}
		})
	}
}

func TestHomePageV3RefsMembers(t *testing.T) {
	t.Parallel()
	doc := readRaw(t, schemasDir, homePageV3)
	props := doc["properties"].(map[string]any)
	for member, file := range map[string]string{"companies": heldV3, "options": optionsV4} {
		if got := props[member].(map[string]any)["$ref"]; got != file {
			t.Errorf("%s refs %v, want %s", member, got, file)
		}
	}
}

// TestNewV3FilesConventions applies the rules of the v3 line to the three new files.
func TestNewV3FilesConventions(t *testing.T) {
	t.Parallel()
	common := readRaw(t, schemasDir, "common.v2.json")["$defs"].(map[string]any)
	for _, name := range []string{homePageV3, heldV3, optionsV4} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(schemasDir, name))
			if err != nil {
				t.Fatal(err)
			}
			doc := readRaw(t, schemasDir, name)
			r := v3Rules{t: t, name: name, common: common, localGap: name == optionsV4}
			if name != optionsV4 { // header expects schema_version 3; v4 is checked in TestOptionsV4
				r.header(doc)
			} else if doc["$id"] != schemaHost+name {
				t.Errorf("$id = %v", doc["$id"])
			}
			if strings.Contains(string(b), "common.v1.json") {
				t.Error("common.v1.json occurs")
			}
			walk(doc, r.node)
		})
	}
}
