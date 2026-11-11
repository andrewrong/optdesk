package option_result_page_test

import (
	"strings"
	"testing"
)

func seriesOf(s map[string]any, name string) map[string]any { return s[name].(map[string]any) }

func points(sr map[string]any) []map[string]any {
	var out []map[string]any
	for _, p := range sr["points"].([]any) {
		out = append(out, p.(map[string]any))
	}
	return out
}

func TestMonthsSeries(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	s := seriesOf(sectionsOf(buildMap(t, c, "main"))[0], "months")
	p := points(s)
	if len(p) != 2 || p[0]["label"] != "2026-09" || p[0]["value"] != 150.0 || p[1]["value"] != -40.5 || p[1]["text"] != "-40.5000" {
		t.Errorf("months = %v", p)
	}
	if s["summary"] != "2026-09 150.0000; 2026-10 -40.5000" || s["empty"] != nil {
		t.Errorf("summary = %v empty = %v", s["summary"], s["empty"])
	}
}

func TestRunningSeries(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	p := points(seriesOf(sectionsOf(buildMap(t, c, "main"))[0], "running"))
	if len(p) != 2 || p[0]["value"] != 150.0 || p[1]["value"] != 109.5 || p[1]["text"] != "109.5000" {
		t.Errorf("running = %v", p)
	}
}

func TestEndingsSeries(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	s := sectionsOf(buildMap(t, c, "main"))[0]
	p := points(seriesOf(s, "endings"))
	var labels []string
	for _, x := range p {
		labels = append(labels, x["label"].(string))
	}
	if strings.Join(labels, ",") != "closed,expired,assigned" || p[2]["value"] != -50.5 || p[0]["muted"] != false {
		t.Errorf("endings = %v", p)
	}
	if lines := strs(s["endings_lines"]); len(lines) != 3 || !strings.HasPrefix(lines[0], "closed") {
		t.Errorf("lines = %v", lines)
	}
}

func TestMutedMonth(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	s := sectionsOf(buildMap(t, c, "main"))[0]
	for _, name := range []string{"months", "running"} {
		p := points(seriesOf(s, name))
		if p[0]["muted"] != false || p[1]["muted"] != true {
			t.Errorf("%s muted = %v %v", name, p[0]["muted"], p[1]["muted"])
		}
	}
}
