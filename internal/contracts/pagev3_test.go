package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// pageV3Files names the files of the v3 line. A file that does not exist is skipped by the tests.
var pageV3Files = []string{
	"page.market_summary.v3.json",
	"page.options_summary.v3.json",
}

var idOverride = map[string]string{"^[A-Za-z0-9_-]{1,64}$": "id_1"}

// toJSONValue sends a Go value through JSON so the validator sees plain JSON types.
func toJSONValue(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// checkPageV3 builds the minimal instance of contracts/schemas/<name> and asserts: it is valid; the same instance
// with schema_version 2 fails at /schema_version with const; it without stale_after fails at the object with required.
func checkPageV3(t *testing.T, name string, overrides map[string]string) {
	t.Helper()
	s := compileURL(t, schemasDir, schemaHost+name)
	build := func() map[string]any {
		inst, ok := minimal(t, schemasDir, name, readRaw(t, schemasDir, name), overrides).(map[string]any)
		if !ok {
			t.Fatalf("%s: not an object schema", name)
		}
		return inst
	}
	if err := s.Validate(toJSONValue(t, build())); err != nil {
		t.Fatalf("minimal instance of %s is invalid: %v", name, err)
	}
	wantLeaf := func(what string, inst map[string]any, location, keyword string) {
		t.Helper()
		ve, ok := s.Validate(toJSONValue(t, inst)).(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("%s: want a validation error", what)
		}
		var ls []leaf
		leaves(ve, &ls)
		for _, l := range ls {
			if l.location == location && l.keyword == keyword {
				return
			}
		}
		t.Errorf("%s: want %q %s, got %v", what, location, keyword, ls)
	}
	bad := build()
	bad["schema_version"] = 2
	wantLeaf("schema_version 2", bad, "/schema_version", "const")
	noStale := build()
	delete(noStale, "stale_after")
	wantLeaf("no stale_after", noStale, "", "required")
}

func TestPageV3FilesCheck(t *testing.T) {
	t.Parallel()
	for _, name := range pageV3Files {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := os.Stat(filepath.Join(schemasDir, name)); os.IsNotExist(err) {
				t.Skip("file does not exist yet")
			}
			checkPageV3(t, name, idOverride)
		})
	}
}

// slotCase describes where a figure slot sits in the minimal instance of a v3 page.
type slotCase struct {
	name   string
	put    func(inst map[string]any, slot any) // puts slot in one figure slot of the card
	figure map[string]any                      // a valid figure of that slot
}

func gapOf(reason string) map[string]any {
	return map[string]any{"kind": "gap", "reason": reason, "text": "无数据"}
}

func v3Slots() []slotCase {
	gap := gapOf("no_data")
	return []slotCase{
		{
			name: "page.market_summary.v3.json",
			put: func(inst map[string]any, slot any) {
				card := inst["card"].(map[string]any)
				card["index_table"].(map[string]any)["rows"] = []any{
					map[string]any{"name": "甲", "iv": slot, "rv20": gap, "iv_minus_rv20": gap},
				}
			},
			figure: map[string]any{"kind": "value", "value": 20.0, "text": "20.0", "unit": "vol_points"},
		},
		{
			name: "page.options_summary.v3.json",
			put: func(inst map[string]any, slot any) {
				tiles := inst["card"].(map[string]any)["tiles"].(map[string]any)
				tiles["max_loss_usd"] = map[string]any{"label": "最大亏损", "figure": slot}
			},
			figure: map[string]any{"kind": "value", "value": 4000.0, "text": "$4,000", "unit": "usd"},
		},
	}
}

func TestPageV3FigureSlots(t *testing.T) {
	t.Parallel()
	for _, sc := range v3Slots() {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			s := compileURL(t, schemasDir, schemaHost+sc.name)
			valid := func(slot any) bool {
				t.Helper()
				inst := minimal(t, schemasDir, sc.name, readRaw(t, schemasDir, sc.name), idOverride).(map[string]any)
				sc.put(inst, slot)
				return s.Validate(toJSONValue(t, inst)) == nil
			}
			if !valid(gapOf("no_data")) {
				t.Error("a gap object is invalid in a figure slot")
			}
			if !valid(sc.figure) {
				t.Error("a value object is invalid in a figure slot")
			}
			both := map[string]any{"reason": "no_data"}
			for k, v := range sc.figure {
				both[k] = v
			}
			both["text"] = "x"
			if valid(both) {
				t.Error("an object with both shapes is valid")
			}
			noKind := map[string]any{}
			for k, v := range sc.figure {
				if k != "kind" {
					noKind[k] = v
				}
			}
			if valid(noKind) {
				t.Error("a figure without kind is valid")
			}
			// "stale" is a reason of the common Gap and not of the local Gap of options_summary.
			wantStale := sc.name == "page.market_summary.v3.json"
			if got := valid(gapOf("stale")); got != wantStale {
				t.Errorf("a Gap of reason stale: valid = %v, want %v", got, wantStale)
			}
		})
	}
}

