package optiontier

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// mk is a fit row of company co: spot 100, strike 90 (10 percent out), delta -0.20, bid 1, DTE 30.
func mk(key, co string, mod func(*optiontierapi.Row)) optiontierapi.Row {
	r := optiontierapi.Row{
		Key: key, Company: co, ChainDay: "2026-11-17", Expiry: "2026-12-17", DTE: 30,
		Spot: "100", Strike: "90", Bid: "1", BuyPrice: "95", Delta: "-0.20", Multiplier: "100", IV: "0.3",
	}
	if mod != nil {
		mod(&r)
	}
	return r
}

func bid(b string) func(*optiontierapi.Row) { return func(r *optiontierapi.Row) { r.Bid = b } }

// rowOutBand fits but is 1 percent out, so outside the band.
func rowOutBand(r *optiontierapi.Row) { r.Strike = "99"; r.BuyPrice = "100" }

// Row makers by condition.
var (
	rowITM      = func(r *optiontierapi.Row) { r.Strike = "105" }
	rowNoSpot   = func(r *optiontierapi.Row) { r.Spot = "" }
	rowNoBid    = func(r *optiontierapi.Row) { r.Bid = "" }
	rowNoBuy    = func(r *optiontierapi.Row) { r.BuyPrice = "" }
	rowAboveBuy = func(r *optiontierapi.Row) { r.BuyPrice = "80" }
	rowBadInput = func(r *optiontierapi.Row) { r.Strike = strikeBadInput }
	rowNoYield  = func(r *optiontierapi.Row) { r.DTE = 0 }
	rowCheck    = func(r *optiontierapi.Row) { r.Marks = []string{"no_open_interest"} }
)

func blocksOf(t *testing.T, p optiontierapi.Params, rows ...optiontierapi.Row) []optiontierapi.CompanyBlock {
	t.Helper()
	res, err := New(&fakeEvaluator{}).Judge(optiontierapi.Input{Day: "2026-11-17", Params: p, Rows: rows})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	return res.Blocks
}

func blockOf(t *testing.T, rows ...optiontierapi.Row) optiontierapi.CompanyBlock {
	t.Helper()
	b := blocksOf(t, DefaultParams(), rows...)
	if len(b) != 1 {
		t.Fatalf("blocks = %d, want 1", len(b))
	}
	return b[0]
}

func TestBlockRowsByYield(t *testing.T) {
	t.Parallel()
	b := blockOf(t,
		mk("y12", "A", bid("0.3")), // 12 percent a year is not needed exactly: order is what counts
		mk("y80", "A", bid("2")),
		mk("none", "A", rowNoYield),
		mk("g2", "A", rowITM),
		mk("g1", "A", rowITM),
	)
	if want := []string{"y80", "y12", "none"}; !reflect.DeepEqual(b.Shown, want) {
		t.Errorf("Shown = %v, want %v", b.Shown, want)
	}
	if want := []string{"g1", "g2"}; !reflect.DeepEqual(b.Folded, want) {
		t.Errorf("Folded = %v, want %v", b.Folded, want)
	}
	if b.Best != "27.0370" {
		t.Errorf("Best = %q, want 27.0370", b.Best)
	}
	// ties: DTE, then strike, then key ascending
	tie := blockOf(t,
		mk("k2", "A", func(r *optiontierapi.Row) { r.Bid = "1" }),
		mk("k1", "A", func(r *optiontierapi.Row) { r.Bid = "1" }),
	)
	if want := []string{"k1", "k2"}; !reflect.DeepEqual(tie.Shown, want) {
		t.Errorf("tie Shown = %v, want %v", tie.Shown, want)
	}
}

func TestBlockStateRule(t *testing.T) {
	t.Parallel()
	type m = func(*optiontierapi.Row)
	cases := []struct {
		name string
		rows []m
		want string
	}{
		{"shown fit with grey", []m{nil, rowITM}, optiontierapi.StateSellable},
		{"only check in band", []m{rowCheckInBand}, optiontierapi.StateSellable},
		{"fits outside band", []m{rowOutBand}, optiontierapi.StateOutBand},
		{"no buy price", []m{rowNoBuy, rowITM}, optiontierapi.StateNoBuyPrice},
		{"above buy with itm and no bid", []m{rowAboveBuy, rowITM, rowNoBid}, optiontierapi.StateSellable},
		{"itm and no spot", []m{rowITM, rowNoSpot}, optiontierapi.StateNoQuote},
		{"itm and no bid", []m{rowITM, rowNoBid}, optiontierapi.StateNoQuote},
		{"above buy and bad input", []m{rowAboveBuy, rowBadInput}, optiontierapi.StateSellable},
		{"itm only", []m{rowITM}, optiontierapi.StateBlocked},
	}
	for _, c := range cases {
		var rows []optiontierapi.Row
		for i, mod := range c.rows {
			rows = append(rows, mk(string(rune('a'+i)), "A", mod))
		}
		if got := blockOf(t, rows...).State; got != c.want {
			t.Errorf("%s: state = %s, want %s", c.name, got, c.want)
		}
	}
}

