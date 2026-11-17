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

// TestDataHealthOkGapDaysCountsButtons: gap_days is the number of missing or expired cells with a button.
func TestDataHealthOkGapDaysCountsButtons(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	for _, d := range m["datasets"].([]any) {
		ds := d.(map[string]any)
		n := 0
		for _, c := range ds["cells"].([]any) {
			cell := c.(map[string]any)
			s := cell["state"]
			if (s == "missing" || s == "expired") && cell["repair"] == "button" {
				n++
			}
		}
		if int(ds["gap_days"].(float64)) != n {
			t.Fatalf("%v: gap_days %v, button gap cells %d", ds["dataset_id"], ds["gap_days"], n)
		}
	}
}

// TestDataHealthOkOneDatasetHasEveryState: one dataset of the ok fixture has every state and every repair value.
func TestDataHealthOkOneDatasetHasEveryState(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	for _, d := range m["datasets"].([]any) {
		states, repairs := map[any]bool{}, map[any]bool{}
		for _, c := range d.(map[string]any)["cells"].([]any) {
			cell := c.(map[string]any)
			states[cell["state"]], repairs[cell["repair"]] = true, true
		}
		if len(states) == 5 && len(repairs) == 3 {
			return
		}
	}
	t.Fatal("no dataset has all five states and all three repair values")
}
