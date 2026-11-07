package contracts_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaHost = "https://optdesk.invalid/schemas/"

// pageV2Files names the files of the v2 line: the five summaries and home_page.
// Files that do not exist yet are skipped by the tests and by the carry-over test.
var pageV2Files = []string{
	"page.held_companies_summary.v2.json",
	"page.home_page.v2.json",
	"page.household_summary.v2.json",
	"page.macro_summary.v2.json",
	"page.market_summary.v2.json",
	"page.options_summary.v2.json",
}

// dirLoader resolves https://optdesk.invalid/schemas/<file> to a file of dir,
// then of contracts/schemas.
type dirLoader struct{ dir string }

func (l dirLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(filepath.Join(l.dir, file))
	if err != nil {
		b, err = os.ReadFile(filepath.Join(schemasDir, file))
		if err != nil {
			return nil, err
		}
	}
	return jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
}

func compileURL(t *testing.T, dir, url string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(dirLoader{dir})
	s, err := c.Compile(url)
	if err != nil {
		t.Fatalf("%s: %v", url, err)
	}
	return s
}

// compilePageV2 compiles contracts/schemas/<name>.json with format assertion.
func compilePageV2(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	return compileURL(t, schemasDir, schemaHost+name)
}

func readRaw(t *testing.T, dir, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return m
}

// follow resolves a $ref of a raw schema of file `name` in dir. It returns the
// target and the file the target belongs to.
func follow(t *testing.T, dir, name, ref string) (map[string]any, string) {
	t.Helper()
	file, frag, _ := strings.Cut(ref, "#")
	if file == "" {
		file = name
	}
	if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
		dir = schemasDir
	}
	var cur any = readRaw(t, dir, file)
	for _, p := range strings.Split(strings.TrimPrefix(frag, "/"), "/") {
		if p == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("bad $ref %q", ref)
		}
		cur = m[p]
	}
	m, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("$ref %q does not resolve", ref)
	}
	return m, file
}

// firstBranch returns the first branch of the anyOf or the oneOf of a raw schema.
func firstBranch(s map[string]any) (map[string]any, bool) {
	for _, k := range []string{"anyOf", "oneOf"} {
		if a, ok := s[k].([]any); ok && len(a) > 0 {
			return a[0].(map[string]any), true
		}
	}
	return nil, false
}

// minimal builds the smallest valid value of a raw schema from its required lists.
func minimal(t *testing.T, dir, file string, s map[string]any, overrides map[string]string) any {
	t.Helper()
	if ref, ok := s["$ref"].(string); ok {
		target, f := follow(t, dir, file, ref)
		return minimal(t, dir, f, target, overrides)
	}
	if c, ok := s["const"]; ok {
		return c
	}
	if e, ok := s["enum"].([]any); ok && len(e) > 0 {
		return e[0]
	}
	if b, ok := firstBranch(s); ok {
		return minimal(t, dir, file, b, overrides)
	}
	num := func(key string) float64 {
		v, _ := s[key].(float64)
		return v
	}
	switch s["type"] {
	case "object":
		out := map[string]any{}
		props, _ := s["properties"].(map[string]any)
		req, _ := s["required"].([]any)
		for _, r := range req {
			name := r.(string)
			ps, ok := props[name].(map[string]any)
			if !ok {
				t.Fatalf("required property %q has no schema", name)
			}
			out[name] = minimal(t, dir, file, ps, overrides)
		}
		return out
	case "array":
		n := int(num("minItems"))
		out := make([]any, 0, n)
		if items, ok := s["items"].(map[string]any); ok {
			for range n {
				out = append(out, minimal(t, dir, file, items, overrides))
			}
		}
		return out
	case "string":
		if p, ok := s["pattern"].(string); ok {
			v, ok := overrides[p]
			if !ok {
				t.Fatalf("no override for pattern %q", p)
			}
			return v
		}
		if s["format"] == "date-time" {
			return "2026-01-02T03:04:05+08:00"
		}
		return strings.Repeat("x", max(1, int(num("minLength"))))
	case "integer", "number":
		return num("minimum")
	case "boolean":
		return false
	case "null":
		return nil
	}
	t.Fatalf("cannot build a value for %v", s)
	return nil
}

