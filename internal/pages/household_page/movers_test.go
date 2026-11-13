package household_page_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// dayData is what the fake store holds for one session day: the rows of capture "c1" (or of the
// capture named in the rows), plus the rows of another capture in other and otherNet.
type dayData struct {
	marks    []mark.PositionMark
	pos      []store.Position
	net, ccy string
	other    []store.Position
	otherNet string
}

// dayAccount answers positions per day.
type dayAccount struct {
	*fakeAccount
	pos map[string][]store.Position
}

func (d *dayAccount) Positions(_ context.Context, _, day string) ([]store.Position, error) {
	return d.pos[day], nil
}

func capStock(sym string, con int64, qty, capID string) store.Position {
	p := stock(sym, con, qty, "1")
	p.CaptureID = capID
	return p
}

func capMark(con int64, price, basis, capID string) mark.PositionMark {
	return mark.PositionMark{CaptureID: capID, ConID: con, Price: price, Basis: basis, Currency: "USD"}
}

func netRow(day, capID, net, ccy string) store.AccountValue {
	return store.AccountValue{CaptureID: capID, Alias: "main", SessionDay: day, Tag: "NetLiquidation", Value: net, Currency: ccy,
		CapturedAt: time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)}
}

// moversConfig is a household config over the fake days; a day with no marks has no mark row.
func moversConfig(days map[string]dayData) (household_page.Config, *fakeMarks) {
	acc := &fakeAccount{values: map[string]map[string][]store.AccountValue{"main": {}}}
	da := &dayAccount{fakeAccount: acc, pos: map[string][]store.Position{}}
	marks := &fakeMarks{byDay: map[string][]mark.PositionMark{}}
	for d, v := range days {
		ccy := v.ccy
		if ccy == "" {
			ccy = "USD"
		}
		var vs []store.AccountValue
		if v.otherNet != "" {
			vs = append(vs, netRow(d, "c0", v.otherNet, ccy))
		}
		acc.values["main"][d] = append(vs, netRow(d, "c1", v.net, ccy))
		da.pos[d] = append(append([]store.Position{}, v.other...), v.pos...)
		if len(v.marks) > 0 {
			marks.byDay[d] = v.marks
		}
	}
	var got blockapi.HouseholdInput
	c := config(acc, &got)
	c.Account = da
	c.Marks = marks
	c.Holdings = pagefig.NewHoldings
	return c, marks
}

func buildOn(t *testing.T, c household_page.Config, page string) (map[string]any, []byte) {
	t.Helper()
	b, err := household_page.NewPage(c, "main").Build(context.Background(), nil, contracts.Day(page), at)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, b
}

func moversOf(m map[string]any) map[string]any { return m["movers"].(map[string]any) }

// twoDays is AAA 100 sh at 240 then 250, on D0 and D1.
func twoDays() map[string]dayData {
	return map[string]dayData{
		"2026-11-09": {net: "99000.00", pos: []store.Position{capStock("AAA", 1, "100", "c1")},
			marks: []mark.PositionMark{capMark(1, "240", mark.BasisLast, "c1")}},
		"2026-11-10": {net: "100000.00", pos: []store.Position{capStock("AAA", 1, "100", "c1")},
			marks: []mark.PositionMark{capMark(1, "250", mark.BasisLast, "c1")}},
	}
}

const noMoversText = "The change today needs prices of two trading days: no data yet"

func amountOf(t *testing.T, row any) float64 {
	t.Helper()
	a, ok := row.(map[string]any)["amount"].(map[string]any)
	if !ok {
		t.Fatalf("row has no amount: %v", row)
	}
	return a["value"].(float64)
}

