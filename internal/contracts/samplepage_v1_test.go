package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func loadSamplePageV1(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(optionPageFixturesDir, "page.sample_page.v1."+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestSamplePageV1Fixtures validates ok, rejects bad-version at /schema_version, a page name outside
// the enum at /page and an extra field at the top.
func TestSamplePageV1Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.sample_page.v1.json")
	failsAt := func(t *testing.T, m map[string]any, want leaf) {
		t.Helper()
		err := s.Validate(m)
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("err = %v, want a validation error", err)
		}
		var got []leaf
		leaves(ve, &got)
		if len(got) != 1 || got[0] != want {
			t.Errorf("failures = %v, want one %v", got, want)
		}
	}
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		if err := s.Validate(loadSamplePageV1(t, "ok")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("every-page-name", func(t *testing.T) {
		t.Parallel()
		for _, p := range []string{"home_page", "household_page", "positions_page", "household_risk", "option_page", "advice_page"} {
			m := loadSamplePageV1(t, "ok")
			m["page"] = p
			if err := s.Validate(m); err != nil {
				t.Errorf("%s: %v", p, err)
			}
		}
	})
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		failsAt(t, loadSamplePageV1(t, "bad-version"), leaf{"/schema_version", "const"})
	})
	t.Run("page-outside-enum", func(t *testing.T) {
		t.Parallel()
		m := loadSamplePageV1(t, "ok")
		m["page"] = "macro_rates"
		failsAt(t, m, leaf{"/page", "enum"})
	})
	t.Run("extra-field", func(t *testing.T) {
		t.Parallel()
		m := loadSamplePageV1(t, "ok")
		m["extra"] = 1
		failsAt(t, m, leaf{"", "additionalProperties"})
	})
}
