package optiontier

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

func TestOrderMarginThenYield(t *testing.T) {
	t.Parallel()
	f := load(t)
	mk := func(key, strike, bid, buy string, dte int) optiontierapi.Row {
		r := f.Base
		r.Key, r.Strike, r.Bid, r.BuyPrice, r.DTE = key, strike, bid, buy, dte
		return r
	}
	rows := []optiontierapi.Row{
		mk("low", "90.00", "0.50", "90.00", 31),   // margin 0.5556
		mk("highY", "90.00", "0.94", "90.00", 20), // margin 1.0444, shorter: higher yield
		mk("highM", "90.00", "0.94", "90.00", 31), // margin 1.0444
		mk("tie2", "91.00", "0.94", "91.00", 31),  // margin 1.0330
		mk("tie1", "91.00", "0.94", "91.00", 31),
		mk("grey", "90.00", "", "90.00", 31),
	}
	res := judge(t, f, rows, "main")
	want := []string{"highY", "highM", "tie1", "tie2", "low", "grey"}
	if !reflect.DeepEqual(res.Order, want) {
		t.Fatalf("Order = %v, want %v", res.Order, want)
	}
	places := map[string]int{}
	for _, v := range res.Verdicts {
		places[v.Key] = v.Place
	}
	if places["highY"] != 1 || places["low"] != 5 || places["grey"] != 1 {
		t.Errorf("places = %v", places)
	}
}

func TestOrderTieBreaksDTEStrikeKey(t *testing.T) {
	t.Parallel()
	f := load(t)
	mk := func(key string, dte int, strike string) optiontierapi.Row {
		r := f.Base
		r.Key, r.DTE, r.Strike, r.Bid, r.BuyPrice, r.IV = key, dte, strike, "0", "", ""
		return r
	}
	// Grey rows of one reason go by expiry, strike, key.
	a, b, c := mk("b", 31, "90.00"), mk("a", 31, "90.00"), mk("c", 31, "80.00")
	b.Expiry = "2026-12-18"
	a.Expiry = "2026-12-18"
	c.Expiry = "2026-12-18"
	late := mk("late", 40, "70.00")
	late.Expiry = "2027-01-01"
	res := judge(t, f, []optiontierapi.Row{late, a, b, c}, "main")
	if want := []string{"c", "a", "b", "late"}; !reflect.DeepEqual(res.Order, want) {
		t.Errorf("Order = %v, want %v", res.Order, want)
	}
}

func TestCompaniesCounts(t *testing.T) {
	t.Parallel()
	f := load(t)
	mk := func(key, company string, mod func(*optiontierapi.Row)) optiontierapi.Row {
		r := f.Base
		r.Key, r.Company = key, company
		mod(&r)
		return r
	}
	rows := []optiontierapi.Row{
		mk("1", "BBB", func(*optiontierapi.Row) {}),
		mk("2", "AAA", func(r *optiontierapi.Row) { r.Delta = "" }),
		mk("3", "BBB", func(r *optiontierapi.Row) { r.Bid = "" }),
		mk("4", "BBB", func(*optiontierapi.Row) {}),
	}
	res := judge(t, f, rows, "main")
	want := []optiontierapi.CompanyCount{{Company: "BBB", Fits: 2, Grey: 1}, {Company: "AAA", Check: 1}}
	if !reflect.DeepEqual(res.Companies, want) {
		t.Errorf("Companies = %+v, want %+v", res.Companies, want)
	}
}
