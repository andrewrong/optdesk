package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func loadMacroRatesV3(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(optionPageFixturesDir, "page.macro_rates.v3."+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestMacroRatesV3Fixtures validates golden, gap, stale and seven of page.macro_rates.v3, rejects
// bad-version at /schema_version, and pins the content of the single-series and the seven fixtures.
func TestMacroRatesV3Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.macro_rates.v3.json")
	for _, name := range []string{"golden", "gap", "stale", "seven"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(loadMacroRatesV3(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		err := s.Validate(loadMacroRatesV3(t, "bad-version"))
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
	seriesOf := func(t *testing.T, name string) []map[string]any {
		t.Helper()
		var out []map[string]any
		raw, _ := loadMacroRatesV3(t, name)["series"].([]any)
		for _, x := range raw {
			out = append(out, x.(map[string]any))
		}
		return out
	}
	for _, name := range []string{"golden", "gap", "stale", "bad-version"} {
		t.Run(name+"-one-series", func(t *testing.T) {
			t.Parallel()
			ss := seriesOf(t, name)
			if len(ss) != 1 || ss[0]["id"] != "DGS10" || ss[0]["unit"] != "percent" {
				t.Errorf("series = %v, want one DGS10 in percent", ss)
			}
		})
	}
	t.Run("seven-content", func(t *testing.T) {
		t.Parallel()
		ss := seriesOf(t, "seven")
		var ids, titles []string
		points := 0
		for _, x := range ss {
			ids = append(ids, x["id"].(string))
			titles = append(titles, x["title"].(string))
			if x["unit"] == "index_points" {
				points++
			}
		}
		wantIDs := []string{"DGS10", "DGS3MO", "SP500", "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS"}
		wantTitles := []string{"10-year Treasury yield", "3-month Treasury yield", "S&P 500", "VIX",
			"VXN (Nasdaq-100 volatility)", "RVX (Russell 2000 volatility)", "VXV (3-month volatility)"}
		if !slices.Equal(ids, wantIDs) || !slices.Equal(titles, wantTitles) || points != 5 {
			t.Errorf("ids %v titles %v index_points %d", ids, titles, points)
		}
	})
	t.Run("v2-payload-rejected", func(t *testing.T) {
		t.Parallel()
		m := loadMacroRatesV3(t, "golden")
		m["schema_version"] = float64(2)
		if s.Validate(m) == nil {
			t.Error("a version 2 payload is valid under v3")
		}
	})
}
