package optiontier

import "testing"

import "github.com/andrewrong/optdesk/internal/compute/optiontierapi"

func TestFiguresOfFitsRow(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, nil, "main")
	got := map[string]string{
		"MarginPct": v.MarginPct, "BelowBuy": v.BelowBuy, "BreakEven": v.BreakEven, "Moves": v.Moves,
		"BelowSpotPct": v.BelowSpotPct, "PerContract": v.PerContract, "Intrinsic": v.Intrinsic,
		"TimeValue": v.TimeValue, "YieldPct": v.YieldPct,
	}
	want := map[string]string{
		"MarginPct": "1.0444", "BelowBuy": "0.94", "BreakEven": "89.06", "Moves": "0.9000",
		"BelowSpotPct": "7.7098", "PerContract": "94", "Intrinsic": "0", "TimeValue": "0.94", "YieldPct": "12.2975",
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
}

func TestMoneyFiguresIntrinsicAndTimeValue(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, func(r *optiontierapi.Row) { *r = f.ITM }, "main")
	if v.Grey != "in_the_money" || v.Intrinsic != "3.5" || v.TimeValue != "1.9" {
		t.Fatalf("verdict = %+v", v)
	}
	// The yield is of the time value only: 1.9 / 100.00 x 365 / 31 x 100 = 22.3710.
	if v.YieldPct != "22.3710" {
		t.Errorf("YieldPct = %q, want 22.3710", v.YieldPct)
	}
	if v.PerContract != "540" {
		t.Errorf("PerContract = %q", v.PerContract)
	}
}

func TestEmptyFiguresHaveWhy(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, func(r *optiontierapi.Row) { r.IV = "" }, "main")
	if v.Moves != "" || v.Why["moves"] != "no_iv" || v.Tier != "fits" {
		t.Errorf("no IV: %+v", v)
	}
	v = one(t, f, func(r *optiontierapi.Row) { r.DTE = 0 }, "main")
	if v.YieldPct != "" || v.Why["yield"] != "zero_dte" {
		t.Errorf("zero DTE: %+v", v)
	}
	v = one(t, f, func(r *optiontierapi.Row) { r.BuyPrice = "" }, "main")
	if v.MarginPct != "" || v.Why["margin"] != "no_buy_price" {
		t.Errorf("no buy price: %+v", v)
	}
}