// rowCheckInBand is a check row (earnings before expiry) that is inside the band.
func rowCheckInBand(r *optiontierapi.Row) { r.EventsReason = eventsEarningsBeforeExpiry }

func TestCompanyAboveBuyIsSellableNotFolded(t *testing.T) {
	t.Parallel()
	rows := []optiontierapi.Row{mk("ab1", "A", rowAboveBuy), mk("ab2", "A", rowAboveBuy)}
	b := blockOf(t, rows...)
	if b.State != optiontierapi.StateSellable || len(b.Shown) != 2 || len(b.Folded) != 0 {
		t.Errorf("state %s shown %v folded %v", b.State, b.Shown, b.Folded)
	}
	rows = append(rows, mk("itm", "A", rowITM), mk("nospot", "A", rowNoSpot))
	b = blockOf(t, rows...)
	if b.State != optiontierapi.StateSellable || len(b.Shown) != 2 {
		t.Errorf("mixed: state %s shown %v", b.State, b.Shown)
	}
}

func TestBandCodes(t *testing.T) {
	t.Parallel()
	delta := func(d string) func(*optiontierapi.Row) { return func(r *optiontierapi.Row) { r.Delta = d } }
	strike := func(s string) func(*optiontierapi.Row) { return func(r *optiontierapi.Row) { r.Strike = s } }
	cases := []struct {
		name string
		mod  func(*optiontierapi.Row)
		want string // "" is shown
	}{
		{"4 out", strike("96"), optiontierapi.FoldOTMLow},
		{"16 out", strike("84"), optiontierapi.FoldOTMHigh},
		{"delta 0.10", delta("-0.10"), optiontierapi.FoldDeltaOut},
		{"delta 0.35 check", delta("-0.35"), optiontierapi.FoldDeltaOut},
		{"no delta check", delta(""), optiontierapi.FoldNoDelta},
		{"low open interest", func(r *optiontierapi.Row) { r.QuoteReason = "low_open_interest" }, optiontierapi.FoldNoOpenInterest},
		{"no open interest mark", rowCheck, optiontierapi.FoldNoOpenInterest},
		{"edge 5", strike("95"), ""},
		{"edge 15", strike("85"), ""},
		{"edge delta 0.15", delta("-0.15"), ""},
		{"edge delta 0.30", delta("-0.30"), ""},
		{"two failures", func(r *optiontierapi.Row) { r.Strike = "96"; r.Delta = "-0.05"; r.Marks = []string{"no_open_interest"} }, optiontierapi.FoldNoOpenInterest},
		{"two failures otm and delta", func(r *optiontierapi.Row) { r.Strike = "84"; r.Delta = "" }, optiontierapi.FoldOTMHigh},
		{"no delta and delta", func(r *optiontierapi.Row) { r.Strike = "96"; r.Delta = "-0.05" }, optiontierapi.FoldOTMLow},
	}
	for _, c := range cases {
		b := blockOf(t, mk("x", "A", c.mod))
		if c.want == "" {
			if !reflect.DeepEqual(b.Shown, []string{"x"}) {
				t.Errorf("%s: Shown = %v, want shown", c.name, b.Shown)
			}
			continue
		}
		want := []optiontierapi.GreyCount{{Code: c.want, N: 1}}
		if len(b.Shown) != 0 || !reflect.DeepEqual(b.Why, want) {
			t.Errorf("%s: Shown = %v Why = %v, want fold %s", c.name, b.Shown, b.Why, c.want)
		}
	}
}

func TestBandFollowsDeltaMax(t *testing.T) {
	t.Parallel()
	row := mk("x", "A", func(r *optiontierapi.Row) { r.Delta = "-0.25" })
	p := DefaultParams()
	p.DeltaMax = "0.20"
	b := blocksOf(t, p, row)[0]
	if len(b.Shown) != 0 || b.Why[0].Code != optiontierapi.FoldDeltaOut {
		t.Errorf("DeltaMax 0.20: %+v", b)
	}
	p.DeltaMax = "0.25"
	if b := blocksOf(t, p, row)[0]; len(b.Shown) != 1 {
		t.Errorf("DeltaMax 0.25: %+v", b)
	}
}

func TestBandDefaults(t *testing.T) {
	t.Parallel()
	p := fillParams(optiontierapi.Params{BandOTMMinPct: "x"})
	if p.BandOTMMinPct != "5" || p.BandOTMMaxPct != "15" || p.BandDeltaMin != "0.15" || p.DeltaMax != "0.3" || p.PerCompany != 3 {
		t.Errorf("defaults = %+v", p)
	}
	res, err := New(&fakeEvaluator{}).Judge(optiontierapi.Input{})
	if err != nil || res.Params != DefaultParams() {
		t.Errorf("echo = %+v, %v", res.Params, err)
	}
}

