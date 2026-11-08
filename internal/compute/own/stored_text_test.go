package own

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// repoRoot walks up from the test directory to go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// TestStoredTextGolden applies the real rounding at the scale of each rule id, read from
// rules.json as data, to a literal input and compares with the expected stored text.
func TestStoredTextGolden(t *testing.T) {
	t.Parallel()
	var rules []struct {
		ID    string `json:"id"`
		Scale int    `json:"scale"`
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "compute", "rules", "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rules); err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID    string  `json:"id"`
		Input float64 `json:"input"`
		Want  string  `json:"want"`
	}
	raw, err = os.ReadFile(filepath.Join("testdata", "stored_text.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	scale := map[string]int{}
	for _, r := range rules {
		scale[r.ID] = r.Scale
	}
	if len(cases) != len(rules) {
		t.Fatalf("%d cases for %d ids", len(cases), len(rules))
	}
	u := NewUnits()
	for _, c := range cases {
		s, ok := scale[c.ID]
		if !ok {
			t.Errorf("%s: no rule", c.ID)
			continue
		}
		got, err := u.RoundHalfEven(c.Input, s)
		if err != nil || got != c.Want {
			t.Errorf("%s: %v -> %q (%v), want %q", c.ID, c.Input, got, err, c.Want)
		}
	}
}
