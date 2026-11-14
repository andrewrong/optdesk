package option_candidates_page

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestValueIsNumberLiteral(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(numCell("95.00"))
	if err != nil || string(b) != `{"text":"95.00","value":95.00,"badges":[],"tone":"none"}` {
		t.Fatalf("numCell = %s, %v", b, err)
	}
	if b, _ = json.Marshal(numCellPtr(nil)); string(b) != `{"text":"—","value":null,"badges":[],"tone":"none"}` {
		t.Errorf("nil cell = %s", b)
	}
	for _, bad := range []string{"1e-2", "abc", "007", "", "1."} {
		if g, ok := numCell(bad).(gap); !ok || g.Reason != "unreadable" {
			t.Errorf("numCell(%q) = %+v", bad, numCell(bad))
		}
	}
	// the yield keeps the stored text as the value
	if b, _ = json.Marshal(yieldCell(strp("0.050000"))); string(b) != `{"text":"5.0%","value":0.050000,"badges":[],"tone":"none"}` {
		t.Errorf("yield cell = %s", b)
	}
}

func TestOptionsPageFormats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, text, value string
		cell              any
	}{
		{"yield", "11.8%", "0.118212", yieldCell(strp("0.118212"))},
		{"yield", "144.4%", "1.444379", yieldCell(strp("1.444379"))},
		{"delta", "\u22120.56", "-0.5594", deltaCell(strp("-0.5594"))},
		{"delta", "0.30", "0.3", deltaCell(strp("0.3"))},
		{"iv", "69.6%", "0.696", ivCell(strp("0.696"))},
		{"iv", "50.9%", "0.5086", ivCell(strp("0.5086"))},
		{"bid", "$65.65", "65.65", moneyCell("65.65")},
		{"break-even", "$1,105.75", "1105.75", moneyCell("1105.75")},
		{"break-even", "$1,100.65", "1100.65", moneyCell("1100.65")},
	}
	for _, c := range cases {
		vc, ok := c.cell.(valueCell)
		if !ok || vc.Text != c.text || vc.Value == nil || string(*vc.Value) != c.value {
			t.Errorf("%s %q: cell = %+v", c.name, c.text, c.cell)
		}
	}
	for _, f := range []func(*string) any{deltaCell, ivCell} {
		if vc, ok := f(nil).(valueCell); !ok || vc.Text != "—" || vc.Value != nil {
			t.Errorf("nil cell = %+v", f(nil))
		}
	}
}

func TestFitsPerAlias(t *testing.T) {
	t.Parallel()
	c := cand("short_put", 1, "ok")
	c.Fits = `{"B":"no_fit","A":"fit","C":"unknown"}`
	cell := fitCell(c).(valueCell)
	if want := []string{"B Not enough cash", "A Enough cash", "C Unknown"}; !reflect.DeepEqual(cell.Badges, want) {
		t.Errorf("badges = %v, want the stored key order %v", cell.Badges, want)
	}
	c.Fits = "{}"
	if cell = fitCell(c).(valueCell); len(cell.Badges) != 0 || cell.Text != "—" {
		t.Errorf("empty fits = %+v", cell)
	}
	for _, bad := range []string{"", "[]", `{"A":1}`, `{"A":"fit"`, `{} {}`} {
		c.Fits = bad
		if _, ok := fitCell(c).(gap); !ok {
			t.Errorf("fits %q is not a gap cell", bad)
		}
	}
}

func TestMarksAreBadges(t *testing.T) {
	t.Parallel()
	c := cand("short_put", 1, "ok")
	c.Marks = `["itm","earnings_unconfirmed","earnings_beyond_horizon","no_dividend","future_mark"]`
	cell := marksCell(c).(valueCell)
	want := []string{
		"itm in the money", "earnings_unconfirmed earnings date not confirmed", "earnings_beyond_horizon earnings after the contract",
		"no_dividend no dividend", "future_mark",
	}
	if !reflect.DeepEqual(cell.Badges, want) {
		t.Errorf("badges = %v", cell.Badges)
	}
	c.Marks = "oops"
	if _, ok := marksCell(c).(gap); !ok {
		t.Error("bad marks is not a gap cell")
	}
}