func TestBlockCap(t *testing.T) {
	t.Parallel()
	var rows []optiontierapi.Row
	for i, b := range []string{"1", "2", "3", "4", "5"} {
		rows = append(rows, mk(string(rune('a'+i)), "A", bid(b)))
	}
	b := blockOf(t, rows...)
	if want := []string{"e", "d", "c"}; !reflect.DeepEqual(b.Shown, want) {
		t.Errorf("Shown = %v, want %v", b.Shown, want)
	}
	if want := []optiontierapi.GreyCount{{Code: optiontierapi.FoldOverCap, N: 2}}; !reflect.DeepEqual(b.Why, want) || b.NOpen != 2 {
		t.Errorf("Why = %v NOpen = %d", b.Why, b.NOpen)
	}
	p := DefaultParams()
	p.PerCompany = 5
	if b := blocksOf(t, p, rows...)[0]; len(b.Shown) != 5 || len(b.Folded) != 0 {
		t.Errorf("cap 5: %+v", b)
	}
}

func TestBlockCountsAddUp(t *testing.T) {
	t.Parallel()
	strike := func(s string) func(*optiontierapi.Row) { return func(r *optiontierapi.Row) { r.Strike = s } }
	rows := []optiontierapi.Row{
		mk("fit", "A", bid("2")),
		mk("chk", "A", rowCheckInBand),
		mk("nobuy", "A", rowNoBuy),
		mk("nospot", "A", rowNoSpot),
		mk("itm", "A", rowITM),
		mk("nobid", "A", rowNoBid),
		mk("above", "A", rowAboveBuy),
		mk("bad", "A", rowBadInput),
		mk("low", "A", strike("96")),
		mk("high", "A", strike("84")),
		mk("nodelta", "A", func(r *optiontierapi.Row) { r.Delta = "" }),
		mk("oi", "A", rowCheck),
		mk("deltaout", "A", func(r *optiontierapi.Row) { r.Delta = "-0.05" }),
		mk("other", "B", rowITM),
	}
	counts := map[string]int{"A": 13, "B": 1}
	for _, b := range blocksOf(t, DefaultParams(), rows...) {
		n := counts[b.Company]
		if len(b.Shown)+len(b.Folded) != n {
			t.Errorf("%s: shown %d + folded %d != %d", b.Company, len(b.Shown), len(b.Folded), n)
		}
		sum := 0
		for _, w := range b.Why {
			if w.N <= 0 {
				t.Errorf("%s: reason %s has N %d", b.Company, w.Code, w.N)
			}
			sum += w.N
		}
		if sum != len(b.Folded) || b.NFit+b.NCheck != len(b.Shown) || b.NOpen != openFolded(b, rows) {
			t.Errorf("%s: block %+v", b.Company, b)
		}
	}
	a := blocksOf(t, DefaultParams(), rows...)[0]
	var codes []string
	for _, w := range a.Why {
		codes = append(codes, w.Code)
	}
	want := []string{"no_buy_price", "no_spot", "in_the_money", "no_bid", "bad_input",
		"no_open_interest", "otm_low", "otm_high", "no_delta", "delta_out"}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("Why order = %v, want %v", codes, want)
	}
}

// openFolded counts the folded rows of b that are not grey.
func openFolded(b optiontierapi.CompanyBlock, rows []optiontierapi.Row) int {
	res, _ := New(&fakeEvaluator{}).Judge(optiontierapi.Input{Day: "2026-11-17", Params: DefaultParams(), Rows: rows})
	tier := map[string]string{}
	for _, v := range res.Verdicts {
		tier[v.Key] = v.Tier
	}
	n := 0
	for _, k := range b.Folded {
		if tier[k] != tierGrey {
			n++
		}
	}
	return n
}

func TestBlockOrder(t *testing.T) {
	t.Parallel()
	rows := []optiontierapi.Row{
		mk("1", "Z-nobuy", rowNoBuy),
		mk("2", "Y-blocked", rowITM),
		mk("3", "X-above", rowAboveBuy),
		mk("4", "W-outband", rowOutBand),
		mk("5", "V-noyield", rowNoYield),
		mk("6", "U-12", bid("0.3")),
		mk("7", "T-80", bid("2")),
		mk("8", "S-quote", rowNoSpot),
		mk("9", "R-80tie", bid("2")),
	}
	var got []string
	for _, b := range blocksOf(t, DefaultParams(), rows...) {
		got = append(got, b.Company)
	}
	want := []string{"R-80tie", "T-80", "X-above", "U-12", "V-noyield", "W-outband", "Y-blocked", "S-quote", "Z-nobuy"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestBlocksEmptyNotNil(t *testing.T) {
	t.Parallel()
	res, err := New(&fakeEvaluator{}).Judge(optiontierapi.Input{})
	if err != nil || res.Blocks == nil || len(res.Blocks) != 0 {
		t.Errorf("Blocks = %#v, %v", res.Blocks, err)
	}
}
