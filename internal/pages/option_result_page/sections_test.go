package option_result_page_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
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
		"Realized": "$109.50", "Premium": "$200.00", "Retained ratio": "75.0%",
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
	if v := figureValues(secs[2]); v["Retained ratio"] != "100.0%" {
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
	if !strings.Contains(a, "$120.00") || !strings.Contains(a, "100") || strings.Contains(a, "-") {
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
	if n := strs(secs[0]["figure_notes"]); len(n) != 0 {
		t.Errorf("sold_usd notes = %v", n)
	}
	n := strings.Join(strs(secs[1]["figure_notes"]), "|")
	if !strings.Contains(n, "Retained ratio") {
		t.Errorf("bought_usd notes %q miss Retained ratio", n)
	}
}

func TestNoEmptyNoteLines(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	for _, s := range sectionsOf(buildMap(t, c, "main")) {
		for _, n := range strs(s["figure_notes"]) {
			if strings.Contains(n, "is empty") {
				t.Errorf("%v note %q", s["key"], n)
			}
		}
	}
}

func TestMoneyTextGroupsAndSigns(t *testing.T) {
	t.Parallel()
	cases := []struct{ cur, in, want string }{
		{"USD", "49112.0000", "$49,112.00"},
		{"JPY", "881660.0000", "¥881,660.00"},
		{"USD", "-1583.3712", "−$1,583.37"},
		{"USD", "-11587.9212", "−$11,587.92"},
		{"USD", "0.0000", "$0.00"},
		{"USD", "-0.0012", "$0.00"},
		{"USD", "0.005", "$0.01"},
		{"USD", "-0.005", "−$0.01"},
		{"USD", "1234567.894", "$1,234,567.89"},
		{"EUR", "1234.5", "EUR 1,234.50"},
		{"USD", "n/a", "n/a"},
	}
	for _, c := range cases {
		cfg, _ := testConfig(t, "result_ok.json")
		cfg.Results = func() compute.OptionResults {
			return fakeResults{sum: compute.ResultSummary{Sections: []compute.ResultSection{
				{Side: "sold", Currency: c.cur, Realized: c.in, Premium: c.in},
			}}, in: &compute.ResultInput{}}
		}
		v := figureValues(sectionsOf(buildMap(t, cfg, "main"))[0])
		if v["Realized"] != c.want || v["Premium"] != c.want {
			t.Errorf("%s %q: realized %q premium %q, want %q", c.cur, c.in, v["Realized"], v["Premium"], c.want)
		}
	}
}

func TestPercentOneDecimal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ share, want string }{
		{"0.949400", "94.9%"}, {"0.123450", "12.3%"}, {"0.123550", "12.4%"}, {"0.000000", "0.0%"}, {"1", "100.0%"}, {"-0.123450", "−12.3%"},
	} {
		c, _ := testConfig(t, "result_ok.json")
		f := loadFixture(t, "result_ok.json")
		f.Summary.Sections[0].KeptShare = tc.share
		c.Results = func() compute.OptionResults { return fakeResults{sum: f.Summary, in: &compute.ResultInput{}} }
		v := figureValues(sectionsOf(buildMap(t, c, "main"))[0])
		if v["Retained ratio"] != tc.want {
			t.Errorf("%s = %q, want %q", tc.share, v["Retained ratio"], tc.want)
		}
	}
}

func TestSectionLeadNamesTheAccount(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	c.Aliases = []string{"mine"}
	secs := sectionsOf(buildMap(t, c, "mine"))
	if len(secs) == 0 {
		t.Fatal("no sections")
	}
	for _, s := range secs {
		want := s["heading"].(string) + "，账户 mine：已平仓的期权，各币种分别列出，不相加"
		if got := s["lead"].(string); got != want {
			t.Errorf("lead = %q, want %q", got, want)
		}
	}
}

var hanPattern = regexp.MustCompile(`\p{Han}`)

func TestResultSentencesAreChinese(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	f := loadFixture(t, "result_ok.json")
	f.Summary.Sections[0].KeptShare = ""
	c.Results = func() compute.OptionResults { return fakeResults{sum: f.Summary, in: &compute.ResultInput{}} }
	m := buildMap(t, c, "main")
	s := sectionsOf(m)[0]
	if s["assignment"] == nil {
		t.Fatal("no assignment")
	}
	texts := []string{s["lead"].(string), s["books_note"].(string), s["assignment"].(string)}
	texts = append(texts, strs(s["figure_notes"])...)
	texts = append(texts, strs(s["books_lines"])...)
	if len(strs(s["figure_notes"])) == 0 {
		t.Fatal("no figure note")
	}
	rel := m["reliability"].([]any)
	texts = append(texts, strs(rel[0].(map[string]any)["lines"])...)
	acc := strs(rel[len(rel)-1].(map[string]any)["lines"])
	texts = append(texts, acc[1], acc[2])
	old := []string{
		"closed options, each", "Broker figures are taken", "has no value:", "Assigned premium", "trade(s)",
		"Unpaired closes", "Cancels or corrections", "Same as broker", "No broker figure",
	}
	for _, x := range texts {
		if !hanPattern.MatchString(x) {
			t.Errorf("%q holds no Chinese character", x)
		}
		for _, o := range old {
			if strings.Contains(x, o) {
				t.Errorf("%q holds old sentence %q", x, o)
			}
		}
	}
}
