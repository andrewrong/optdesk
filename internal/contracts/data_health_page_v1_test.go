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

func failureRows(m map[string]any) []any {
	for _, d := range m["datasets"].([]any) {
		if rows, ok := d.(map[string]any)["repeated_failures"].([]any); ok && len(rows) > 0 {
			return rows
		}
	}
	return nil
}

// TestDataHealthOkHasFailedAndEmptyRows: the ok fixture lists one failed row (retryable) and one empty row.
func TestDataHealthOkHasFailedAndEmptyRows(t *testing.T) {
	t.Parallel()
	rows := failureRows(dataHealthFixture(t, "ok"))
	if len(rows) != 2 {
		t.Fatalf("%d repeated_failures rows, want 2", len(rows))
	}
	r0, r1 := rows[0].(map[string]any), rows[1].(map[string]any)
	if r0["kind"] != "failed" || r0["last_class"] != "retryable" || r1["kind"] != "empty" {
		t.Fatalf("rows %v %v", r0, r1)
	}
}

// TestDataHealthGapFixtureFailuresUnavailable: the gap fixture holds the failures_unavailable gap.
func TestDataHealthGapFixtureFailuresUnavailable(t *testing.T) {
	t.Parallel()
	gaps := dataHealthFixture(t, "gap")["gaps"].([]any)
	if len(gaps) != 1 || gaps[0].(map[string]any)["reason"] != "failures_unavailable" {
		t.Fatalf("gaps %v", gaps)
	}
}

// TestDataHealthFailureRowRules: days below 3, runs below 3, a missing class_text, an extra key and a bad kind are refused.
func TestDataHealthFailureRowRules(t *testing.T) {
	t.Parallel()
	cases := map[string]func(r map[string]any){
		"days 2":        func(r map[string]any) { r["days"] = 2 },
		"runs 2":        func(r map[string]any) { r["runs"] = 2 },
		"no class_text": func(r map[string]any) { delete(r, "class_text") },
		"extra":         func(r map[string]any) { r["extra"] = 1 },
		"bad kind":      func(r map[string]any) { r["kind"] = "partial" },
	}
	for name, mut := range cases {
		m := dataHealthFixture(t, "ok")
		mut(failureRows(m)[0].(map[string]any))
		if pageV1Valid(t, dataHealthPageV1, m) {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TestDataHealthFailureFieldsOptional: a document without any of the new fields validates.
func TestDataHealthFailureFieldsOptional(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	delete(m, "failures_note")
	delete(m, "failures_none_text")
	for _, d := range m["datasets"].([]any) {
		ds := d.(map[string]any)
		for _, k := range []string{"failures_total", "failures_hidden", "failures_hidden_text", "repeated_failures"} {
			delete(ds, k)
		}
	}
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("a document without the failure fields was refused")
	}
}

// TestDataHealthRunsBelowDaysRefused: a row with runs below days is refused, and runs equal to days is accepted.
func TestDataHealthRunsBelowDaysRefused(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	r := failureRows(m)[0].(map[string]any)
	r["days"] = 9
	r["runs"] = 3
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("a row with runs below days was accepted")
	}
	r["runs"] = 9
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("a row with runs equal to days was refused")
	}
}

// TestDataHealthFailureDayLength: a day string longer than 10 characters is refused.
func TestDataHealthFailureDayLength(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	failureRows(m)[0].(map[string]any)["last_day"] = "2026-11-17T00:00:00Z"
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("a long last_day was accepted")
	}
}

func dhRunning() map[string]any {
	return map[string]any{
		"job": "statement_capture", "started_at": "2026-11-17T09:20:00Z", "keys_done": 2, "keys_total": 5,
		"step": "invented-key", "elapsed_minutes": 10, "usual_minutes": 8, "slow": true, "line": "running line",
	}
}

// TestDataHealthRunningValidates: a page with running passes, and so does one without.
func TestDataHealthRunningValidates(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	m["running"] = []any{dhRunning(), dhRunning()}
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("page with running refused")
	}
	delete(m, "running")
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("page without running refused")
	}
}

// TestDataHealthRunningNullFails: a null in running, or an unknown key in an item, fails.
func TestDataHealthRunningNullFails(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"job", "started_at", "keys_done", "keys_total", "step", "elapsed_minutes", "usual_minutes", "slow"} {
		m := dataHealthFixture(t, "ok")
		it := dhRunning()
		it[k] = nil
		m["running"] = []any{it}
		if pageV1Valid(t, dataHealthPageV1, m) {
			t.Errorf("running.%s null accepted", k)
		}
	}
	m := dataHealthFixture(t, "ok")
	m["running"] = nil
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Error("running null accepted")
	}
	m = dataHealthFixture(t, "ok")
	it := dhRunning()
	it["extra"] = 1
	m["running"] = []any{it}
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Error("item with an extra key accepted")
	}
}

// TestDataHealthRunningWithoutLineFails: a running item without line is refused.
func TestDataHealthRunningWithoutLineFails(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "ok")
	it := dhRunning()
	delete(it, "line")
	m["running"] = []any{it}
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Error("running item without line accepted")
	}
}