func TestMoversDaysNotPageDay(t *testing.T) {
	t.Parallel()
	days := map[string]dayData{
		"2026-11-10": twoDays()["2026-11-09"],
		"2026-11-12": twoDays()["2026-11-10"],
	}
	c, _ := moversConfig(days)
	m, b := buildOn(t, c, "2026-11-13")
	validate(t, b)
	mv := moversOf(m)
	cols := mv["columns"].([]any)
	if cols[1] != "Previous (11-10)" || cols[2] != "Last (11-12)" || cols[0] != "Name" || cols[3] != "Moved (USD)" {
		t.Fatalf("columns = %v", cols)
	}
	if mv["gap"] != nil || amountOf(t, mv["stocks"].([]any)[0]) != 1000 {
		t.Fatalf("movers = %v", mv)
	}
}

func TestMoversNoDataOneDay(t *testing.T) {
	t.Parallel()
	one := twoDays()
	delete(one, "2026-11-09")
	for name, days := range map[string]map[string]dayData{
		"one day": one,
		"no day":  {"2026-11-10": {net: "1.00"}},
	} {
		c, _ := moversConfig(days)
		m, b := buildOn(t, c, "2026-11-10")
		validate(t, b)
		if g := gapOf(t, m, "movers"); g["reason"] != "no_data" || g["text"] != noMoversText {
			t.Fatalf("%s: gap = %v", name, g)
		}
	}
}

func TestMoversStaleD1(t *testing.T) {
	t.Parallel()
	days := map[string]dayData{
		"2026-11-03": twoDays()["2026-11-09"],
		"2026-11-04": twoDays()["2026-11-10"],
		"2026-11-10": {net: "1.00"},
	}
	c, _ := moversConfig(days)
	m, _ := buildOn(t, c, "2026-11-10")
	if g := gapOf(t, m, "movers"); g["reason"] != "stale_input" || !strings.Contains(g["text"].(string), "2026-11-04") {
		t.Fatalf("gap = %v", g)
	}
}

func TestMoversStaleD0(t *testing.T) {
	t.Parallel()
	days := map[string]dayData{
		"2026-11-03": twoDays()["2026-11-09"],
		"2026-11-10": twoDays()["2026-11-10"],
	}
	c, _ := moversConfig(days)
	m, _ := buildOn(t, c, "2026-11-10")
	if g := gapOf(t, m, "movers"); g["reason"] != "stale_input" || !strings.Contains(g["text"].(string), "2026-11-03") {
		t.Fatalf("gap = %v", g)
	}
}

func TestMoversHeldBothDaysOnly(t *testing.T) {
	t.Parallel()
	days := twoDays()
	d0, d1 := days["2026-11-09"], days["2026-11-10"]
	d0.pos = append(d0.pos, capStock("SOLD", 2, "10", "c1"), capStock("ZERO", 4, "0", "c1"))
	d1.pos = append(d1.pos, capStock("NEW", 3, "10", "c1"), capStock("ZERO", 4, "5", "c1"))
	for _, con := range []int64{2, 3, 4} {
		d0.marks = append(d0.marks, capMark(con, "10", mark.BasisLast, "c1"))
		d1.marks = append(d1.marks, capMark(con, "12", mark.BasisLast, "c1"))
	}
	days["2026-11-09"], days["2026-11-10"] = d0, d1
	c, _ := moversConfig(days)
	m, _ := buildOn(t, c, "2026-11-10")
	stocks := moversOf(m)["stocks"].([]any)
	if len(stocks) != 1 || stocks[0].(map[string]any)["name"] != "AAA" {
		t.Fatalf("stocks = %v", stocks)
	}
}

func TestMoversCaptureNotMixed(t *testing.T) {
	t.Parallel()
	days := twoDays()
	d0 := days["2026-11-09"]
	d0.other = []store.Position{capStock("AAA", 1, "999", "c0")}
	d0.otherNet = "1.00"
	days["2026-11-09"] = d0
	c, _ := moversConfig(days)
	m, _ := buildOn(t, c, "2026-11-10")
	mv := moversOf(m)
	if got := amountOf(t, mv["stocks"].([]any)[0]); got != 1000 {
		t.Fatalf("amount = %v, want 1000 (100 sh of the marked capture)", got)
	}
	rows := mv["rows"].([]any)
	if got := amountOf(t, rows[0]); got != 1000 {
		t.Fatalf("total = %v, want 100000 - 99000", got)
	}
}