func checkPageV2In(t *testing.T, dir, name string, overrides map[string]string) {
	t.Helper()
	s := compileURL(t, dir, schemaHost+name)
	build := func() map[string]any {
		raw := readRaw(t, dir, name)
		inst, ok := minimal(t, dir, name, raw, overrides).(map[string]any)
		if !ok {
			t.Fatalf("%s: not an object schema", name)
		}
		return inst
	}
	// The instance goes through JSON so the validator sees plain JSON types.
	roundTrip := func(m map[string]any) any {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	if err := s.Validate(roundTrip(build())); err != nil {
		t.Fatalf("minimal instance of %s is invalid: %v", name, err)
	}

	bad := build()
	bad["schema_version"] = 3
	err := s.Validate(roundTrip(bad))
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		t.Fatalf("wrong schema_version: want a validation error, got %v", err)
	}
	var ls []leaf
	leaves(ve, &ls)
	found := false
	for _, l := range ls {
		if l.location == "/schema_version" && l.keyword == "const" {
			found = true
		}
	}
	if !found {
		t.Errorf("wrong schema_version: want /schema_version const, got %v", ls)
	}

	noStale := build()
	delete(noStale, "stale_after")
	if err := s.Validate(roundTrip(noStale)); err != nil {
		t.Errorf("instance without stale_after is invalid: %v", err)
	}
}

// checkPageV2 builds a minimal instance of contracts/schemas/<name>.json and
// asserts: valid; wrong schema_version fails at /schema_version with const; valid
// without stale_after. overrides maps a string pattern to a value that matches it.
func checkPageV2(t *testing.T, name string, overrides map[string]string) {
	t.Helper()
	checkPageV2In(t, schemasDir, name, overrides)
}

func TestCheckPageV2Helper(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const page = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://optdesk.invalid/schemas/page.tiny.v2.json",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "run_id", "generated_at", "status"],
  "properties": {
    "schema_version": {"const": 2, "type": "integer"},
    "run_id": {"$ref": "common.v1.json#/$defs/Id"},
    "generated_at": {"$ref": "common.v1.json#/$defs/Timestamp"},
    "status": {"$ref": "common.v1.json#/$defs/RunStatus"},
    "stale_after": {"$ref": "common.v1.json#/$defs/Timestamp"}
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "page.tiny.v2.json"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	checkPageV2In(t, dir, "page.tiny.v2.json", map[string]string{"^[A-Za-z0-9_-]{1,64}$": "run_1"})
}

// Each file of pageV2Files that exists passes the three outcomes of checkPageV2.
func TestPageV2FilesCheck(t *testing.T) {
	t.Parallel()
	overrides := map[string]string{"^[A-Za-z0-9_-]{1,64}$": "id_1"}
	for _, name := range pageV2Files {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := os.Stat(filepath.Join(schemasDir, name)); os.IsNotExist(err) {
				t.Skip("file does not exist yet")
			}
			checkPageV2(t, name, overrides)
		})
	}
}

