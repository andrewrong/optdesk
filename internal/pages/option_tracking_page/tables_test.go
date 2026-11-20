package option_tracking_page

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/pages/shared"
)

func ids(rows []row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func colIDs(t *table) []string {
	var out []string
	for _, c := range t.Columns {
		out = append(out, c.ID)
	}
	return out
}

func TestOrderKept(t *testing.T) {
	t.Parallel()
	res := okResult()
	res.Open = []opt{sampleOpen("b"), sampleOpen("a"), sampleOpen("c")}
	res.Recent = []opt{sampleClosed("z"), sampleClosed("y")}
	res.Recon = []trackapi.TrackRecon{{Underlying: "ZZ", Code: "not_in_flex"}, {Underlying: "AA", Code: "not_in_account"}}
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main")).Accounts[0]
	if got := ids(a.Open.Rows); !reflect.DeepEqual(got, []string{"pos-b", "pos-a", "pos-c"}) {
		t.Fatalf("open = %v", got)
	}
	if got := ids(a.Recent.Rows); !reflect.DeepEqual(got, []string{"pos-z", "pos-y"}) {
		t.Fatalf("recent = %v", got)
	}
	if got := ids(a.Recon.Rows); !reflect.DeepEqual(got, []string{"chk-not_in_flex-ZZ---", "chk-not_in_account-AA---"}) {
		t.Fatalf("recon = %v", got)
	}
}

func TestOpenColumns(t *testing.T) {
	t.Parallel()
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": okResult()}, "main")).Accounts[0]
	want := []string{"company", "contract", "opened", "qty", "dte", "moneyness", "spot", "mid", "premium", "kept", "kept_percent",
		"delta", "iv", "results", "ex", "guard", "coverage"}
	if got := colIDs(a.Open); !reflect.DeepEqual(got, want) {
		t.Fatalf("columns = %v", got)
	}
	for _, id := range want {
		if _, ok := a.Open.Rows[0].Cells[id]; !ok {
			t.Errorf("row has no cell %s", id)
		}
	}
}

func TestOpenTableShowsOpenedDay(t *testing.T) {
	t.Parallel()
	res := okResult()
	res.Open = []opt{sampleOpen("a"), sampleOpen("b")}
	res.Open[0].PremiumIncome, res.Open[1].PremiumIncome = "647", "647"
	res.Open[0].OpenedOn, res.Open[1].OpenedOn = "2026-10-01", "2026-10-05"
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main")).Accounts[0]
	if len(a.Open.Rows) != 2 {
		t.Fatalf("%d rows", len(a.Open.Rows))
	}
	for i, day := range []string{"2026-10-01", "2026-10-05"} {
		if c := cellMap(t, a.Open.Rows[i], "opened"); c["text"] != day {
			t.Errorf("row %d opened = %v", i, c)
		}
	}
}

func TestRecentColumns(t *testing.T) {
	t.Parallel()
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": okResult()}, "main")).Accounts[0]
	want := []string{"company", "contract", "qty", "status", "closed_day", "premium", "commissions", "realized"}
	if got := colIDs(a.Recent); !reflect.DeepEqual(got, want) {
		t.Fatalf("columns = %v", got)
	}
	if c := cellMap(t, a.Recent.Rows[0], "closed_day"); c["text"] != "2026-11-03" {
		t.Fatalf("closed_day = %v", c)
	}
}

func TestReconColumns(t *testing.T) {
	t.Parallel()
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": okResult()}, "main")).Accounts[0]
	want := []string{"company", "contract", "qty", "code", "note"}
	if got := colIDs(a.Recon); !reflect.DeepEqual(got, want) {
		t.Fatalf("columns = %v", got)
	}
	if c := cellMap(t, a.Recon.Rows[0], "contract"); c["text"] != "P 300 2026-12-18" {
		t.Fatalf("contract = %v", c)
	}
}

func TestNoTotalsRow(t *testing.T) {
	t.Parallel()
	res := okResult()
	res.Open = []opt{sampleOpen("a"), sampleOpen("b")}
	res.Open[0].PremiumIncome, res.Open[1].PremiumIncome = "100", "50"
	res.Recent = nil
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main")).Accounts[0]
	if len(a.Open.Rows) != 2 {
		t.Fatalf("%d rows", len(a.Open.Rows))
	}
	for _, r := range a.Open.Rows {
		if !strings.HasPrefix(r.ID, "pos-") {
			t.Errorf("row id %s", r.ID)
		}
		for id, c := range r.Cells {
			if m, ok := c.(map[string]any); ok && m["text"] == "150" {
				t.Errorf("cell %s is 150", id)
			}
		}
	}
}

func TestNoMarginNoCash(t *testing.T) {
	t.Parallel()
	raw := strings.ToLower(string(buildRaw(t, cfgFor(map[string]trackapi.TrackResult{"main": okResult()}, "main"))))
	for _, w := range []string{"margin", "cushion", "collateral", "assignment cash", "bag-holding"} {
		if strings.Contains(raw, w) {
			t.Errorf("output holds %q", w)
		}
	}
	var vm viewModel
	if err := json.Unmarshal([]byte(raw), &vm); err != nil {
		t.Fatal(err)
	}
	a := vm.Accounts[0]
	for _, tb := range []*table{a.Open, a.Recent, a.Recon} {
		for _, id := range colIDs(tb) {
			if strings.Contains(id, "cash") {
				t.Errorf("column %s", id)
			}
		}
	}
}

func TestRecentPremiumShowsCurrency(t *testing.T) {
	t.Parallel()
	o := sampleClosed("z")
	o.Currency, o.PremiumIncome = "USD", "320"
	tb := recentTable([]opt{o})
	if v := asValue(t, tb.Rows[0].Cells["premium"]); v.Text != "$320.00" {
		t.Fatalf("premium = %+v", v)
	}
}

func TestAccountTitleNamesAccount(t *testing.T) {
	t.Parallel()
	a, _ := formatAccount("a", trackapi.TrackResult{Open: []opt{sampleOpen("p")}}, false, "", nil)
	if a.Title != "Account (a)" || a.Alias != "a" {
		t.Fatalf("account = %+v", a)
	}
	a, _ = formatAccount("a", trackapi.TrackResult{Recent: []opt{sampleClosed("z")}}, false, "", nil)
	if a.Title != "Account (a) · No open short options" {
		t.Fatalf("title = %q", a.Title)
	}
}

func TestOpenTablePricesHaveTwoDecimals(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	o.Spot, o.Mid, o.Kept = "394.8229675292969", "3.3250", "201.5000"
	o.KeptPercent, o.Delta, o.Commissions, o.RealizedPnL = "62.98765", "-0.2512345", "1.3", "12.345678"
	o.Iv = "0.31254"
	c := openTable([]opt{o}, "", nil).Rows[0].Cells
	want := map[string]string{"spot": "$394.82", "mid": "$3.33", "kept": "$201.50", "kept_percent": "63.0%",
		"delta": "−0.25", "iv": "31.3%"}
	for k, w := range want {
		if v := asValue(t, c[k]); v.Text != w {
			t.Errorf("%s = %q, want %q", k, v.Text, w)
		}
	}
	r := recentTable([]opt{o}).Rows[0].Cells
	if v := asValue(t, r["commissions"]); v.Text != "$1.30" {
		t.Errorf("commissions = %q", v.Text)
	}
	if v := asValue(t, r["realized"]); v.Text != "$12.35" {
		t.Errorf("realized = %q", v.Text)
	}
}

func TestOpenTableDeltaIvFormat(t *testing.T) {
	t.Parallel()
	cases := []struct{ delta, iv, wantDelta, wantIv string }{
		{"-0.0912", "0.4511", "−0.09", "45.1%"},
		{"0.25", "0.31254", "0.25", "31.3%"},
		{"-0.001", "0.6955", "0.00", "69.6%"},
	}
	for _, tc := range cases {
		o := sampleOpen("p")
		o.Delta, o.Iv = tc.delta, tc.iv
		c := openTable([]opt{o}, "", nil).Rows[0].Cells
		if v := asValue(t, c["delta"]); v.Text != tc.wantDelta || string(*v.Value) != tc.delta {
			t.Errorf("delta %s = %q (%v), want %q", tc.delta, v.Text, v.Value, tc.wantDelta)
		}
		if v := asValue(t, c["iv"]); v.Text != tc.wantIv || string(*v.Value) != tc.iv {
			t.Errorf("iv %s = %q (%v), want %q", tc.iv, v.Text, v.Value, tc.wantIv)
		}
	}
}

func TestTrackingMoneyColumnsFormat(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	o.Currency, o.Spot, o.Mid, o.Commissions, o.RealizedPnL = "USD", "12345.675", "-3.005", "1.3", "-1234.5"
	open := openTable([]opt{o}, "", nil).Rows[0].Cells
	recent := recentTable([]opt{o}).Rows[0].Cells
	cases := []struct {
		cells     map[string]any
		col, want string
		stored    string
	}{
		{open, "spot", "$12,345.68", "12345.675"},
		{open, "mid", "−$3.01", "-3.005"},
		{recent, "commissions", "$1.30", "1.3"},
		{recent, "realized", "−$1,234.50", "-1234.5"},
	}
	for _, tc := range cases {
		v := asValue(t, tc.cells[tc.col])
		if v.Text != tc.want || v.Value == nil || string(*v.Value) != tc.stored {
			t.Errorf("%s = %q (%v), want %q with value %s", tc.col, v.Text, v.Value, tc.want, tc.stored)
		}
	}
}

func TestRecentMoneyColumnsRoundingAndGap(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	o.Currency, o.Commissions, o.RealizedPnL = "USD", "-3.005", "-0.001"
	r := recentTable([]opt{o}).Rows[0].Cells
	for col, want := range map[string]string{"commissions": "−$3.01", "realized": "$0.00"} {
		if v := asValue(t, r[col]); v.Text != want {
			t.Errorf("%s = %q, want %q", col, v.Text, want)
		}
	}
	// A row without a mark: the recent columns keep the no_value gap, not the mark gap.
	o.Commissions, o.RealizedPnL, o.MarkStatus = "", "", codeNoMark
	r = recentTable([]opt{o}).Rows[0].Cells
	for _, col := range []string{"commissions", "realized"} {
		if g := asGap(t, r[col]); g != codeGap(codeNoValue) {
			t.Errorf("%s gap = %+v, want %+v", col, g, codeGap(codeNoValue))
		}
	}
}

func TestRecentTableLabelsNetPremium(t *testing.T) {
	t.Parallel()
	o := sampleClosed("z")
	o.Right, o.Status, o.Currency, o.PremiumIncome = "P", "closed", "USD", "-3541.0000"
	res := trackapi.TrackResult{Open: []opt{sampleOpen("p1")}, Recent: []opt{o}}
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main")).Accounts[0]
	find := func(tb *table, id string) column {
		for _, c := range tb.Columns {
			if c.ID == id {
				return c
			}
		}
		t.Fatalf("no column %s", id)
		return column{}
	}
	c := find(a.Recent, "premium")
	if c.Label != "Net premium" || c.Hint == nil || *c.Hint == "" {
		t.Fatalf("closed premium column = %+v", c)
	}
	if got := cellMap(t, a.Recent.Rows[0], "premium")["text"]; got != "−$3,541.00" {
		t.Fatalf("premium text = %v", got)
	}
	if c := find(a.Open, "premium"); c.Label != "Premium collected" {
		t.Fatalf("open premium label = %q", c.Label)
	}
}

func TestSpotBadgeNamesSourceAndDay(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	o.Spot = "156.05"
	spot := func(c shared.Close) valueCell {
		return asValue(t, openTable([]opt{o}, "2026-11-18", map[string]shared.Close{"AAPL": c}).Rows[0].Cells["spot"])
	}
	v := spot(shared.Close{Day: "2026-11-17", Price: "156.05", Via: "bar"})
	if v.Text != "$156.05" || !reflect.DeepEqual(v.Badges, []string{"close 2026-11-17"}) {
		t.Fatalf("bar spot = %+v", v)
	}
	v = spot(shared.Close{Day: "2026-11-18", Price: "156.05", Via: "option_eod"})
	if !reflect.DeepEqual(v.Badges, []string{"last print 2026-11-18"}) {
		t.Fatalf("print spot = %+v", v)
	}
	v = asValue(t, openTable([]opt{o}, "2026-11-18", nil).Rows[0].Cells["spot"])
	if v.Text != "$156.05" || len(v.Badges) != 0 {
		t.Fatalf("spot without a picked close = %+v", v)
	}
}

func TestNoMidKeepsSpotAndMoneyness(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	o.MarkStatus, o.Mid, o.Kept, o.KeptPercent, o.Delta, o.Iv = "no_mark", "", "", "", "", ""
	o.Spot, o.OtmPercent, o.Moneyness = "145.2", "3.20", "otm"
	cells := openTable([]opt{o}, "2026-11-18", nil).Rows[0].Cells
	if v := asValue(t, cells["spot"]); v.Text != "$145.20" {
		t.Fatalf("spot = %+v", v)
	}
	if v := asValue(t, cells["moneyness"]); v.Text != "+3.2%" && v.Text != "3.2%" {
		t.Fatalf("moneyness = %+v", v)
	}
	if g := asGap(t, cells["mid"]); g.Text != codeTexts["no_mark"] {
		t.Fatalf("mid = %+v", g)
	}
	o.Spot = ""
	if g := asGap(t, openTable([]opt{o}, "", nil).Rows[0].Cells["spot"]); g.Text != codeTexts["no_close"] {
		t.Fatalf("no close spot = %+v", g)
	}
}

func TestKeptPercentBeforeDelta(t *testing.T) {
	t.Parallel()
	var ids []string
	for _, c := range openTable([]opt{sampleOpen("p")}, "", nil).Columns {
		ids = append(ids, c.ID)
	}
	at := func(id string) int {
		for i, x := range ids {
			if x == id {
				return i
			}
		}
		t.Fatalf("column %s missing in %v", id, ids)
		return -1
	}
	if at("kept") != at("premium")+1 || at("kept_percent") != at("premium")+2 || at("delta") != at("premium")+3 {
		t.Fatalf("columns = %v", ids)
	}
	at("ex")
	at("guard")
}
