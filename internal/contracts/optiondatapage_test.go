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
	for _, name := range []string{"ok", "terms", "gap", "stale", "choices", "jpy"} {
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

// TestOptionDataPageJPY checks the shape of the jpy fixture and that the four optional fields are
// validated: an approx_usd_text over 200 characters, an earnings_tone outside the enum and an
// unknown key in Underlying each fail at their own location.
func TestOptionDataPageJPY(t *testing.T) {
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
	// jpyUnderlying returns the underlying object of the jpy fixture.
	jpyUnderlying := func(t *testing.T) (map[string]any, map[string]any) {
		t.Helper()
		v, ok := load(t, "jpy").(map[string]any)
		if !ok {
			t.Fatal("jpy is not an object")
		}
		u, ok := v["underlying"].(map[string]any)
		if !ok {
			t.Fatal("jpy underlying is not an object")
		}
		return v, u
	}
	t.Run("jpy-shape", func(t *testing.T) {
		t.Parallel()
		v, u := jpyUnderlying(t)
		if series, ok := v["quote_series"].([]any); !ok || len(series) != 0 {
			t.Errorf("quote_series = %v, want an empty list", v["quote_series"])
		}
		if u["currency"] != "JPY" {
			t.Errorf("currency = %v, want JPY", u["currency"])
		}
		if tone := u["earnings_tone"]; tone != "in_term" {
			t.Errorf("earnings_tone = %v, want in_term", tone)
		}
		if vl, _ := u["vol_line"].(string); !strings.Contains(vl, "HV30 缺") {
			t.Errorf("vol_line = %q, want it to contain HV30 缺", vl)
		}
		if et, _ := u["earnings_text"].(string); et == "" {
			t.Error("earnings_text is empty")
		}
		price, _ := u["price"].(map[string]any)
		if price["kind"] != "value" || price["approx_usd_text"] == nil {
			t.Errorf("price = %v, want a value figure with approx_usd_text", price)
		}
		chain, _ := v["chain"].(map[string]any)
		rows, _ := chain["rows"].([]any)
		checkJPYRows(t, rows)
	})
	t.Run("jpy-bad-approx-usd-text", func(t *testing.T) {
		t.Parallel()
		v, u := jpyUnderlying(t)
		_ = v
		price, _ := u["price"].(map[string]any)
		price["approx_usd_text"] = strings.Repeat("x", 201)
		wantLeaf(t, s.Validate(v), leaf{"/underlying/price/approx_usd_text", "maxLength"})
	})
	t.Run("jpy-bad-earnings-tone", func(t *testing.T) {
		t.Parallel()
		v, u := jpyUnderlying(t)
		u["earnings_tone"] = "red"
		wantLeaf(t, s.Validate(v), leaf{"/underlying/earnings_tone", "enum"})
	})
	t.Run("jpy-unknown-underlying-key", func(t *testing.T) {
		t.Parallel()
		v, u := jpyUnderlying(t)
		u["bogus"] = "x"
		wantLeaf(t, s.Validate(v), leaf{"/underlying", "additionalProperties"})
	})
}

// wantLeaf fails unless err is a validation error with a failure at want.
func wantLeaf(t *testing.T, err error, want leaf) {
	t.Helper()
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		t.Fatalf("err = %v, want a validation error", err)
	}
	var got []leaf
	leaves(ve, &got)
	for _, g := range got {
		if g == want {
			return
		}
	}
	t.Errorf("failures = %v, want one at %s with %s", got, want.location, want.keyword)
}

// checkJPYRows asserts every strike is a Figure with approx_usd_text and exactly one bid is a Gap with a reason and text.
func checkJPYRows(t *testing.T, rows []any) {
	t.Helper()
	gaps := 0
	for i, r := range rows {
		row, _ := r.(map[string]any)
		strike, _ := row["strike"].(map[string]any)
		if strike["kind"] != "value" || strike["approx_usd_text"] == nil {
			t.Errorf("rows[%d].strike = %v, want a Figure with approx_usd_text", i, strike)
		}
		bid, _ := row["bid"].(map[string]any)
		switch bid["kind"] {
		case "gap":
			gaps++
			if r, _ := bid["reason"].(string); r == "" {
				t.Errorf("rows[%d].bid gap has no reason", i)
			}
			if x, _ := bid["text"].(string); x == "" {
				t.Errorf("rows[%d].bid gap has no text", i)
			}
		case "value":
			if bid["approx_usd_text"] == nil {
				t.Errorf("rows[%d].bid has no approx_usd_text", i)
			}
		}
	}
	if gaps != 1 {
		t.Errorf("rows with a gap bid = %d, want 1", gaps)
	}
}