func TestMoversMissingMarkGap(t *testing.T) {
	t.Parallel()
	days := twoDays()
	d0, d1 := days["2026-11-09"], days["2026-11-10"]
	d0.pos = append(d0.pos, capStock("BBB", 2, "50", "c1"))
	d1.pos = append(d1.pos, capStock("BBB", 2, "50", "c1"))
	d0.marks = append(d0.marks, capMark(2, "100", mark.BasisLast, "c1"))
	days["2026-11-09"], days["2026-11-10"] = d0, d1
	c, _ := moversConfig(days)
	m, b := buildOn(t, c, "2026-11-10")
	validate(t, b)
	mv := moversOf(m)
	stocks := mv["stocks"].([]any)
	bbb := stocks[1].(map[string]any)
	if bbb["name"] != "BBB" || bbb["amount"] != nil || bbb["current"].(map[string]any)["reason"] != "missing_quote" {
		t.Fatalf("BBB = %v", bbb)
	}
	if mv["gap"] != nil || len(mv["rows"].([]any)) != 2 {
		t.Fatalf("a line without a mark must not stop the total: %v", mv)
	}
}

func TestMoversBasisMismatch(t *testing.T) {
	t.Parallel()
	days := twoDays()
	d0 := days["2026-11-09"]
	d0.marks = []mark.PositionMark{capMark(1, "240", mark.BasisClose, "c1")}
	days["2026-11-09"] = d0
	c, _ := moversConfig(days)
	m, b := buildOn(t, c, "2026-11-10")
	validate(t, b)
	row := moversOf(m)["stocks"].([]any)[0].(map[string]any)
	if row["amount"] != nil || row["previous"].(map[string]any)["reason"] != "missing_quote" ||
		row["current"].(map[string]any)["reason"] != "missing_quote" {
		t.Fatalf("row = %v", row)
	}
}

func jpyDays() map[string]dayData {
	jp := func(con int64, qty string) store.Position {
		p := capStock("4063", con, qty, "c1")
		p.Currency = "JPY"
		return p
	}
	jm := func(price string) []mark.PositionMark {
		return []mark.PositionMark{{CaptureID: "c1", ConID: 7, Price: price, Basis: mark.BasisLast, Currency: "JPY"}}
	}
	return map[string]dayData{
		"2026-11-09": {net: "15000000", ccy: "JPY", pos: []store.Position{jp(7, "100")}, marks: jm("3400")},
		"2026-11-10": {net: "15150000", ccy: "JPY", pos: []store.Position{jp(7, "100")}, marks: jm("3500")},
	}
}

