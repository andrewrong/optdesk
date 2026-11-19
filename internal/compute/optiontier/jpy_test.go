package optiontier

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

type jpyFixture struct {
	Params optiontierapi.Params
	JPY    optiontierapi.Row
	USD    optiontierapi.Row
	Expect map[string]struct {
		Tier   string
		Badges []string
	}
}

func loadJPY(t *testing.T) jpyFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/jpy_rows.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var f jpyFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func judgeJPY(t *testing.T, f jpyFixture, r optiontierapi.Row, accts ...string) optiontierapi.Verdict {
	t.Helper()
	return judge(t, fixture{Params: f.Params}, []optiontierapi.Row{r}, accts...).Verdicts[0]
}

func TestJPYMarkNeverFits(t *testing.T) {
	t.Parallel()
	f := loadJPY(t)
	cases := map[string][]string{
		"mark_rate": {"no_jpy_rate"}, "mark_funds": {"no_jpy_funds"},
		"mark_both": {"no_jpy_rate", "no_jpy_funds"}, "no_mark": nil,
	}
	for name, marks := range cases {
		r := f.JPY
		r.Marks = marks
		v := judgeJPY(t, f, r)
		want := f.Expect[name]
		if v.Tier != want.Tier || !reflect.DeepEqual(v.Badges, want.Badges) {
			t.Errorf("%s: tier %q badges %v, want %q %v", name, v.Tier, v.Badges, want.Tier, want.Badges)
		}
	}
	// The mark is the only difference, also for a row with the currency empty.
	r := f.USD
	r.Marks = []string{"no_jpy_rate"}
	if v := judgeJPY(t, f, r); v.Tier != "check" || !has(v.Badges, "no_jpy_rate") {
		t.Errorf("marked US row: %+v", v)
	}
}

func TestJPYNoBuyPriceGrey(t *testing.T) {
	t.Parallel()
	f := loadJPY(t)
	r := f.JPY
	r.BuyPrice = ""
	r.Marks = []string{"no_jpy_rate", "no_jpy_funds"}
	if v := judgeJPY(t, f, r); v.Tier != "grey" || v.Grey != "no_buy_price" {
		t.Errorf("JPY no buy price: %+v", v)
	}
}

func TestJPYCashOtherCurrency(t *testing.T) {
	t.Parallel()
	f := loadJPY(t)
	// main and small hold USD; neither may be compared with a yen strike (a comparison would say short).
	v := judgeJPY(t, f, f.JPY, "main", "small", "empty")
	want := []struct{ alias, why string }{{"main", "other_currency"}, {"small", "other_currency"}, {"empty", "no_cash"}}
	for i, w := range want {
		c := v.Cash[i]
		if c.Alias != w.alias || c.Verdict != "unknown" || c.Why != w.why || c.Free != "" || c.Committed != "" {
			t.Errorf("cell %d = %+v, want unknown %s", i, c, w.why)
		}
	}
	if v.FitAlias != "" || has(v.Badges, "cash_short") || v.Tier != "fits" {
		t.Errorf("verdict = %+v", v)
	}
	// A USD row is compared as before.
	if u := judgeJPY(t, f, f.USD, "main"); u.Cash[0].Verdict != "fits" {
		t.Errorf("USD cell = %+v", u.Cash[0])
	}
}
