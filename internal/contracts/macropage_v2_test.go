package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func loadMacroPageV2(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(optionPageFixturesDir, "page.macro_page.v2."+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func failuresOf(t *testing.T, err error) []leaf {
	t.Helper()
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		t.Fatalf("err = %v, want a validation error", err)
	}
	var got []leaf
	leaves(ve, &got)
	return got
}

// TestMacroPageV2Fixtures validates golden, gap and stale of page.macro_page.v2 and rejects
// bad-version at /schema_version.
func TestMacroPageV2Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.macro_page.v2.json")
	for _, name := range []string{"golden", "gap", "stale"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(loadMacroPageV2(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		got := failuresOf(t, s.Validate(loadMacroPageV2(t, "bad-version")))
		if len(got) != 1 || got[0] != (leaf{"/schema_version", "const"}) {
			t.Errorf("failures = %v, want one /schema_version const", got)
		}
	})
}

// TestMacroPageV2LiquidityShape rejects a block with three or five lines, a tap key outside the
// four, and a line key outside the four.
func TestMacroPageV2LiquidityShape(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.macro_page.v2.json")
	cases := []struct {
		name   string
		mutate func(liq map[string]any)
		want   leaf
	}{
		{"three-lines", func(l map[string]any) { l["lines"] = l["lines"].([]any)[:3] }, leaf{"/liquidity/lines", "minItems"}},
		{"five-lines", func(l map[string]any) {
			ls := l["lines"].([]any)
			l["lines"] = append(ls, ls[0])
		}, leaf{"/liquidity/lines", "maxItems"}},
		{"tap-key-score", func(l map[string]any) {
			l["taps"].([]any)[0].(map[string]any)["key"] = "score"
		}, leaf{"/liquidity/taps/0/key", "enum"}},
		{"line-key-regime", func(l map[string]any) {
			l["lines"].([]any)[1].(map[string]any)["key"] = "regime"
		}, leaf{"/liquidity/lines/1/key", "enum"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := loadMacroPageV2(t, "golden")
			c.mutate(m["liquidity"].(map[string]any))
			err := s.Validate(m)
			if err == nil {
				t.Fatal("payload is valid, want a failure")
			}
			found := false
			for _, g := range failuresOf(t, err) {
				if g == c.want {
					found = true
				}
			}
			if !found {
				t.Errorf("failures = %v, want %v among them", failuresOf(t, err), c.want)
			}
		})
	}
}
