package contracts_test

import "testing"

const (
	valuationPageV1       = "page.valuation_page.v1.json"
	valuationParamsPageV1 = "page.valuation_params_page.v1.json"
)

var valuationFixtures = []struct{ schema, file string }{
	{valuationParamsPageV1, "page.valuation_params_page.v1.ok.json"},
	{valuationPageV1, "page.valuation_page.v1.table.json"},
	{valuationPageV1, "page.valuation_page.v1.company.json"},
	{valuationPageV1, "page.valuation_page.v1.hkcn.json"},
	{valuationPageV1, "page.valuation_page.v1.form.json"},
}

func valuationFixture(t *testing.T, file string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, file)
}

// TestValuationFixturesValidate: every fixture passes its schema.
func TestValuationFixturesValidate(t *testing.T) {
	t.Parallel()
	for _, f := range valuationFixtures {
		s := compilePageV2(t, f.schema)
		if err := s.Validate(pageV2JSON(t, valuationFixture(t, f.file))); err != nil {
			t.Errorf("%s: %v", f.file, err)
		}
	}
}

// TestValuationRejectsUnknownField: an extra key on a lines object fails.
func TestValuationRejectsUnknownField(t *testing.T) {
	t.Parallel()
	m := valuationFixture(t, "page.valuation_page.v1.company.json")
	m["lines"].(map[string]any)["extra"] = "1"
	if pageV1Valid(t, valuationPageV1, m) {
		t.Fatal("a lines object with an extra key was accepted")
	}
}

// TestValuationFiguresAreText: a number where a text is required fails.
func TestValuationFiguresAreText(t *testing.T) {
	t.Parallel()
	m := valuationFixture(t, "page.valuation_page.v1.company.json")
	m["lines"].(map[string]any)["c"] = 90
	if pageV1Valid(t, valuationPageV1, m) {
		t.Fatal("a number for the fair price was accepted")
	}
	p := valuationFixture(t, "page.valuation_params_page.v1.ok.json")
	p["current_r"].(map[string]any)["r"].(map[string]any)["us"] = 0.9
	if pageV1Valid(t, valuationParamsPageV1, p) {
		t.Fatal("a number for an R value was accepted")
	}
}

// TestValuationViewEnum: only table, company and form are views.
func TestValuationViewEnum(t *testing.T) {
	t.Parallel()
	m := valuationFixture(t, "page.valuation_page.v1.company.json")
	m["view"] = "chart"
	if pageV1Valid(t, valuationPageV1, m) {
		t.Fatal(`view "chart" accepted`)
	}
}

// TestValuationParamsNoRTableGap: a page with no R table, no proposal and the gap no_r_table validates.
func TestValuationParamsNoRTableGap(t *testing.T) {
	t.Parallel()
	p := valuationFixture(t, "page.valuation_params_page.v1.ok.json")
	p["r_table"] = nil
	p["proposal"] = nil
	p["gaps"] = []any{map[string]any{"kind": "gap", "reason": "no_r_table", "text": "no R table stored"}}
	if !pageV1Valid(t, valuationParamsPageV1, p) {
		t.Fatal("the no_r_table gap that design entry 6 requires is refused")
	}
}