func TestCommonDefs(t *testing.T) {
	t.Parallel()
	raw := readRaw(t, schemasDir, "common.v1.json")
	if raw["$id"] != schemaHost+"common.v1.json" {
		t.Errorf("$id = %v", raw["$id"])
	}
	var names []string
	for n := range raw["$defs"].(map[string]any) {
		names = append(names, n)
	}
	sort.Strings(names)
	want := []string{"Gap", "Id", "PercentFigure", "RunStatus", "SharePercentFigure", "SourceTime", "StaleLabel", "Timestamp", "UsdFigure", "UsdSignedFigure"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("$defs = %v, want %v", names, want)
	}

	rows := []struct{ def, valid, invalid string }{
		{"Id", `"run_1-A"`, `"has space"`},
		{"Id", `"x"`, `""`},
		{"Timestamp", `"2026-01-02T03:04:05+00:00"`, `"yesterday"`},
		{"Timestamp", `"2026-01-02T03:04:05+08:00"`, `"2026-13-02T03:04:05+08:00"`},
		{"Timestamp", `"2026-01-02T03:04:05Z"`, `20260102`},
		{"RunStatus", `"partial"`, `"done"`},
		{"SourceTime", `{"source":"s1","data_time":"2026-01-02T03:04:05+08:00"}`, `{"source":"bad id","data_time":"2026-01-02T03:04:05+08:00"}`},
		{
			"StaleLabel",
			`{"data_time":"2026-01-02T03:04:05+08:00","stale_after":"2026-01-02T04:04:05+08:00","text":"old"}`,
			`{"data_time":"2026-01-02T03:04:05+08:00","text":"old"}`,
		},
		{"Gap", `{"reason":"stale_input","text":"late"}`, `{"reason":"bogus","text":"late"}`},
		{"UsdFigure", `{"value":1.25,"text":"$1","unit":"USD"}`, `{"value":-1,"text":"$1","unit":"USD"}`},
		{"PercentFigure", `{"value":2.1,"text":"+2.1%","unit":"percent"}`, `{"value":10001,"text":"x","unit":"percent"}`},
		{"SharePercentFigure", `{"value":50,"text":"50%","unit":"share_percent"}`, `{"value":101,"text":"x","unit":"share_percent"}`},
		{"UsdSignedFigure", `{"value":-5.5,"text":"−$5","unit":"usd_signed"}`, `{"value":1,"text":"x","unit":"USD"}`},
	}
	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.def] = true
		t.Run(r.def+"/"+r.valid, func(t *testing.T) {
			t.Parallel()
			s := compileURL(t, schemasDir, schemaHost+"common.v1.json#/$defs/"+r.def)
			for _, c := range []struct {
				text string
				ok   bool
			}{{r.valid, true}, {r.invalid, false}} {
				inst, err := jsonschema.UnmarshalJSON(strings.NewReader(c.text))
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Validate(inst); (err == nil) != c.ok {
					t.Errorf("%s: valid = %v, want %v (%v)", c.text, err == nil, c.ok, err)
				}
			}
		})
	}
	for _, n := range want {
		if !covered[n] {
			t.Errorf("def %s has no row", n)
		}
	}
}

func walk(v any, f func(key string, val any)) {
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			f(k, c)
			walk(c, f)
		}
	case []any:
		for _, c := range x {
			walk(c, f)
		}
	}
}

func TestPageV2Conventions(t *testing.T) {
	t.Parallel()
	for _, name := range pageV2Files {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(schemasDir, name))
			if os.IsNotExist(err) {
				t.Skip("file does not exist yet")
			}
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Fatal(err)
			}
			if doc["$id"] != schemaHost+name {
				t.Errorf("$id = %v", doc["$id"])
			}
			props, _ := doc["properties"].(map[string]any)
			sv, _ := props["schema_version"].(map[string]any)
			if sv["const"] != float64(2) || sv["type"] != "integer" {
				t.Errorf("schema_version = %v, want integer const 2", sv)
			}
			if _, ok := props["stale_after"]; !ok {
				t.Error("stale_after is not in properties")
			}
			req, _ := doc["required"].([]any)
			for _, r := range req {
				if r == "stale_after" {
					t.Error("stale_after is required")
				}
			}
			defs, _ := doc["$defs"].(map[string]any)
			for n := range defs {
				if strings.Contains(n, "__") {
					t.Errorf("def name %q contains __", n)
				}
			}
			if strings.Contains(string(b), "ROUND_HALF_UP") {
				t.Error("ROUND_HALF_UP occurs")
			}
			walk(doc, func(k string, v any) {
				if k == "x-timezone" && v != "owner" {
					t.Errorf("x-timezone = %v, want owner", v)
				}
			})
			compilePageV2(t, name)
		})
	}
}
