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

// TestStrategyReplayPageFixtureShapes: form has six default fields, report has an under-minimum
// line with a null figure, empty has no lines and an empty_text, off has no form and form_off.
func TestStrategyReplayPageFixtureShapes(t *testing.T) {
	t.Parallel()
	load := func(n string) map[string]any {
		return readRaw(t, optionPageFixturesDir, "page.strategy_replay_page.v1."+n+".json")
	}
	f := load("form")
	if f["view"] != "form" {
		t.Errorf("form view = %v", f["view"])
	}
	var names []string
	for _, x := range f["form"].(map[string]any)["fields"].([]any) {
		names = append(names, x.(map[string]any)["name"].(string))
	}
	want := []string{"lots", "alias", "as_of", "early_keep", "early_dte", "min_lots"}
	if len(names) != len(want) {
		t.Fatalf("fields = %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("field %d = %s, want %s", i, names[i], want[i])
		}
	}
	r := load("report")
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
	e := load("empty")["report"].(map[string]any)
	if e["counts"].(map[string]any)["lots"] != float64(0) || len(e["lines"].([]any)) != 0 || e["empty_text"] == nil {
		t.Errorf("empty report = %v", e)
	}
	o := load("off")
	n := o["notices"].([]any)
	if o["view"] != "form" || o["form"] != nil || len(n) != 1 || n[0].(map[string]any)["key"] != "form_off" {
		t.Errorf("off = %v", o)
	}
}
