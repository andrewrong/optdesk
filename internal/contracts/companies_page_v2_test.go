package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const companiesPageV2 = "page.companies_page.v2.json"

func companiesV2Fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	return readRaw(t, filepath.Join("..", "..", "contracts", "fixtures"), "page.companies_page.v2."+name+".json")
}

// TestCompaniesPageV2Fixtures: the ok and gap fixtures validate, an unknown verdict field is
// refused at the page, in a block and in a price line cell, and the input fixture parses with
// the sections a builder test reads.
func TestCompaniesPageV2Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, companiesPageV2)
	for _, name := range []string{"ok", "gap"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(companiesV2Fixture(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("unknown-verdict-field", func(t *testing.T) {
		t.Parallel()
		page := companiesV2Fixture(t, "ok")
		page["verdict"] = "buy"
		if s.Validate(page) == nil {
			t.Fatal("unknown top-level field validates")
		}
		page = companiesV2Fixture(t, "ok")
		page["core"].(map[string]any)["verdict"] = "buy"
		if s.Validate(page) == nil {
			t.Fatal("unknown core field validates")
		}
		page = companiesV2Fixture(t, "ok")
		rows := page["watch_list"].(map[string]any)["rows"].([]any)
		rows[0].(map[string]any)["cells"].([]any)[1].(map[string]any)["verdict"] = "buy"
		if s.Validate(page) == nil {
			t.Fatal("unknown cell field validates")
		}
	})
	t.Run("price-state-enum", func(t *testing.T) {
		t.Parallel()
		for state, want := range map[string]bool{"band": true, "owner override": true, "awaiting approval": true, "expired": true, "cheap": false} {
			page := companiesV2Fixture(t, "ok")
			rows := page["watch_list"].(map[string]any)["rows"].([]any)
			rows[0].(map[string]any)["cells"].([]any)[1].(map[string]any)["price_state"] = state
			if got := s.Validate(page) == nil; got != want {
				t.Errorf("price_state %q: valid = %v, want %v", state, got, want)
			}
		}
	})
	t.Run("blocks-are-required", func(t *testing.T) {
		t.Parallel()
		for _, key := range []string{"core", "diagnosis", "questions"} {
			page := companiesV2Fixture(t, "ok")
			delete(page, key)
			if s.Validate(page) == nil {
				t.Errorf("page without %s validates", key)
			}
		}
	})
	t.Run("version-1-fails", func(t *testing.T) {
		t.Parallel()
		page := companiesV2Fixture(t, "ok")
		page["schema_version"] = 1
		if s.Validate(page) == nil {
			t.Fatal("version 1 validates")
		}
	})
	t.Run("input-parses", func(t *testing.T) {
		t.Parallel()
		checkCompaniesV2Input(t)
	})
}

// checkCompaniesV2Input parses the input fixture and checks the sections a builder test reads.
func checkCompaniesV2Input(t *testing.T) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.companies_page.v2.input.json"))
	if err != nil {
		t.Fatal(err)
	}
	var in struct {
		Today    string            `json:"today"`
		Subjects []json.RawMessage `json:"subjects"`
		Readings []json.RawMessage `json:"readings"`
		Facts    []json.RawMessage `json:"facts"`
		Series   []struct {
			SeriesID        string `json:"series_id"`
			ObservationDate string `json:"observation_date"`
		} `json:"series"`
		Instruments []struct {
			ID     int    `json:"id"`
			Symbol string `json:"symbol"`
		} `json:"instruments"`
		Quotes     []json.RawMessage          `json:"quotes"`
		OptionRows []struct{ Strike string }  `json:"option_rows"`
		OwnerText  map[string]map[string]any  `json:"owner_text"`
		Figures    map[string]json.RawMessage `json:"figures"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	if in.Today == "" || len(in.Subjects) == 0 || len(in.Readings) == 0 || len(in.Facts) == 0 ||
		len(in.Quotes) == 0 || len(in.OwnerText) == 0 {
		t.Fatalf("an input section is empty: %+v", in)
	}
	if len(in.Series) == 0 || in.Series[0].SeriesID != "DGS10" {
		t.Errorf("series = %v, want DGS10 points", in.Series)
	}
	for _, o := range in.OptionRows {
		if o.Strike == "" {
			t.Error("an option row has no strike")
		}
	}
	for _, inst := range in.Instruments {
		if _, ok := in.Figures[inst.Symbol]; !ok {
			t.Errorf("no figures for instrument symbol %s", inst.Symbol)
		}
	}
	if len(in.Instruments) == 0 {
		t.Error("no instruments")
	}
}

// TestCompaniesPageV2PepbFixture: the pepb fixture validates, the ok and gap fixtures (no pepb)
// still validate, and a verdict field on a pepb row or line is refused.
func TestCompaniesPageV2PepbFixture(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, companiesPageV2)
	for _, name := range []string{"pepb", "ok", "gap"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			page := companiesV2Fixture(t, name)
			if _, has := page["pepb"]; has != (name == "pepb") {
				t.Fatalf("%s: pepb present = %v", name, has)
			}
			if err := s.Validate(page); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	pepbRows := func(page map[string]any) []any {
		return page["pepb"].(map[string]any)["rows"].([]any)
	}
	t.Run("verdict-field-on-row", func(t *testing.T) {
		t.Parallel()
		page := companiesV2Fixture(t, "pepb")
		pepbRows(page)[0].(map[string]any)["verdict"] = "cheap"
		if s.Validate(page) == nil {
			t.Fatal("verdict field on a pepb row validates")
		}
	})
	t.Run("verdict-field-on-line", func(t *testing.T) {
		t.Parallel()
		page := companiesV2Fixture(t, "pepb")
		pepbRows(page)[0].(map[string]any)["pe"].(map[string]any)["verdict"] = "cheap"
		if s.Validate(page) == nil {
			t.Fatal("verdict field on a pepb line validates")
		}
	})
	t.Run("forward-must-be-gap", func(t *testing.T) {
		t.Parallel()
		page := companiesV2Fixture(t, "pepb")
		row := pepbRows(page)[0].(map[string]any)
		row["forward"] = row["pe"]
		if s.Validate(page) == nil {
			t.Fatal("a line as forward validates")
		}
	})
	t.Run("pepb-not-required", func(t *testing.T) {
		t.Parallel()
		page := companiesV2Fixture(t, "pepb")
		delete(page, "pepb")
		if err := s.Validate(page); err != nil {
			t.Fatalf("page without pepb: %v", err)
		}
	})
}
