package spreadlegs_test

import (
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionstruct"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegs"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegsapi"
)

func lg(right, strike, qty, price string) compute.StructureLeg {
	return compute.StructureLeg{PositionID: "p", Right: right, Strike: strike, Expiry: "2026-12-18",
		Quantity: qty, Multiplier: "100", OpenPrice: price}
}

func st(id, kind string, legs ...compute.StructureLeg) compute.Structure {
	return compute.Structure{ID: id, Underlying: "AAA", Currency: "USD", Kind: kind, Legs: legs}
}

func one(s compute.Structure) spreadlegsapi.SpreadFigures {
	return spreadlegs.New().Spreads(spreadlegsapi.SpreadIn{Structures: []compute.Structure{s}, AsOf: "2026-11-21"}).Rows[0]
}

func putSpread() compute.Structure {
	return st("a:1", "put_spread", lg("P", "95", "-2", "2.40"), lg("P", "90", "2", "1.10"))
}

func TestSpreadNetCreditPutSpread(t *testing.T) {
	t.Parallel()
	if got := one(putSpread()).NetCredit; got.Value != "260.00" || got.Gap != "" {
		t.Errorf("put spread net credit = %+v, want 260.00", got)
	}
	debit := st("a:2", "call_debit_spread", lg("C", "100", "1", "3.00"), lg("C", "110", "-1", "1.00"))
	if got := one(debit).NetCredit; got.Value != "-200.00" {
		t.Errorf("call debit spread net credit = %+v, want -200.00", got)
	}
}

func TestSpreadNetCreditGaps(t *testing.T) {
	t.Parallel()
	for _, price := range []string{"", "abc"} {
		s := st("a:1", "put_spread", lg("P", "95", "-1", price), lg("P", "90", "1", "1.10"))
		if got := one(s).NetCredit; got.Gap != "missing_input" || got.Value != "" {
			t.Errorf("open price %q: %+v, want gap missing_input", price, got)
		}
	}
	jpy := putSpread()
	jpy.Currency = "JPY"
	r := one(jpy)
	if r.NetCredit.Gap != "unsupported_currency" || r.NetCredit.Value != "" || r.BreakEvenGap != "unsupported_currency" {
		t.Errorf("JPY: %+v", r)
	}
}

func TestSpreadNetCreditPartLeg(t *testing.T) {
	t.Parallel()
	// the leg quantity is -1 although the position may be larger: the credit is of one lot
	s := st("a:1", "put_spread", lg("P", "95", "-1", "2.40"), lg("P", "90", "1", "1.10"))
	if got := one(s).NetCredit.Value; got != "130.00" {
		t.Errorf("net credit = %s, want 130.00", got)
	}
}

