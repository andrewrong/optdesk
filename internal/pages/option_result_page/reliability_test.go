package option_result_page_test

import (
	"strings"
	"testing"
)

func TestReliabilityLines(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	m := buildMap(t, c, "main")
	rel := m["reliability"].([]any)
	if len(rel) != 3 {
		t.Fatalf("reliability = %d", len(rel))
	}
	r := rel[0].(map[string]any)
	lines := strs(r["lines"])
	if r["key"] != "sold_usd" || len(lines) != 3 {
		t.Fatalf("entry = %v", r)
	}
	if !strings.Contains(lines[0], "3") || !strings.Contains(lines[1], "1") {
		t.Errorf("lines = %v", lines)
	}
	if !strings.Contains(lines[2], "T9") || !strings.Contains(lines[2], "T7, T8") || !strings.Contains(lines[2], "not in any figure") {
		t.Errorf("last line = %q", lines[2])
	}
	if l := strs(rel[1].(map[string]any)["lines"]); !strings.Contains(l[2], "1 trade(s)") {
		t.Errorf("no broker line = %q", l[2])
	}
}
