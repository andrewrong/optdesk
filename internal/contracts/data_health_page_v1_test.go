package contracts_test

import "testing"

const dataHealthPageV1 = "page.data_health_page.v1.json"

func dataHealthFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.data_health_page.v1."+name+".json")
}

func firstCell(m map[string]any) map[string]any {
	ds := m["datasets"].([]any)[0].(map[string]any)
	return ds["cells"].([]any)[0].(map[string]any)
}

// TestDataHealthFixturesValidate: both fixtures pass the schema.
func TestDataHealthFixturesValidate(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, dataHealthPageV1)
	for _, name := range []string{"ok", "gap"} {
		if err := s.Validate(pageV2JSON(t, dataHealthFixture(t, name))); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestDataHealthRejectsUnknownField: a cell with an extra key fails.
func TestDataHealthRejectsUnknownField(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	firstCell(m)["extra"] = 1
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("a cell with an extra key was accepted")
	}
}

// TestDataHealthRepairEnum: only none, button and not_refetchable are repair values.
func TestDataHealthRepairEnum(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"none", "button", "not_refetchable"} {
		m := dataHealthFixture(t, "ok")
		firstCell(m)["repair"] = v
		if !pageV1Valid(t, dataHealthPageV1, m) {
			t.Errorf("repair %q refused", v)
		}
	}
	m := dataHealthFixture(t, "ok")
	firstCell(m)["repair"] = "button_until"
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal(`repair "button_until" accepted`)
	}
}

// TestDataHealthOkHasFredRow: the ok fixture has a FRED row with newest, and newest sits only on it.
func TestDataHealthOkHasFredRow(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	fred := false
	for _, d := range m["datasets"].([]any) {
		ds := d.(map[string]any)
		if len(ds["newest"].([]any)) > 0 {
			if ds["dataset_id"] != "obs.series_point" {
				t.Fatalf("newest on non-FRED dataset %v", ds["dataset_id"])
			}
			fred = true
		}
	}
	if !fred {
		t.Fatal("no FRED dataset with a newest row")
	}
}
