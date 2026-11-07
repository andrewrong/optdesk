package contracts_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestCommonV2Defs pins the ten defs of common.v2.json with one valid and one invalid
// instance per def.
func TestCommonV2Defs(t *testing.T) {
	t.Parallel()
	raw := readRaw(t, schemasDir, "common.v2.json")
	if raw["$id"] != schemaHost+"common.v2.json" {
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

	const at = `"2026-01-02T03:04:05Z"`
	rows := []struct{ name, def, valid, invalid string }{
		{"id", "Id", `"run_1-A"`, `"has space"`},
		{"timestamp zero offset", "Timestamp", `"2026-01-02T03:04:05+00:00"`, `"yesterday"`},
		{"timestamp offset", "Timestamp", `"2026-01-02T03:04:05+08:00"`, `20260102`},
		{"run status", "RunStatus", `"partial"`, `"done"`},
		{"source time", "SourceTime", `{"source":"s1","data_time":` + at + `}`, `{"source":"bad id","data_time":` + at + `}`},
		{"stale label", "StaleLabel", `{"data_time":` + at + `,"text":"old","is_stale":true}`, `{"data_time":` + at + `,"text":"old"}`},
		{"stale label with stale_after", "StaleLabel", `{"data_time":` + at + `,"text":"old","is_stale":false}`, `{"data_time":` + at + `,"stale_after":` + at + `,"text":"old"}`},
		{"stale label long text", "StaleLabel", `{"data_time":` + at + `,"text":"x","is_stale":false}`, `{"data_time":` + at + `,"text":"` + strings.Repeat("x", 201) + `","is_stale":false}`},
		{"gap", "Gap", `{"kind":"gap","reason":"stale_input","text":"late"}`, `{"reason":"stale_input","text":"late"}`},
		{"gap reason", "Gap", `{"kind":"gap","reason":"stale","text":"late"}`, `{"kind":"gap","reason":"bogus","text":"late"}`},
		{"usd", "UsdFigure", `{"kind":"value","value":1.25,"text":"$1","unit":"usd"}`, `{"value":1.25,"text":"$1","unit":"usd"}`},
		{"usd unit", "UsdFigure", `{"kind":"value","value":1,"text":"$1","unit":"usd"}`, `{"kind":"value","value":1,"text":"$1","unit":"USD"}`},
		{"percent", "PercentFigure", `{"kind":"value","value":2.1,"text":"+2.1%","unit":"percent","tone":"up"}`, `{"kind":"value","value":10001,"text":"x","unit":"percent"}`},
		{"share percent", "SharePercentFigure", `{"kind":"value","value":50,"text":"50%","unit":"share_percent"}`, `{"kind":"value","value":101,"text":"x","unit":"share_percent"}`},
		{"usd signed", "UsdSignedFigure", `{"kind":"value","value":-5.5,"text":"−$5","unit":"usd_signed"}`, `{"kind":"value","value":-5.5,"text":"−$5","unit":"usd_signed","tone":null}`},
	}
	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.def] = true
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			s := compileURL(t, schemasDir, schemaHost+"common.v2.json#/$defs/"+r.def)
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