func TestSpreadBreakEvens(t *testing.T) {
	t.Parallel()
	r := one(putSpread())
	if !slices.Equal(r.BreakEvens, []string{"93.7000"}) || r.BreakEvenGap != "" {
		t.Errorf("put spread break-evens = %v gap %q", r.BreakEvens, r.BreakEvenGap)
	}
	// the same input through the matcher gives the same single number
	pos := func(id, right, strike, qty, price string) compute.LedgerPosition {
		return compute.LedgerPosition{PositionID: id, Alias: "acct", Underlying: "AAA", Currency: "USD", Status: "open",
			Right: right, Expiry: "2026-12-18", Strike: strike, Quantity: qty, Multiplier: "100", OpenPrice: price,
			OpenedOn: "2026-01-02", MarkStatus: "no_mark"}
	}
	res := optionstruct.New().Evaluate(compute.StructureInput{Alias: "acct", Positions: []compute.LedgerPosition{
		pos("s", "P", "95", "-2", "2.40"), pos("b", "P", "90", "2", "1.10")}}, compute.StructureParams{})
	if len(res.Structures) != 1 {
		t.Fatalf("matcher gave %d structures", len(res.Structures))
	}
	m := res.Structures[0]
	if got := one(m); len(got.BreakEvens) != 1 || got.BreakEvens[0] != m.Figures.BreakEven.Value {
		t.Errorf("break-evens %v, matcher %q", got.BreakEvens, m.Figures.BreakEven.Value)
	}

	condor := st("a:3", "iron_condor", lg("C", "115", "1", "0.70"), lg("P", "90", "1", "1.10"),
		lg("C", "110", "-1", "2.00"), lg("P", "95", "-1", "2.40"))
	if got := one(condor); !slices.Equal(got.BreakEvens, []string{"92.4000", "112.6000"}) {
		t.Errorf("iron condor = %v (%s), want 92.4 and 112.6", got.BreakEvens, got.BreakEvenGap)
	}

	twoExpiries := putSpread()
	twoExpiries.Legs[1].Expiry = "2027-01-15"
	cases := map[string]compute.Structure{
		"two expiries": twoExpiries,
		"covered call": st("a:4", "covered_call", lg("C", "120", "-1", "1.80")),
		"other":        st("a:5", "other", lg("P", "95", "-1", "2.40"), lg("C", "100", "1", "1.00")),
	}
	for name, s := range cases {
		if got := one(s); got.BreakEvenGap != "no_rule" || len(got.BreakEvens) != 0 {
			t.Errorf("%s: %v gap %q, want no_rule", name, got.BreakEvens, got.BreakEvenGap)
		}
	}

	straddle := st("a:6", "long_straddle", lg("P", "100", "1", "0"), lg("C", "100", "1", "0"))
	if got := one(straddle); !slices.Equal(got.BreakEvens, []string{"100.0000"}) {
		t.Errorf("touch at a corner = %v (%s), want one value 100", got.BreakEvens, got.BreakEvenGap)
	}

	ratio := st("a:7", "other", lg("C", "100", "1", "1"), lg("C", "110", "-2", "0.20"))
	ratio.Kind = "call_ratio"
	if got := one(ratio); !slices.Equal(got.BreakEvens, []string{"100.6000", "119.4000"}) {
		t.Errorf("crossing above the top strike = %v (%s), want 100.6 and 119.4", got.BreakEvens, got.BreakEvenGap)
	}

	missing := st("a:8", "put_spread", lg("P", "95", "-1", ""), lg("P", "90", "1", "1.10"))
	if got := one(missing); got.BreakEvenGap != "missing_input" {
		t.Errorf("missing price gap = %q", got.BreakEvenGap)
	}
}

func TestSpreadRowsOrder(t *testing.T) {
	t.Parallel()
	mk := func(id, under, expiry string) compute.Structure {
		s := putSpread()
		s.ID, s.Underlying = id, under
		s.Legs = slices.Clone(s.Legs)
		s.Legs[0].Expiry, s.Legs[1].Expiry = expiry, expiry
		return s
	}
	structs := []compute.Structure{
		mk("z:1", "AAA", "2026-12-18"),
		mk("b:2", "BBB", "2026-11-27"),
		mk("a:3", "BBB", "2026-12-18"),
		mk("a:9", "AAA", "2026-12-18"),
		mk("c:4", "CCC", "2026-11-21"),
	}
	want := []string{"c:4", "b:2", "a:9", "z:1", "a:3"}
	days := []int{0, 6, 27, 27, 27}
	for _, perm := range [][]int{{0, 1, 2, 3, 4}, {4, 3, 2, 1, 0}, {2, 0, 4, 1, 3}} {
		var in []compute.Structure
		for _, i := range perm {
			in = append(in, structs[i])
		}
		rows := spreadlegs.New().Spreads(spreadlegsapi.SpreadIn{Structures: in, AsOf: "2026-11-21"}).Rows
		var ids []string
		var ds []int
		for _, r := range rows {
			ids, ds = append(ids, r.ID), append(ds, r.DaysToExpiry)
		}
		if !slices.Equal(ids, want) || !slices.Equal(ds, days) {
			t.Errorf("order %v: ids %v days %v, want %v %v", perm, ids, ds, want, days)
		}
	}
	rows := spreadlegs.New().Spreads(spreadlegsapi.SpreadIn{Structures: structs[:1], AsOf: "2026-11-21"}).Rows
	if rows[0].Expiry != "2026-12-18" {
		t.Errorf("expiry = %s", rows[0].Expiry)
	}
	collar := st("a:c", "collar", lg("C", "110", "-1", "1"), lg("P", "90", "1", "1"))
	if got := one(collar); got.BreakEvenGap != "no_rule" {
		t.Errorf("collar gap = %q, want no_rule", got.BreakEvenGap)
	}
}
