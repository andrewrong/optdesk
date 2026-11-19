package contracts_test

import "testing"

// TestStrategyReplayPageFixturesValidate validates the four fixtures against the schema and
// checks the view each one stands for.
func TestStrategyReplayPageFixturesValidate(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.strategy_replay_page.v1.json")
	for _, name := range []string{"form", "report", "empty", "off"} {
		d := readRaw(t, optionPageFixturesDir, "page.strategy_replay_page.v1."+name+".json")
		if err := s.Validate(d); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func loadStrategyReplay(t *testing.T, n string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.strategy_replay_page.v1."+n+".json")
}

// TestStrategyReplayFormFixture: view form, six fields in order, each with its default.
func TestStrategyReplayFormFixture(t *testing.T) {
	t.Parallel()
	f := loadStrategyReplay(t, "form")
	if f["view"] != "form" {
		t.Errorf("form view = %v", f["view"])
	}
	want := []struct{ name, value string }{
		{"lots", "short_puts"}, {"alias", ""}, {"as_of", ""},
		{"early_keep", "0.50"}, {"early_dte", "21"}, {"min_lots", "20"},
	}
	fields := f["form"].(map[string]any)["fields"].([]any)
	if len(fields) != len(want) {
		t.Fatalf("%d fields, want %d", len(fields), len(want))
	}
	for i, w := range want {
		m := fields[i].(map[string]any)
		if m["name"] != w.name || m["value"] != w.value {
			t.Errorf("field %d = %v/%v, want %s/%q", i, m["name"], m["value"], w.name, w.value)
		}
	}
}

// TestStrategyReplayReportFixture: view report with an under-minimum line carrying minimum_text
// and a null figure.
func TestStrategyReplayReportFixture(t *testing.T) {
	t.Parallel()
	r := loadStrategyReplay(t, "report")
	if r["view"] != "report" {
		t.Errorf("report view = %v", r["view"])
	}
	under, null := false, false
	for _, l := range r["report"].(map[string]any)["lines"].([]any) {
		m := l.(map[string]any)
		if m["under_minimum"] == true && m["minimum_text"] != nil {
			under = true
		}
		if m["mean_gross"] == nil {
			null = true
		}
	}
	if !under || !null {
		t.Errorf("report: under-minimum line %v, null figure %v", under, null)
	}
}

// TestStrategyReplayEmptyFixture: view report, zero lots, no lines, empty_text set.
func TestStrategyReplayEmptyFixture(t *testing.T) {
	t.Parallel()
	ed := loadStrategyReplay(t, "empty")
	if ed["view"] != "report" {
		t.Errorf("empty view = %v", ed["view"])
	}
	e := ed["report"].(map[string]any)
	if e["counts"].(map[string]any)["lots"] != float64(0) || len(e["lines"].([]any)) != 0 || e["empty_text"] == nil {
		t.Errorf("empty report = %v", e)
	}
}

// TestStrategyReplayOffFixture: view form, form null, the one notice form_off.
func TestStrategyReplayOffFixture(t *testing.T) {
	t.Parallel()
	o := loadStrategyReplay(t, "off")
	n := o["notices"].([]any)
	if o["view"] != "form" || o["form"] != nil || len(n) != 1 || n[0].(map[string]any)["key"] != "form_off" {
		t.Errorf("off = %v", o)
	}
}
