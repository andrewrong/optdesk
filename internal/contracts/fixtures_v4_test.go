package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestFixturesV4Options: the three options v4 fixtures of contracts/fixtures are valid, hold no null, and
// carry what their name says (no gap in ok, a gap in gap, a stale label in stale).
func TestFixturesV4Options(t *testing.T) {
	t.Parallel()
	s := compileURL(t, schemasDir, schemaHost+optionsV4)
	for _, name := range []string{"ok", "gap", "stale"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(optionPageFixturesDir, "page.options_summary.v4."+name+".json")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Validate(inst); err != nil {
				t.Fatalf("invalid: %v", err)
			}
			plain := readRaw(t, optionPageFixturesDir, "page.options_summary.v4."+name+".json")
			if hasNull(plain) {
				t.Error("holds null")
			}
			card := plain["card"].(map[string]any)
			if card["closed_series_text"] == "" || card["closed_series_text"] == nil {
				t.Error("closed_series_text is empty")
			}
			switch name {
			case "ok":
				if anyObject(plain, isGap) || anyObject(plain, isStaleTrue) {
					t.Error("ok holds a gap or a stale label")
				}
			case "gap":
				if !anyObject(plain, isGap) {
					t.Error("holds no gap")
				}
			case "stale":
				if !anyObject(plain, isStaleTrue) {
					t.Error("holds no stale label")
				}
			}
		})
	}
}
