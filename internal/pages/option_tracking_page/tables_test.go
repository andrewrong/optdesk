package option_tracking_page

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/trackapi"
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
	want := []string{"company", "contract", "qty", "dte", "moneyness", "spot", "mid", "premium", "kept", "kept_percent",
		"delta", "iv", "results", "ex", "guard"}
	if got := colIDs(a.Open); !reflect.DeepEqual(got, want) {
		t.Fatalf("columns = %v", got)
	}
	for _, id := range want {
		if _, ok := a.Open.Rows[0].Cells[id]; !ok {
			t.Errorf("row has no cell %s", id)
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
	a, _ := formatAccount("a", trackapi.TrackResult{Open: []opt{sampleOpen("p")}}, false)
	if a.Title != "Account (a)" || a.Alias != "a" {
		t.Fatalf("account = %+v", a)
	}
	a, _ = formatAccount("a", trackapi.TrackResult{Recent: []opt{sampleClosed("z")}}, false)
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
	c := openTable([]opt{o}).Rows[0].Cells
	want := map[string]string{"spot": "394.82", "mid": "3.33", "kept": "201.50", "kept_percent": "62.99",
		"delta": "-0.25", "iv": "0.3125"}
	for k, w := range want {
		if v := asValue(t, c[k]); v.Text != w {
			t.Errorf("%s = %q, want %q", k, v.Text, w)
		}
	}
	r := recentTable([]opt{o}).Rows[0].Cells
	if v := asValue(t, r["commissions"]); v.Text != "1.30" {
		t.Errorf("commissions = %q", v.Text)
	}
	if v := asValue(t, r["realized"]); v.Text != "12.35" {
		t.Errorf("realized = %q", v.Text)
	}
}
