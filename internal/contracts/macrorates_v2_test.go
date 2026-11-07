package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const macroRatesV2Prefix = "page.macro_rates.v2."

// TestMacroRatesV2Fixtures validates the fixtures of page.macro_rates.v2: golden, gap and stale are
// valid; bad-version fails once, at /schema_version with const; an unknown reason of a GapLine, a
// missing text of a Point and an extra top-level field each fail at the place they are.
func TestMacroRatesV2Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.macro_rates.v2.json")
	load := func(t *testing.T, name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(optionPageFixturesDir, macroRatesV2Prefix+name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, name := range []string{"golden", "gap", "stale"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(load(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("golden-content", func(t *testing.T) {
		t.Parallel()
		m := load(t, "golden")
		series, _ := m["series"].([]any)
		gaps, _ := m["gaps"].([]any)
		if len(series) != 1 || len(gaps) != 1 {
			t.Fatalf("series %d, gaps %d, want 1 and 1", len(series), len(gaps))
		}
		points, _ := series[0].(map[string]any)["points"].([]any)
		if len(points) != 3 {
			t.Errorf("points = %d, want 3", len(points))
		}
		if r := gaps[0].(map[string]any)["reason"]; r != "no_value" {
			t.Errorf("gap reason = %v, want no_value", r)
		}
	})
	failsAt := func(t *testing.T, m map[string]any, want leaf) {
		t.Helper()
		err := s.Validate(m)
		if err == nil {
			t.Fatal("valid, want a failure")
		}
		var got []leaf
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("err = %v, want a validation error", err)
		}
		leaves(ve, &got)
		if len(got) != 1 || got[0] != want {
			t.Errorf("failures = %v, want one %v", got, want)
		}
	}
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		failsAt(t, load(t, "bad-version"), leaf{"/schema_version", "const"})
	})
	t.Run("unknown-gap-reason", func(t *testing.T) {
		t.Parallel()
		m := load(t, "golden")
		m["gaps"].([]any)[0].(map[string]any)["reason"] = "missing"
		failsAt(t, m, leaf{"/gaps/0/reason", "enum"})
	})
	t.Run("point-without-text", func(t *testing.T) {
		t.Parallel()
		m := load(t, "golden")
		p := m["series"].([]any)[0].(map[string]any)["points"].([]any)[0].(map[string]any)
		delete(p, "text")
		failsAt(t, m, leaf{"/series/0/points/0", "required"})
	})
	t.Run("extra-top-level-field", func(t *testing.T) {
		t.Parallel()
		m := load(t, "golden")
		m["extra"] = 1
		failsAt(t, m, leaf{"", "additionalProperties"})
	})
}
