package contracts_test

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func loadUnderlyingsV2(t *testing.T, name string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.underlyings.v2."+name+".json")
}

// TestUnderlyingsV2Fixtures validates golden, confirm and readonly, shows bad-version fails at /schema_version, and
// shows the confirm step needs fields, at most 15 of them, each value at most 300 characters.
func TestUnderlyingsV2Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.underlyings.v2.json")
	for _, name := range []string{"golden", "confirm", "readonly"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(loadUnderlyingsV2(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		err := s.Validate(loadUnderlyingsV2(t, "bad-version"))
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("err = %v, want a validation error", err)
		}
		var got []leaf
		leaves(ve, &got)
		if len(got) != 1 || got[0] != (leaf{"/schema_version", "const"}) {
			t.Errorf("failures = %v, want one /schema_version const", got)
		}
	})
	mutations := map[string]func(c map[string]any){
		"no-fields": func(c map[string]any) { delete(c, "fields") },
		"sixteen-fields": func(c map[string]any) {
			var l []any
			for range 16 {
				l = append(l, map[string]any{"name": "note", "value": "x"})
			}
			c["fields"] = l
		},
		"value-301": func(c map[string]any) {
			c["fields"] = []any{map[string]any{"name": "note", "value": strings.Repeat("x", 301)}}
		},
		"empty-name": func(c map[string]any) {
			c["fields"] = []any{map[string]any{"name": "", "value": "x"}}
		},
		"extra-property": func(c map[string]any) {
			c["fields"] = []any{map[string]any{"name": "note", "value": "x", "extra": 1}}
		},
	}
	for name, mutate := range mutations {
		t.Run("mutation-"+name, func(t *testing.T) {
			t.Parallel()
			m := loadUnderlyingsV2(t, "confirm")
			mutate(m["confirm"].(map[string]any))
			if s.Validate(m) == nil {
				t.Error("mutated page is valid")
			}
		})
	}
	t.Run("confirm-fields", func(t *testing.T) {
		t.Parallel()
		c, _ := loadUnderlyingsV2(t, "confirm")["confirm"].(map[string]any)
		got := map[string]string{}
		for _, x := range c["fields"].([]any) {
			f := x.(map[string]any)
			got[f["name"].(string)] = f["value"].(string)
		}
		if got["key"] != "US:AAA" || got["status"] != "paused" {
			t.Errorf("fields = %v", got)
		}
	})
}
