package optiontier

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

func TestQuoteReasonsMapToBadges(t *testing.T) {
	t.Parallel()
	f := load(t)
	want := map[string]string{
		"low_open_interest": "low_open_interest", "no_quote": "one_sided_quote", "no_bid": "one_sided_quote",
		"no_ask": "one_sided_quote", "small_bid_size": "one_sided_quote", "wide_spread": "wide_quote",
		"delayed_data": "stale_quote",
	}
	for reason, badge := range want {
		v := one(t, f, func(r *optiontierapi.Row) { r.QuoteReason = reason }, "main")
		if !reflect.DeepEqual(v.Badges, []string{badge}) || v.Tier != "check" {
			t.Errorf("quote %s: %+v, want badge %s", reason, v, badge)
		}
	}
	v := one(t, f, func(r *optiontierapi.Row) { r.EventsReason = "stale_chain" }, "main")
	if !reflect.DeepEqual(v.Badges, []string{"stale_quote"}) {
		t.Errorf("stale_chain: %+v", v.Badges)
	}
	// Fixed order of several badges, whatever the order of the input.
	v = one(t, f, func(r *optiontierapi.Row) {
		r.QuoteReason, r.EventsReason, r.Marks = "wide_spread", "earnings_before_expiry", []string{"no_open_interest"}
		r.Delta = ""
	}, "main")
	if want := []string{"no_open_interest", "wide_quote", "earnings_before_expiry", "delta_unknown"}; !reflect.DeepEqual(v.Badges, want) {
		t.Errorf("order = %v, want %v", v.Badges, want)
	}
}

func TestEarningsBeforeExpiryBadge(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, func(r *optiontierapi.Row) { r.EventsReason = "earnings_before_expiry" }, "main")
	if v.Tier != "check" || !has(v.Badges, "earnings_before_expiry") {
		t.Errorf("before expiry: %+v", v)
	}
	for _, reason := range []string{"earnings_unconfirmed", "beyond_horizon", "no_earnings", "stale_earnings", "earnings_unsupported", "earnings_error"} {
		v := one(t, f, func(r *optiontierapi.Row) { r.EventsReason = reason }, "main")
		if v.Tier != "fits" || len(v.Badges) != 0 {
			t.Errorf("%s: %+v", reason, v)
		}
	}
}

func twoRows(f fixture, mod func(*optiontierapi.Row)) []optiontierapi.Row {
	a, b := f.Base, f.Base
	b.Key = "b"
	b.Strike = "85.00"
	mod(&a)
	mod(&b)
	return []optiontierapi.Row{a, b}
}

func TestCommonMarkOnceNeverDemotes(t *testing.T) {
	t.Parallel()
	f := load(t)
	res := judge(t, f, twoRows(f, func(r *optiontierapi.Row) { r.Marks = []string{"earnings_unconfirmed"} }), "main")
	if !reflect.DeepEqual(res.Common, []string{"earnings_unconfirmed"}) {
		t.Errorf("Common = %v", res.Common)
	}
	for _, v := range res.Verdicts {
		if v.Tier != "fits" || has(v.Infos, "earnings_unconfirmed") {
			t.Errorf("row %s: %+v", v.Key, v)
		}
	}
	// Carried by one row only: not common, kept in that row's Infos.
	rows := twoRows(f, func(*optiontierapi.Row) {})
	rows[0].Marks = []string{"earnings_unconfirmed"}
	res = judge(t, f, rows, "main")
	if len(res.Common) != 0 || !has(res.Verdicts[0].Infos, "earnings_unconfirmed") {
		t.Errorf("one row: common %v, infos %v", res.Common, res.Verdicts[0].Infos)
	}
}

func TestKeptLookAtEveryRow(t *testing.T) {
	t.Parallel()
	f := load(t)
	res := judge(t, f, twoRows(f, func(r *optiontierapi.Row) { r.Marks = []string{"no_open_interest"} }), "main")
	if len(res.Common) != 0 {
		t.Errorf("Common = %v", res.Common)
	}
	want := []optiontierapi.Kept{{Code: "no_open_interest", N: 2, Every: true}}
	if !reflect.DeepEqual(res.Kept, want) {
		t.Errorf("Kept = %+v, want %+v", res.Kept, want)
	}
	for _, v := range res.Verdicts {
		if v.Tier == "fits" {
			t.Errorf("row %s is fits", v.Key)
		}
	}
}

func TestKeptCountsGreyReasons(t *testing.T) {
	t.Parallel()
	f := load(t)
	mk := func(key string, mod func(*optiontierapi.Row)) optiontierapi.Row {
		r := f.Base
		r.Key = key
		mod(&r)
		return r
	}
	rows := []optiontierapi.Row{
		mk("1", func(r *optiontierapi.Row) { r.Bid = "" }),
		mk("2", func(r *optiontierapi.Row) { r.Bid = "0" }),
		mk("3", func(r *optiontierapi.Row) { r.BuyPrice = "" }),
		mk("4", func(r *optiontierapi.Row) { r.Marks = []string{"no_open_interest"} }),
		mk("5", func(r *optiontierapi.Row) { r.Marks = []string{"no_open_interest"}; r.QuoteReason = "wide_spread" }),
		mk("6", func(*optiontierapi.Row) {}),
	}
	res := judge(t, f, rows, "main")
	want := []optiontierapi.Kept{
		{Code: "no_buy_price", N: 1}, {Code: "no_bid", N: 2},
		{Code: "no_open_interest", N: 2}, {Code: "wide_quote", N: 1},
	}
	if !reflect.DeepEqual(res.Kept, want) {
		t.Errorf("Kept = %+v, want %+v", res.Kept, want)
	}
}
