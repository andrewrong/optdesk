package contracts_test

import (
	"strings"
	"testing"
)

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

// TestGatewaysFixtureValidates: the gateways fixture passes the schema and holds the gateways main and wife.
func TestGatewaysFixtureValidates(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "gateways")
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("gateways fixture refused")
	}
	gws := m["gateways"].([]any)
	if len(gws) != 2 {
		t.Fatalf("%d gateways, want 2", len(gws))
	}
	g0, g1 := gws[0].(map[string]any), gws[1].(map[string]any)
	if g0["name"] != "main" || g0["verdict"] != "farm_down" || g1["name"] != "wife" || g1["verdict"] != "unreachable" {
		t.Fatalf("gateways %v %v", g0, g1)
	}
}

// TestGatewaysVerdictOutsideEnum: a verdict outside the four keys is refused, each of the four is accepted.
func TestGatewaysVerdictOutsideEnum(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"session_up", "session_retrying", "farm_down", "unreachable"} {
		m := dataHealthFixture(t, "gateways")
		m["gateways"].([]any)[0].(map[string]any)["verdict"] = v
		if !pageV1Valid(t, dataHealthPageV1, m) {
			t.Errorf("verdict %q refused", v)
		}
	}
	m := dataHealthFixture(t, "gateways")
	m["gateways"].([]any)[0].(map[string]any)["verdict"] = "farm_broken"
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal(`verdict "farm_broken" accepted`)
	}
}

func dataCheckFindings(m map[string]any) []any {
	return m["data_check"].(map[string]any)["findings"].([]any)
}

// TestDataCheckFixtureValidates: the datacheck fixture passes and holds one finding of each severity, one new, and no gap.
func TestDataCheckFixtureValidates(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "datacheck")
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("datacheck fixture refused")
	}
	sev, isNew := map[any]bool{}, 0
	for _, f := range dataCheckFindings(m) {
		fm := f.(map[string]any)
		sev[fm["severity"]] = true
		if fm["is_new"] == true {
			isNew++
		}
	}
	if len(sev) != 3 || isNew == 0 || len(m["gaps"].([]any)) != 0 {
		t.Fatalf("severities %v, new %d, gaps %v", sev, isNew, m["gaps"])
	}
}

// TestDataCheckRejectsUnknownField: a finding, the block or the counts with an extra key fails.
func TestDataCheckRejectsUnknownField(t *testing.T) {
	t.Parallel()
	muts := map[string]func(m map[string]any){
		"finding": func(m map[string]any) { dataCheckFindings(m)[0].(map[string]any)["extra"] = 1 },
		"block":   func(m map[string]any) { m["data_check"].(map[string]any)["extra"] = 1 },
		"counts":  func(m map[string]any) { m["data_check"].(map[string]any)["counts"].(map[string]any)["extra"] = 1 },
	}
	for name, mut := range muts {
		m := dataHealthFixture(t, "datacheck")
		mut(m)
		if pageV1Valid(t, dataHealthPageV1, m) {
			t.Errorf("%s with an extra key accepted", name)
		}
	}
}

// TestDataCheckSeverityEnum: the three severities pass and "fatal" fails.
func TestDataCheckSeverityEnum(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"error", "warn", "unknown"} {
		m := dataHealthFixture(t, "datacheck")
		dataCheckFindings(m)[0].(map[string]any)["severity"] = v
		if !pageV1Valid(t, dataHealthPageV1, m) {
			t.Errorf("severity %q refused", v)
		}
	}
	m := dataHealthFixture(t, "datacheck")
	dataCheckFindings(m)[0].(map[string]any)["severity"] = "fatal"
	if pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal(`severity "fatal" accepted`)
	}
}

// TestDataCheckOptionalNullableAndGap: data_check may be absent or null, and the new gap reason passes.
func TestDataCheckOptionalNullableAndGap(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "datacheck")
	m["data_check"] = nil
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Error("null data_check refused")
	}
	delete(m, "data_check")
	m["gaps"] = []any{map[string]any{"kind": "gap", "reason": "data_check_unavailable", "text": "Invented gap text."}}
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Error("absent data_check or data_check_unavailable gap refused")
	}
}

// TestOldFixturesStillValidate: the fixtures written before data_check still pass.
func TestOldFixturesStillValidate(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ok", "gap", "gateways"} {
		if !pageV1Valid(t, dataHealthPageV1, dataHealthFixture(t, name)) {
			t.Errorf("%s refused", name)
		}
	}
}

func gatewaySinceGateway(m map[string]any, i int) map[string]any {
	return m["gateways"].([]any)[i].(map[string]any)
}

// TestGatewaySinceFixtureValidates: the gatewaysince fixture passes; the unreachable gateway has a time and a text, farm_down the two empty strings.
func TestGatewaySinceFixtureValidates(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "gatewaysince")
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("gatewaysince fixture refused")
	}
	for i := range 2 {
		g := gatewaySinceGateway(m, i)
		switch g["verdict"] {
		case "unreachable":
			if g["since"] == "" || !strings.HasPrefix(g["since_text"].(string), "无连接自 ") {
				t.Errorf("unreachable gateway %v", g)
			}
		case "farm_down":
			if g["since"] != "" || g["since_text"] != "" {
				t.Errorf("farm_down gateway %v", g)
			}
		default:
			t.Errorf("verdict %v", g["verdict"])
		}
	}
}

// TestGatewaySinceRejectsBadTime: a since that is neither a date-time nor the empty string fails.
func TestGatewaySinceRejectsBadTime(t *testing.T) {
	t.Parallel()
	for _, bad := range []any{"yesterday", "2026-11-17", 12} {
		m := dataHealthFixture(t, "gatewaysince")
		gatewaySinceGateway(m, 1)["since"] = bad
		if pageV1Valid(t, dataHealthPageV1, m) {
			t.Errorf("since %v accepted", bad)
		}
	}
}

// TestGatewayWithoutSinceStillValidates: a gateway with neither field passes.
func TestGatewayWithoutSinceStillValidates(t *testing.T) {
	t.Parallel()
	m := dataHealthFixture(t, "gatewaysince")
	for i := range 2 {
		g := gatewaySinceGateway(m, i)
		delete(g, "since")
		delete(g, "since_text")
	}
	if !pageV1Valid(t, dataHealthPageV1, m) {
		t.Fatal("gateway without since refused")
	}
}
