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
	if strings.Join(labels, ",") != "已实现,权利金,留存比例,已平仓笔数,被行权笔数,到期笔数" {
		t.Errorf("labels = %v", labels)
	}
	v := figureValues(secs[0])
	want := map[string]string{"已实现": "109.5000", "权利金": "200.0000", "留存比例": "75.0000%", "已平仓笔数": "2", "被行权笔数": "1", "到期笔数": "1"}
	for k, w := range want {
		if v[k] != w {
			t.Errorf("%s = %q, want %q", k, v[k], w)
		}
	}
	if v := figureValues(secs[1]); v["留存比例"] != "—" || v["被行权笔数"] != "0" || v["到期笔数"] != "0" {
		t.Errorf("bought figures = %v", v)
	}
	if v := figureValues(secs[2]); v["留存比例"] != "100.0000%" {
		t.Errorf("jpy kept = %q", v["留存比例"])
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
	if n := strs(secs[0]["figure_notes"]); len(n) != 1 || !strings.Contains(n[0], "保护") {
		t.Errorf("sold_usd notes = %v", n)
	}
	n := strings.Join(strs(secs[1]["figure_notes"]), "|")
	for _, w := range []string{"留存比例", "被行权", "保护"} {
		if !strings.Contains(n, w) {
			t.Errorf("bought_usd notes %q miss %q", n, w)
		}
	}
}
