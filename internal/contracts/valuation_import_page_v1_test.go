package contracts_test

import "testing"

const valuationImportPageV1 = "page.valuation_import_page.v1.json"

func valuationImportFixture(t *testing.T, view string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.valuation_import_page.v1."+view+".json")
}

// TestValuationImportPreviewValid: the preview fixture passes the schema and carries an unmatched row.
func TestValuationImportPreviewValid(t *testing.T) {
	t.Parallel()
	m := valuationImportFixture(t, "preview")
	if err := compilePageV2(t, valuationImportPageV1).Validate(pageV2JSON(t, m)); err != nil {
		t.Fatal(err)
	}
	if m["view"] != "preview" {
		t.Fatalf("view = %v", m["view"])
	}
	unmatched := false
	for _, r := range m["rows"].([]any) {
		if r.(map[string]any)["unmatched"] == true {
			unmatched = true
		}
	}
	if !unmatched {
		t.Fatal("the preview fixture has no unmatched row")
	}
}

// TestValuationImportResultValid: the result fixture passes the schema and lists outcomes.
func TestValuationImportResultValid(t *testing.T) {
	t.Parallel()
	m := valuationImportFixture(t, "result")
	if err := compilePageV2(t, valuationImportPageV1).Validate(pageV2JSON(t, m)); err != nil {
		t.Fatal(err)
	}
	if m["view"] != "result" || len(m["result"].([]any)) == 0 {
		t.Fatal("the result fixture has no result items")
	}
}

// TestValuationImportRejectsBadView: only empty, preview and result are views; an extra key fails.
func TestValuationImportRejectsBadView(t *testing.T) {
	t.Parallel()
	m := valuationImportFixture(t, "preview")
	m["view"] = "confirm"
	if pageV1Valid(t, valuationImportPageV1, m) {
		t.Fatal(`view "confirm" accepted`)
	}
	m = valuationImportFixture(t, "preview")
	m["digest"] = "abc"
	if pageV1Valid(t, valuationImportPageV1, m) {
		t.Fatal("a digest property was accepted")
	}
	m = valuationImportFixture(t, "preview")
	m["post_path"] = "other-post"
	if pageV1Valid(t, valuationImportPageV1, m) {
		t.Fatal("another post_path was accepted")
	}
}

// TestValuationImportRejectsBadOutcome: an outcome outside the six names fails.
func TestValuationImportRejectsBadOutcome(t *testing.T) {
	t.Parallel()
	m := valuationImportFixture(t, "result")
	m["result"].([]any)[0].(map[string]any)["outcome"] = "written"
	if pageV1Valid(t, valuationImportPageV1, m) {
		t.Fatal(`outcome "written" accepted`)
	}
}
