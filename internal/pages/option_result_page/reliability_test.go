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
	if len(rel) != 4 {
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
	if l := strs(rel[1].(map[string]any)["lines"]); !strings.Contains(l[2], "1 trade(s)") {
		t.Errorf("no broker line = %q", l[2])
	}
}

func TestIdListsOnceForThreeSections(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	m := buildMap(t, c, "main")
	rel := m["reliability"].([]any)
	if len(sectionsOf(m)) != 3 {
		t.Fatalf("sections = %d", len(sectionsOf(m)))
	}
	all := ""
	for i, e := range rel {
		all += strings.Join(strs(e.(map[string]any)["lines"]), "\n") + "\n"
		if i < 3 {
			if l := strings.Join(strs(e.(map[string]any)["lines"]), "|"); strings.Contains(l, "T9") || strings.Contains(l, "T7") {
				t.Errorf("section block %d names ids: %q", i, l)
			}
		}
	}
	for _, id := range []string{"T9", "T7", "T8"} {
		if n := strings.Count(all, id); n != 1 {
			t.Errorf("%s written %d times", id, n)
		}
	}
	acc := strs(rel[3].(map[string]any)["lines"])
	if !strings.Contains(acc[1], "T9") || !strings.Contains(acc[2], "T7, T8") || !strings.Contains(acc[2], "not in any figure") {
		t.Errorf("account block = %v", acc)
	}
	if !strings.Contains(acc[0], "Same") && !strings.Contains(acc[0], "一致") {
		t.Errorf("gloss = %q", acc[0])
	}
}
