package option_candidates_page

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

func legendText(v *tiersView, tier string) string {
	for _, l := range v.Legend {
		if l.Tier == tier {
			return l.Text
		}
	}
	return ""
}

func TestLegendPrintsCurrentValues(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "ok", "90.00", "0.94")}
	res := resultOf(fitsVerdict(keyOf(rows, 0)), greyVerdict(keyOf(rows, 1), "in_the_money"))
	res.Params = optiontierapi.Params{DeltaMax: "0.25", ThinDelta: "0.12", MaxCashAgeDays: 3, DaysInYear: 365}
	c := tierConfig(rows, &fakeTierer{res: res})
	c.Owner = fakeOwner{params: []ownerset.Param{{Key: "put_delta_max", Value: "0.25"}}}
	c.RuleParams = fakeRules{}
	c.Account, c.Aliases = &fakeAccount{values: cashAt("A", "2026-11-06", "5000.00")}, []string{"A"}
	view := buildTiersOf(t, c)
	if len(view.Legend) != 4 {
		t.Fatalf("legend = %+v", view.Legend)
	}
	fits := legendText(view, "fits")
	for _, want := range []string{"|delta| 不高于 0.25、现金够付全额行权价"} {
		if !strings.Contains(fits, want) {
			t.Errorf("fits text %q lacks %q", fits, want)
		}
	}
	if !strings.HasPrefix(view.Legend[0].Title, "✓ 适合 1") || view.Legend[2].Title != "– 未判定 1" {
		t.Errorf("titles = %+v", view.Legend)
	}
	cash := legendText(view, "cash")
	if strings.Contains(cash, "2026-11-06") || strings.Contains(cash, "3 天") {
		t.Errorf("cash text = %q", cash)
	}
}

func TestLegendDefaultBound(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)))}))
	if fits := legendText(view, "fits"); !strings.Contains(fits, "不高于 0.3、") || strings.Contains(fits, "默认值") {
		t.Errorf("fits text = %q", fits)
	}
}

func TestLegendCashClause(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	// the judge says the cash figure is built only when an account was passed
	judge := func(built bool) *fakeTierer {
		return &fakeTierer{fn: func(in optiontierapi.Input) optiontierapi.Result {
			r := resultOf(fitsVerdict(keyOf(rows, 0)))
			r.CashBuilt = built && len(in.Accounts) > 0
			return r
		}}
	}
	built := tierConfig(rows, judge(true))
	built.Account, built.Aliases = &fakeAccount{values: cashAt("A", "2026-11-09", "1.00")}, []string{"A"}
	v := buildTiersOf(t, built)
	if fits := legendText(v, "fits"); !strings.Contains(fits, "现金够付全额行权价") || strings.Contains(fits, "只做标记") {
		t.Errorf("built: fits text %q", fits)
	}
	if cash := legendText(v, "cash"); !strings.Contains(cash, "单独对照一个账户") {
		t.Errorf("built: cash text %q", cash)
	}

	f := judge(true)
	none := tierConfig(rows, f)
	none.Aliases = []string{"A"} // no Account reader
	v = buildTiersOf(t, none)
	if len(f.in[0].Accounts) != 0 {
		t.Fatalf("accounts passed without a reader: %+v", f.in[0].Accounts)
	}
	if fits := legendText(v, "fits"); !strings.Contains(fits, "现金只做标记") || strings.Contains(fits, "现金够付") {
		t.Errorf("mark: fits text %q", fits)
	}
	if cash := legendText(v, "cash"); !strings.Contains(cash, "cash_unknown") {
		t.Errorf("mark: cash text %q", cash)
	}
}

func TestLegendTextsShort(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	judge := func(built bool) *fakeTierer {
		return &fakeTierer{fn: func(in optiontierapi.Input) optiontierapi.Result {
			r := resultOf(fitsVerdict(keyOf(rows, 0)))
			r.Params = optiontierapi.Params{DeltaMax: "0.25", ThinDelta: "0.12", MaxCashAgeDays: 3, DaysInYear: 365}
			r.CashBuilt = built && len(in.Accounts) > 0
			return r
		}}
	}
	const check = "买入价、价外两项都满足，但有需要再看一眼的徽标。"
	const grey = "没有买入价等原因，所以没有评判；已折叠，点开可看每一行为什么在这里。"
	const cash = "每一行单独对照一个账户的可用现金，不在各行之间扣减，也不替你选账户。"
	built := tierConfig(rows, judge(true))
	built.Account, built.Aliases = &fakeAccount{values: cashAt("A", "2026-11-09", "1.00")}, []string{"A"}
	v := buildTiersOf(t, built)
	want := map[string]string{
		"fits":  "满足你的全部条件：有买入价、价外、|delta| 不高于 0.25、现金够付全额行权价。这不是买入建议。",
		"check": check, "grey": grey, "cash": cash,
	}
	for _, l := range v.Legend {
		if l.Text != want[l.Tier] {
			t.Errorf("built %s text = %q, want %q", l.Tier, l.Text, want[l.Tier])
		}
	}
	none := tierConfig(rows, judge(true))
	none.Aliases = []string{"A"}
	v = buildTiersOf(t, none)
	if got, w := legendText(v, "fits"), "满足你的全部条件：有买入价、价外、|delta| 不高于 0.25、现金只做标记。这不是买入建议。"; got != w {
		t.Errorf("unbuilt fits = %q, want %q", got, w)
	}
	if got := legendText(v, "cash"); got != cashNoFigure {
		t.Errorf("unbuilt cash = %q, want %q", got, cashNoFigure)
	}
}
