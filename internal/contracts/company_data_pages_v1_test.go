package contracts_test

import "testing"

const (
	companyQuartersPageV1 = "page.company_quarters_page.v1.json"
	companyDataPageV1     = "page.company_data_page.v1.json"
)

var companyDataFixtures = []struct{ schema, file string }{
	{companyQuartersPageV1, "page.company_quarters_page.v1.ok.json"},
	{companyQuartersPageV1, "page.company_quarters_page.v1.gap.json"},
	{companyDataPageV1, "page.company_data_page.v1.ok.json"},
	{companyDataPageV1, "page.company_data_page.v1.gap.json"},
}

func companyDataFixture(t *testing.T, file string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, file)
}

// TestCompanyDataFixturesValidate: every fixture passes its schema, and a gap fixture carries a gap.
func TestCompanyDataFixturesValidate(t *testing.T) {
	t.Parallel()
	for _, f := range companyDataFixtures {
		s := compilePageV2(t, f.schema)
		m := companyDataFixture(t, f.file)
		if err := s.Validate(pageV2JSON(t, m)); err != nil {
			t.Errorf("%s: %v", f.file, err)
		}
	}
	for _, f := range []string{"page.company_quarters_page.v1.gap.json", "page.company_data_page.v1.gap.json"} {
		if len(companyDataFixture(t, f)["gaps"].([]any)) == 0 {
			t.Errorf("%s: gaps is empty", f)
		}
	}
}

// TestFundamentalFactOldFixtureStillValidates: a fact with no fy, fp and frame passes the r2 schema.
func TestFundamentalFactOldFixtureStillValidates(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.fundamental_fact")
	recs, _ := loadRecords(t, "obs.fundamental_fact")
	for i, r := range recs {
		for _, k := range []string{"fy", "fp", "frame"} {
			delete(r, k)
		}
		if err := s.Validate(r); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
}

// TestFundamentalFactPeriodFieldsValidate: fy, fp and frame are accepted; fy as text is not.
func TestFundamentalFactPeriodFieldsValidate(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.fundamental_fact")
	recs, _ := loadRecords(t, "obs.fundamental_fact")
	rec := recs[1]
	rec["fy"], rec["fp"], rec["frame"] = float64(2025), "Q3", "CY2025Q2"
	if err := s.Validate(rec); err != nil {
		t.Fatalf("fy 2025, fp Q3, frame CY2025Q2 refused: %v", err)
	}
	rec["fy"] = nil
	if err := s.Validate(rec); err != nil {
		t.Fatalf("null fy refused: %v", err)
	}
	rec["fy"] = "2025"
	if err := s.Validate(rec); err == nil {
		t.Fatal(`fy "2025" as text was accepted`)
	}
}

// TestCompanyDataRejectsUnknownField: an extra key on a company or a cell fails.
func TestCompanyDataRejectsUnknownField(t *testing.T) {
	t.Parallel()
	d := companyDataFixture(t, "page.company_data_page.v1.ok.json")
	d["companies"].([]any)[0].(map[string]any)["extra"] = "1"
	if pageV1Valid(t, companyDataPageV1, d) {
		t.Fatal("a company with an extra key was accepted")
	}
	q := companyDataFixture(t, "page.company_quarters_page.v1.ok.json")
	cells := q["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)
	cells["revenue"].(map[string]any)["extra"] = "1"
	if pageV1Valid(t, companyQuartersPageV1, q) {
		t.Fatal("a cell with an extra key was accepted")
	}
	r := companyDataFixture(t, "page.company_quarters_page.v1.ok.json")
	r["extra"] = "1"
	if pageV1Valid(t, companyQuartersPageV1, r) {
		t.Fatal("a page with an extra key was accepted")
	}
}

// TestCompanyDataFiguresAreText: a number where a figure text is required fails.
func TestCompanyDataFiguresAreText(t *testing.T) {
	t.Parallel()
	d := companyDataFixture(t, "page.company_data_page.v1.ok.json")
	d["companies"].([]any)[0].(map[string]any)["bar_count"] = 271
	if pageV1Valid(t, companyDataPageV1, d) {
		t.Fatal("a number for bar_count was accepted")
	}
	e := companyDataFixture(t, "page.company_data_page.v1.ok.json")
	e["companies"].([]any)[0].(map[string]any)["eps_quarters"] = 12
	if pageV1Valid(t, companyDataPageV1, e) {
		t.Fatal("a number for eps_quarters was accepted")
	}
	q := companyDataFixture(t, "page.company_quarters_page.v1.ok.json")
	cells := q["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)
	cells["revenue"].(map[string]any)["text"] = 1234500000
	if pageV1Valid(t, companyQuartersPageV1, q) {
		t.Fatal("a number for a cell text was accepted")
	}
}
