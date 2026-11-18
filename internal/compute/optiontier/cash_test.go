package optiontier

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

func TestCashNetOfCommittedStrikes(t *testing.T) {
	t.Parallel()
	f := load(t)
	// main: 120000.00 less 90.00 x 100 x 10 = 90000.00 committed, so 30000.00 is free.
	big := one(t, f, func(r *optiontierapi.Row) { r.Strike = "350.00"; r.Spot = "360.00"; r.BuyPrice = "350.00" }, "main")
	if big.Tier != "check" || !has(big.Badges, "cash_short") || big.FitAlias != "" {
		t.Errorf("strike 350.00 (needs 35000.00): %+v", big)
	}
	c := big.Cash[0]
	if c.Verdict != "short" || c.Free != "30000" || c.Committed != "90000" {
		t.Errorf("cell = %+v", c)
	}
	ok := one(t, f, func(r *optiontierapi.Row) { r.Strike = "290.00"; r.Spot = "300.00"; r.BuyPrice = "290.00" }, "main")
	if ok.Tier != "fits" || ok.FitAlias != "main" || ok.Cash[0].Verdict != "fits" {
		t.Errorf("strike 290.00 (needs 29000.00): %+v", ok)
	}
}

func TestCashAliasUnknownReasons(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, nil, "empty", "eur", "stale", "foreign", "small")
	want := []struct{ alias, verdict, why string }{
		{"empty", "unknown", "no_cash"}, {"eur", "unknown", "not_usd"}, {"stale", "unknown", "stale_cash"},
		{"foreign", "unknown", "other_currency"}, {"small", "short", ""},
	}
	for i, w := range want {
		c := v.Cash[i]
		if c.Alias != w.alias || c.Verdict != w.verdict || c.Why != w.why {
			t.Errorf("cell %d = %+v, want %+v", i, c, w)
		}
	}
}

func TestCashNotBuiltIsMark(t *testing.T) {
	t.Parallel()
	f := load(t)
	res := judge(t, f, []optiontierapi.Row{f.Base}, "empty", "eur")
	if res.CashBuilt {
		t.Error("CashBuilt = true")
	}
	v := res.Verdicts[0]
	if v.Tier != "fits" || has(v.Badges, "cash_short") || !has(v.Infos, "cash_unknown") {
		t.Errorf("verdict = %+v", v)
	}
	if res := judge(t, f, []optiontierapi.Row{f.Base}); res.CashBuilt || res.Verdicts[0].Tier != "fits" {
		t.Errorf("no accounts: %+v", res)
	}
	if res := judge(t, f, []optiontierapi.Row{f.Base}, "empty", "main"); !res.CashBuilt || has(res.Verdicts[0].Infos, "cash_unknown") {
		t.Errorf("one known alias: %+v", res)
	}
}

func TestCashShortIsCheck(t *testing.T) {
	t.Parallel()
	f := load(t)
	// small has 5000.00 free; strike 90.00 needs 9000.00.
	v := one(t, f, nil, "small")
	if v.Tier != "check" || !has(v.Badges, "cash_short") {
		t.Errorf("verdict = %+v", v)
	}
}

func TestFitAliasFirstFitting(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, nil, "small", "main", "empty")
	if v.FitAlias != "main" || v.Tier != "fits" {
		t.Errorf("verdict = %+v", v)
	}
	v = one(t, f, nil, "main", "small")
	if v.FitAlias != "main" {
		t.Errorf("FitAlias = %q", v.FitAlias)
	}
}
