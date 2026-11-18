package optiontier

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

func TestGreyPrecedence(t *testing.T) {
	t.Parallel()
	f := load(t)
	cases := []struct {
		name string
		mod  func(*optiontierapi.Row)
		want string
	}{
		{"no buy price and in the money", func(r *optiontierapi.Row) { r.BuyPrice = ""; r.Strike = "100.00" }, "no_buy_price"},
		{"zero buy price", func(r *optiontierapi.Row) { r.BuyPrice = "0" }, "no_buy_price"},
		{"no spot beats in the money", func(r *optiontierapi.Row) { r.Spot = ""; r.Bid = "" }, "no_spot"},
		{"in the money beats no bid", func(r *optiontierapi.Row) { r.Strike = "100.00"; r.Bid = "" }, "in_the_money"},
		{"empty bid", func(r *optiontierapi.Row) { r.Bid = "" }, "no_bid"},
		{"zero bid", func(r *optiontierapi.Row) { r.Bid = "0.00" }, "no_bid"},
		{"above buy price", func(r *optiontierapi.Row) { r.BuyPrice = "80.00" }, "above_buy_price"},
		{"unreadable gates mark", func(r *optiontierapi.Row) { r.Marks = []string{"unreadable_gates"} }, "bad_input"},
		{"no bid beats unreadable gates", func(r *optiontierapi.Row) { r.Marks = []string{"unreadable_gates"}; r.Bid = "" }, "no_bid"},
	}
	for _, c := range cases {
		v := one(t, f, c.mod, "main")
		if v.Tier != "grey" || v.Grey != c.want {
			t.Errorf("%s: tier %q grey %q, want grey %q", c.name, v.Tier, v.Grey, c.want)
		}
	}
}

func TestEvaluatorBadInputIsGrey(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, func(r *optiontierapi.Row) { r.Strike = strikeBadInput }, "main")
	if v.Tier != "grey" || v.Grey != "bad_input" {
		t.Errorf("verdict = %+v", v)
	}
}

func TestEvaluatorOtherErrorReturned(t *testing.T) {
	t.Parallel()
	f := load(t)
	r := f.Base
	r.Strike = strikeBoom
	_, err := New(&fakeEvaluator{}).Judge(optiontierapi.Input{Params: f.Params, Rows: []optiontierapi.Row{r}})
	if !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want errBoom", err)
	}
}

func TestEvaluateSkippedForEarlyGrey(t *testing.T) {
	t.Parallel()
	f := load(t)
	mods := []func(*optiontierapi.Row){
		func(r *optiontierapi.Row) { r.BuyPrice = "" },
		func(r *optiontierapi.Row) { r.Spot = "" },
		func(r *optiontierapi.Row) { r.Bid = "" },
	}
	for i, m := range mods {
		r := f.Base
		m(&r)
		ev := &fakeEvaluator{}
		if _, err := New(ev).Judge(optiontierapi.Input{Params: f.Params, Rows: []optiontierapi.Row{r}}); err != nil {
			t.Fatal(err)
		}
		if ev.calls != 0 {
			t.Errorf("case %d: Evaluate called %d times", i, ev.calls)
		}
	}
}

func TestEveryRowInOrder(t *testing.T) {
	t.Parallel()
	f := load(t)
	mk := func(key string, mod func(*optiontierapi.Row)) optiontierapi.Row {
		r := f.Base
		r.Key = key
		if mod != nil {
			mod(&r)
		}
		return r
	}
	rows := []optiontierapi.Row{
		mk("bid", func(r *optiontierapi.Row) { r.Bid = "" }),
		mk("fit", nil),
		mk("nobuy", func(r *optiontierapi.Row) { r.BuyPrice = "" }),
		mk("chk", func(r *optiontierapi.Row) { r.Delta = "" }),
		mk("itm", func(r *optiontierapi.Row) { r.Strike = "100.00" }),
	}
	res := judge(t, f, rows, "main")
	if len(res.Verdicts) != len(rows) {
		t.Fatalf("verdicts %d, rows %d", len(res.Verdicts), len(rows))
	}
	for i, v := range res.Verdicts {
		if v.Key != rows[i].Key {
			t.Errorf("verdict %d key %q, want input order %q", i, v.Key, rows[i].Key)
		}
	}
	want := []string{"fit", "chk", "nobuy", "itm", "bid"}
	if !reflect.DeepEqual(res.Order, want) {
		t.Errorf("Order = %v, want %v", res.Order, want)
	}
}
