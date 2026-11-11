package option_result_page_test

import (
	"strings"
	"testing"
)

func figureValues(s map[string]any) map[string]string {
	out := map[string]string{}
	for _, f := range s["figures"].([]any) {
		m := f.(map[string]any)
		out[m["label"].(string)] = m["value"].(string)
	}
	return out
}

func TestSixFiguresPerSection(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	secs := sectionsOf(buildMap(t, c, "main"))
	if len(secs) != 3 {
		t.Fatalf("sections = %d", len(secs))
	}
	for _, s := range secs {
		if n := len(s["figures"].([]any)); n != 6 {
			t.Errorf("%v has %d figures", s["key"], n)
		}
	}
	var labels []string
	for _, f := range secs[0]["figures"].([]any) {
		labels = append(labels, f.(map[string]any)["label"].(string))
	}
	if strings.Join(labels, ",") != "Realized,Premium,Retained ratio,Closed trades,Assigned trades,Expired trades" {
		t.Errorf("labels = %v", labels)
	}
	v := figureValues(secs[0])
	want := map[string]string{
		"Realized": "109.5000", "Premium": "200.0000", "Retained ratio": "75.0000%",
		"Closed trades": "2", "Assigned trades": "1", "Expired trades": "1",
	}
	for k, w := range want {
		if v[k] != w {
			t.Errorf("%s = %q, want %q", k, v[k], w)
		}
	}
	if v := figureValues(secs[1]); v["Retained ratio"] != "—" || v["Assigned trades"] != "0" || v["Expired trades"] != "0" {
		t.Errorf("bought figures = %v", v)
	}
	if v := figureValues(secs[2]); v["Retained ratio"] != "100.0000%" {
		t.Errorf("jpy kept = %q", v["Retained ratio"])
	}
}

func TestSectionKeys(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	secs := sectionsOf(buildMap(t, c, "main"))
	for i, k := range []string{"sold_usd", "bought_usd", "sold_jpy"} {
		if secs[i]["key"] != k || secs[i]["scope"] != "main" {
			t.Errorf("section %d = %v scope %v", i, secs[i]["key"], secs[i]["scope"])
		}
	}
}

func TestAssignmentText(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	secs := sectionsOf(buildMap(t, c, "main"))
	a, _ := secs[0]["assignment"].(string)
	if !strings.Contains(a, "120.0000") || !strings.Contains(a, "100") || strings.Contains(a, "-") {
		t.Errorf("assignment = %q", a)
	}
	if secs[1]["assignment"] != nil || secs[2]["assignment"] != nil {
		t.Error("assignment set without an assigned ending")
	}
}

func TestProtectionNull(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	for _, s := range sectionsOf(buildMap(t, c, "main")) {
		if s["protection"] != nil {
			t.Errorf("%v protection = %v", s["key"], s["protection"])
		}
	}
}

func TestNotesNameNulls(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	secs := sectionsOf(buildMap(t, c, "main"))
	if n := strs(secs[0]["figure_notes"]); len(n) != 1 || !strings.Contains(n[0], "Protection") {
		t.Errorf("sold_usd notes = %v", n)
	}
	n := strings.Join(strs(secs[1]["figure_notes"]), "|")
	for _, w := range []string{"Retained ratio", "Assigned", "Protection"} {
		if !strings.Contains(n, w) {
			t.Errorf("bought_usd notes %q miss %q", n, w)
		}
	}
}