func TestMoversJpyRate(t *testing.T) {
	t.Parallel()
	t.Run("rate of the capture day is reused", func(t *testing.T) {
		t.Parallel()
		c, _ := moversConfig(jpyDays())
		fx := &fakeFx{rate: "150"}
		c.Fx = fx
		m, b := buildOn(t, c, "2026-11-10")
		validate(t, b)
		mv := moversOf(m)
		if len(fx.calls) != 1 {
			t.Fatalf("fx calls = %v, want the one of the holdings block", fx.calls)
		}
		if got := amountOf(t, mv["stocks"].([]any)[0]); got != 66.67 {
			t.Fatalf("amount = %v", got)
		}
		if got := amountOf(t, mv["rows"].([]any)[0]); got != 1000 {
			t.Fatalf("total = %v", got)
		}
	})
	t.Run("D1 before the capture day reads its own rate", func(t *testing.T) {
		t.Parallel()
		days := jpyDays()
		days["2026-11-11"] = dayData{net: "15150000", ccy: "JPY"}
		c, _ := moversConfig(days)
		fx := &fakeFx{rate: "150"}
		c.Fx = fx
		m, _ := buildOn(t, c, "2026-11-11")
		found := false
		for _, call := range fx.calls {
			found = found || call == "USDJPY 2026-11-10"
		}
		if !found || amountOf(t, moversOf(m)["stocks"].([]any)[0]) != 66.67 {
			t.Fatalf("fx calls = %v, movers = %v", fx.calls, moversOf(m))
		}
	})
	t.Run("no rate gives the block gap and keeps the lines", func(t *testing.T) {
		t.Parallel()
		c, _ := moversConfig(jpyDays())
		c.Fx = &fakeFx{}
		m, b := buildOn(t, c, "2026-11-10")
		validate(t, b)
		g := gapOf(t, m, "movers")
		if g["reason"] != "missing_input" || !strings.Contains(g["text"].(string), "USDJPY") {
			t.Fatalf("gap = %v", g)
		}
		mv := moversOf(m)
		if len(mv["rows"].([]any)) != 0 || len(mv["stocks"].([]any)) != 1 {
			t.Fatalf("movers = %v", mv)
		}
	})
}

func TestMoversRowsHaveAliasScope(t *testing.T) {
	t.Parallel()
	c, _ := moversConfig(twoDays())
	m, _ := buildOn(t, c, "2026-11-10")
	mv := moversOf(m)
	rows := mv["rows"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["name"] != "Change in net liquidation" || rows[1].(map[string]any)["name"] != "Other" {
		t.Fatalf("rows = %v", rows)
	}
	for _, r := range rows {
		if r.(map[string]any)["scope"] != "main" {
			t.Fatalf("row scope: %v", r)
		}
	}
	if got := amountOf(t, rows[1]); got != 0 {
		t.Fatalf("other = %v, want 1000 total - 1000 line", got)
	}
	if tot, ok := mv["totals"].([]any); !ok || len(tot) != 0 {
		t.Fatalf("totals = %v", mv["totals"])
	}
	if s := mv["stocks"].([]any)[0].(map[string]any); s["scope"] != "main" {
		t.Fatalf("stock scope: %v", s)
	}
}

func TestMoversDataTimesNameBothDays(t *testing.T) {
	t.Parallel()
	days := twoDays()
	d0, d1 := days["2026-11-09"], days["2026-11-10"]
	old := capMark(1, "240", mark.BasisLast, "c1")
	old.MarkedAt = time.Date(2026, 11, 9, 20, 30, 0, 0, time.UTC)
	older := old
	older.ConID, older.MarkedAt = 2, old.MarkedAt.Add(-time.Hour)
	d0.marks = []mark.PositionMark{older, old}
	cur := capMark(1, "250", mark.BasisLast, "c1")
	cur.MarkedAt = time.Date(2026, 11, 10, 20, 30, 0, 0, time.UTC)
	d1.marks = []mark.PositionMark{cur}
	days["2026-11-09"], days["2026-11-10"] = d0, d1
	c, _ := moversConfig(days)
	m, _ := buildOn(t, c, "2026-11-10")
	got := map[string]string{}
	for _, e := range m["data_times"].([]any) {
		got[e.(map[string]any)["source"].(string)] = e.(map[string]any)["data_time"].(string)
	}
	if got["position_mark_prev"] != "2026-11-10T04:30:00+08:00" || got["position_mark_last"] != "2026-11-11T04:30:00+08:00" {
		t.Fatalf("data_times = %v", got)
	}
}

func TestMoversPageValidatesSchema(t *testing.T) {
	t.Parallel()
	c, _ := moversConfig(twoDays())
	m, b := buildOn(t, c, "2026-11-10")
	validate(t, b)
	if _, ok := m["long_term"]; ok {
		t.Fatalf("long_term is emitted: %s", b)
	}
}