func TestPageV3ChartPoint(t *testing.T) {
	t.Parallel()
	const name = "page.market_summary.v3.json"
	s := compileURL(t, schemasDir, schemaHost+name)
	valid := func(point map[string]any) bool {
		inst := minimal(t, schemasDir, name, readRaw(t, schemasDir, name), idOverride).(map[string]any)
		inst["card"].(map[string]any)["chart"].(map[string]any)["points"] = []any{point}
		return s.Validate(toJSONValue(t, inst)) == nil
	}
	if !valid(map[string]any{"label": "10-24"}) {
		t.Error("a chart point with no close is invalid")
	}
	if valid(map[string]any{"label": "10-24", "close": nil}) {
		t.Error("a chart point with close null is valid")
	}
	if !valid(map[string]any{"label": "10-23", "close": 8.0, "ma200": 6.5, "band_low": 5.5, "band_high": 7.0}) {
		t.Error("a full chart point is invalid")
	}
}

var unitName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// v3Rules holds what the conventions of a v3 file need to know while it is walked.
type v3Rules struct {
	t        *testing.T
	name     string
	common   map[string]any
	localGap bool // options_summary keeps a local Gap
}

// sharedLocal reports whether a local name takes the place of a shared def.
func (r v3Rules) sharedLocal(n string) bool {
	_, shared := r.common[n]
	return shared && (!r.localGap || n != "Gap")
}

func (r v3Rules) header(doc map[string]any) {
	t := r.t
	if doc["$id"] != schemaHost+r.name {
		t.Errorf("$id = %v", doc["$id"])
	}
	props, _ := doc["properties"].(map[string]any)
	sv, _ := props["schema_version"].(map[string]any)
	if sv["const"] != float64(3) || sv["type"] != "integer" {
		t.Errorf("schema_version = %v, want integer const 3", sv)
	}
	req, _ := doc["required"].([]any)
	if !slices.Contains(req, any("stale_after")) {
		t.Error("stale_after is not required")
	}
	defs, _ := doc["$defs"].(map[string]any)
	for n := range defs {
		if r.sharedLocal(n) {
			t.Errorf("local def %q has the name of a shared def", n)
		}
		if strings.Contains(n, "__") {
			t.Errorf("def name %q contains __", n)
		}
	}
}

// oneOf: every branch requires a kind with a const, and the consts differ.
func (r v3Rules) oneOf(v any) {
	branches, _ := v.([]any)
	consts := map[any]bool{}
	for _, br := range branches {
		bm := br.(map[string]any)
		if ref, ok := bm["$ref"].(string); ok {
			bm, _ = follow(r.t, schemasDir, r.name, ref)
		}
		req, _ := bm["required"].([]any)
		kp, _ := bm["properties"].(map[string]any)
		kind, _ := kp["kind"].(map[string]any)
		c, hasConst := kind["const"]
		if !slices.Contains(req, any("kind")) || !hasConst {
			r.t.Errorf("oneOf branch %v does not require a kind const", br)
		}
		if consts[c] {
			r.t.Errorf("oneOf branches share the kind %v", c)
		}
		consts[c] = true
	}
}

// properties checks the unit const and the number type of money.
func (r v3Rules) properties(v any) {
	pm, _ := v.(map[string]any)
	unit, ok := pm["unit"].(map[string]any)
	if !ok {
		return
	}
	u, _ := unit["const"].(string)
	if !unitName.MatchString(u) {
		r.t.Errorf("unit const %q does not match %s", u, unitName)
	}
	if val, _ := pm["value"].(map[string]any); strings.HasPrefix(u, "usd") && val["type"] != "number" {
		r.t.Errorf("money value of unit %s is %v, want number", u, val["type"])
	}
}

func (r v3Rules) node(k string, v any) {
	switch k {
	case "$ref":
		ref, _ := v.(string)
		if local, ok := strings.CutPrefix(ref, "#/$defs/"); ok && r.sharedLocal(local) {
			r.t.Errorf("ref %q is a local ref to a shared name", ref)
		}
	case "type":
		if v == "null" {
			r.t.Error(`type "null" occurs`)
		}
		if l, ok := v.([]any); ok && slices.Contains(l, any("null")) {
			r.t.Error(`type list holds "null"`)
		}
	case "anyOf":
		r.t.Error("anyOf occurs")
	case "oneOf":
		r.oneOf(v)
	case "x-timezone":
		if v != "owner" {
			r.t.Errorf("x-timezone = %v, want owner", v)
		}
	case "properties":
		r.properties(v)
	}
}

// shared names come from common.v2.json; options_summary keeps a local Gap.
func TestPageV3Conventions(t *testing.T) {
	t.Parallel()
	common := readRaw(t, schemasDir, "common.v2.json")["$defs"].(map[string]any)
	for _, name := range pageV3Files {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(schemasDir, name))
			if os.IsNotExist(err) {
				t.Skip("file does not exist yet")
			}
			if err != nil {
				t.Fatal(err)
			}
			r := v3Rules{t: t, name: name, common: common, localGap: name == "page.options_summary.v3.json"}
			doc := readRaw(t, schemasDir, name)
			r.header(doc)
			for _, bad := range []string{"common.v1.json", "ROUND_HALF_UP"} {
				if strings.Contains(string(b), bad) {
					t.Errorf("%s occurs", bad)
				}
			}
			walk(doc, r.node)
			compilePageV2(t, name)
		})
	}
}
