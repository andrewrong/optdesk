package optionshortlist

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	api "github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
)

// good is a row that passes every check; tests change one field.
func good(key string) api.Row {
	return api.Row{
		Key: key, Company: "AAA", Ticker: "AAA", Strike: "645", Spot: "686.97",
		Delta: "-0.22", Bid: "5.10", Annualized: "0.20", DTE: "30",
	}
}

func sel(t *testing.T, rows ...api.Row) api.Result {
	t.Helper()
	r, err := New().Select(api.Input{Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func dropN(r api.Result, code string) int {
	for _, d := range r.Dropped {
		if d.Code == code {
			return d.N
		}
	}
	return 0
}

func TestDistanceFromStrikeAndSpot(t *testing.T) {
	t.Parallel()
	r := sel(t, good("a"))
	if r.Shown != 1 || !strings.HasPrefix(r.Groups[0].Rows[0].OTM, "6.1") {
		t.Fatalf("got %+v", r)
	}
	low := good("b")
	low.Strike, low.Spot = "97", "100" // 3 percent; the break-even would be 11.5 percent below
	r = sel(t, low)
	if r.Shown != 0 || dropN(r, "otm_low") != 1 {
		t.Fatalf("got %+v", r)
	}
	high := good("c")
	high.Strike = "80"
	high.Spot = "100"
	if r = sel(t, high); dropN(r, "otm_high") != 1 {
		t.Fatalf("got %+v", r)
	}
}

func TestAnnualizedIsStoredYield(t *testing.T) {
	t.Parallel()
	a, b := good("a"), good("b")
	a.Company, a.Ticker, a.Annualized = "AAA", "AAA", "0.10"
	b.Company, b.Ticker, b.Annualized = "BBB", "BBB", "0.30"
	b.Bid = "0.01" // the bid over the strike would say otherwise; the stored text rules
	r := sel(t, a, b)
	if len(r.Groups) != 2 || r.Groups[0].Company != "BBB" || r.Groups[0].Best != "0.30" || r.Groups[1].Best != "0.10" {
		t.Fatalf("got %+v", r.Groups)
	}
}

func TestDeltaBandUsesAbsoluteValue(t *testing.T) {
	t.Parallel()
	a, b := good("a"), good("b")
	b.Delta = "-0.35"
	r := sel(t, a, b)
	if r.Shown != 1 || r.Groups[0].Rows[0].AbsDelta != "0.22" || dropN(r, "delta_out") != 1 {
		t.Fatalf("got %+v", r)
	}
}

func TestEmptyDeltaIsNoDelta(t *testing.T) {
	t.Parallel()
	a := good("a")
	a.Delta = ""
	if r := sel(t, a); dropN(r, "no_delta") != 1 || r.Shown != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestWideSpreadWithLowOpenInterestMarkDropped(t *testing.T) {
	t.Parallel()
	a := good("a")
	a.QuoteReason = "wide_spread"
	a.Marks = []string{"no_open_interest"}
	if r := sel(t, a); dropN(r, "no_open_interest") != 1 || r.Shown != 0 {
		t.Fatalf("got %+v", r)
	}
	b := good("b")
	b.QuoteReason = "low_open_interest"
	if r := sel(t, b); dropN(r, "no_open_interest") != 1 {
		t.Fatalf("got %+v", r)
	}
}

func TestWideSpreadAloneKeptWithBadge(t *testing.T) {
	t.Parallel()
	a := good("a")
	a.QuoteReason = "wide_spread"
	r := sel(t, a)
	if r.Shown != 1 || r.Groups[0].Rows[0].Liquidity != "wide_spread" {
		t.Fatalf("got %+v", r)
	}
}

func TestDropOrderIsFixed(t *testing.T) {
	t.Parallel()
	a := good("a")
	a.Bid = ""
	a.Delta = "-0.9"
	r := sel(t, a)
	want := []api.Drop{{Code: "no_bid", N: 1}}
	if !reflect.DeepEqual(r.Dropped, want) {
		t.Fatalf("got %+v", r.Dropped)
	}
	for _, q := range []string{"no_quote", "no_bid", "no_ask", "small_bid_size"} {
		b := good("b")
		b.QuoteReason = q
		if dropN(sel(t, b), "no_bid") != 1 {
			t.Fatalf("quote %s", q)
		}
	}
}

func TestCapKeepsBestYield(t *testing.T) {
	t.Parallel()
	var rows []api.Row
	for i, y := range []string{"0.11", "0.15", "0.12", "0.14", "0.13"} {
		r := good("k" + strconv.Itoa(i))
		r.Annualized = y
		rows = append(rows, r)
	}
	r := sel(t, rows...)
	if r.Shown != 3 || dropN(r, "over_cap") != 2 {
		t.Fatalf("got %+v", r)
	}
	var keys []string
	for _, p := range r.Groups[0].Rows {
		keys = append(keys, p.Key)
	}
	if !reflect.DeepEqual(keys, []string{"k1", "k3", "k4"}) {
		t.Fatalf("keys %v", keys)
	}
}

func TestTieBreakStable(t *testing.T) {
	t.Parallel()
	a, b, c := good("a"), good("b"), good("c")
	b.Strike, c.Company, c.Ticker = "650", "CCC", "CCC"
	r1 := sel(t, a, b, c)
	r2 := sel(t, c, b, a)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatalf("%+v vs %+v", r1, r2)
	}
	if r1.Groups[0].Company != "AAA" || r1.Groups[0].Rows[0].Key != "a" {
		t.Fatalf("got %+v", r1.Groups)
	}
}

func TestCountsAddUp(t *testing.T) {
	t.Parallel()
	rows := []api.Row{good("a"), good("b"), good("c"), good("d"), good("e")}
	rows[1].Bid = "0"
	rows[2].Spot = ""
	rows[3].Marks = []string{"unreadable_gates"}
	rows[4].Delta = "x"
	r := sel(t, append(rows, good("f"), good("g"))...)
	sum := r.Shown
	for _, d := range r.Dropped {
		sum += d.N
	}
	if r.Scanned != 7 || sum != r.Scanned || r.Shown != 3 {
		t.Fatalf("got %+v", r)
	}
	if r.DTEMin != "30" || r.DTEMax != "30" || r.Bounds.PerUnderlying != 3 {
		t.Fatalf("got %+v", r)
	}
	st := good("s")
	st.EventsReason = "stale_chain"
	if sel(t, st).StaleChain != 1 {
		t.Fatal("stale")
	}
}

func TestUnreadableBidDropsRow(t *testing.T) {
	t.Parallel()
	a, b, c := good("a"), good("b"), good("c")
	b.Bid = "abc"
	c.Company, c.Ticker, c.Delta = "CCC", "CCC", "n/a"
	r := sel(t, a, b, c)
	if r.Shown != 1 || dropN(r, "bad_row") != 1 || dropN(r, "no_delta") != 1 {
		t.Fatalf("got %+v", r)
	}
}
