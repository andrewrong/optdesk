package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const optionPageFixturesDir = "../../contracts/fixtures"

// TestOptionDataPageFixtures validates the fixtures of page.option_data_page.v1: ok, terms, gap and
// stale and choices are valid; a choices href with a scheme fails at href; bad-version fails once, at /schema_version, with the keyword const.
func TestOptionDataPageFixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.option_data_page.v1.json")
	load := func(t *testing.T, name string) any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(optionPageFixturesDir, "page.option_data_page.v1."+name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, name := range []string{"ok", "terms", "gap", "stale", "choices"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(load(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("choices-bad-href", func(t *testing.T) {
		t.Parallel()
		v, ok := load(t, "choices").(map[string]any)
		if !ok {
			t.Fatal("choices is not an object")
		}
		first, ok := v["underlyings"].([]any)[0].(map[string]any)
		if !ok {
			t.Fatal("underlyings[0] is not an object")
		}
		first["href"] = "https://x/y"
		err := s.Validate(v)
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("err = %v, want a validation error", err)
		}
		var got []leaf
		leaves(ve, &got)
		if len(got) != 1 || got[0] != (leaf{"/underlyings/0/href", "pattern"}) {
			t.Errorf("failures = %v, want one at /underlyings/0/href with pattern", got)
		}
	})
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		err := s.Validate(load(t, "bad-version"))
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("err = %v, want a validation error", err)
		}
		var got []leaf
		leaves(ve, &got)
		if len(got) != 1 || got[0] != (leaf{"/schema_version", "const"}) {
			t.Errorf("failures = %v, want one at /schema_version with const", got)
		}
	})
}
