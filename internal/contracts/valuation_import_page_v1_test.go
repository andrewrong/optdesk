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

// TestValuationImportLabels: both fixtures validate with labels; labels without general_tick, and with an
// unknown key, are rejected.
func TestValuationImportLabels(t *testing.T) {
	t.Parallel()
	for _, view := range []string{"preview", "result"} {
		m := valuationImportFixture(t, view)
		labels, ok := m["labels"].(map[string]any)
		if !ok || len(labels) != 4 {
			t.Fatalf("%s fixture: labels = %v", view, m["labels"])
		}
		if !pageV1Valid(t, valuationImportPageV1, m) {
			t.Fatalf("%s fixture with labels rejected", view)
		}
		delete(labels, "general_tick")
		if pageV1Valid(t, valuationImportPageV1, m) {
			t.Fatalf("%s: labels without general_tick accepted", view)
		}
		labels["general_tick"] = "x"
		labels["extra"] = "x"
		if pageV1Valid(t, valuationImportPageV1, m) {
			t.Fatalf("%s: labels with an unknown key accepted", view)
		}
	}
}

// TestValuationImportLabelsEmptyState: labels that also hold the eighteen page texts validate, and the
// four-key labels of the first version still validate.
func TestValuationImportLabelsEmptyState(t *testing.T) {
	t.Parallel()
	m := valuationImportFixture(t, "preview")
	labels, ok := m["labels"].(map[string]any)
	if !ok || len(labels) != 4 {
		t.Fatalf("preview fixture: labels = %v", m["labels"])
	}
	if !pageV1Valid(t, valuationImportPageV1, m) {
		t.Fatal("four-key labels rejected")
	}
	for _, k := range []string{"title", "intro", "file", "preview", "not_in_file", "unvalued", "removed",
		"count_insert", "count_same", "count_older", "count_refused", "count_needs_tick", "stored",
		"older_than_stored", "margin_table", "result", "same_file_again", "confirm"} {
		labels[k] = "文本 text"
	}
	if len(labels) != 22 {
		t.Fatalf("labels = %d keys", len(labels))
	}
	if !pageV1Valid(t, valuationImportPageV1, m) {
		t.Fatal("labels with the eighteen new texts rejected")
	}
}
