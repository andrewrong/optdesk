package option_result_page_test

import (
	"github.com/andrewrong/optdesk/internal/compute"
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
	if len(p) != 2 || p[0]["label"] != "2026-09" || p[0]["value"] != 150.0 || p[1]["value"] != -40.5 || p[1]["text"] != "−$40.50" {
		t.Errorf("months = %v", p)
	}
	if s["summary"] != "2026-09 $150.00; 2026-10 −$40.50" || s["empty"] != nil {
		t.Errorf("summary = %v empty = %v", s["summary"], s["empty"])
	}
}

func TestRunningSeries(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	p := points(seriesOf(sectionsOf(buildMap(t, c, "main"))[0], "running"))
	if len(p) != 2 || p[0]["value"] != 150.0 || p[1]["value"] != 109.5 || p[1]["text"] != "$109.50" {
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

func TestEndingsTextIsMoney(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	s := sectionsOf(buildMap(t, c, "main"))[0]
	p := points(seriesOf(s, "endings"))
	if p[2]["text"] != "1 笔，realized −$50.50" || p[0]["text"] != "2 笔，realized $60.00" {
		t.Errorf("endings points = %v", p)
	}
	lines := strs(s["endings_lines"])
	if len(lines) != 3 || lines[2] != "assigned: 1 笔，realized −$50.50" {
		t.Errorf("lines = %v", lines)
	}
}

func TestEmptySeriesTextsAreChinese(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	f := loadFixture(t, "result_ok.json")
	f.Summary.Sections[0].Months = nil
	f.Summary.Sections[0].Endings = nil
	c.Results = func() compute.OptionResults { return fakeResults{sum: f.Summary, in: &compute.ResultInput{}} }
	s := sectionsOf(buildMap(t, c, "main"))[0]
	for key, want := range map[string]string{"months": "没有已平仓的月份", "running": "没有累计结果", "endings": "没有已结束的头寸"} {
		if got := s[key].(map[string]any)["empty"]; got != want {
			t.Errorf("%s empty = %v, want %q", key, got, want)
		}
	}
}
