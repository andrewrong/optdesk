package blockapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeFigures keeps the promises of the interface: pure, and every input reaches the output.
type fakeFigures struct{}

func fv(v string) compute.FigureValue { return compute.FigureValue{Value: v} }

func (fakeFigures) Household(in blockapi.HouseholdInput) blockapi.HouseholdResult {
	line := func(name string) []blockapi.MoneyLine {
		return []blockapi.MoneyLine{{
			Name: name, Currency: in.Values[0].Currency, Note: in.Alias,
			Amount: fv(in.Whose), Share: compute.FigureValue{Gap: "missing_input"},
		}}
	}
	o := in.Obligations[0]
	return blockapi.HouseholdResult{
		Overview: line(in.Values[0].Tag), CashAfter: line(in.MonthStart[0].Tag),
		Stress: line(o.Underlying + o.Currency), Obligations: line(o.Strike + o.Multiplier + o.Contracts),
		Income: line(o.PremiumIncome), MonthChange: line(in.UsdJpy),
		Accounts: line(string(in.Day)), Trend: in.History,
	}
}

func (fakeFigures) Trades(_ contracts.Day, in []blockapi.ExecIn) []blockapi.TradeLine {
	out := make([]blockapi.TradeLine, 0, len(in))
	for _, e := range in {
		out = append(out, blockapi.TradeLine{
			Account: e.Symbol, Time: e.Time, Kind: "open", Instrument: e.SecType + e.Right + e.Strike + e.Expiry + e.Side,
			Quantity: e.Quantity + e.PriorQuantity, Price: e.Price + e.Currency,
		})
	}
	return out
}

func (fakeFigures) AccountMove(in blockapi.AccountMoveIn) blockapi.AccountMoveOut {
	return blockapi.AccountMoveOut{
		Net: fv(in.Alias + in.Values[0].Value), DayAmount: fv(in.Prior[0].Value),
		DayPct: fv(in.Obligations[0].Strike), MarginBuffer: compute.FigureValue{Gap: "no_data"}, CashAfterAssignment: fv("0"),
	}
}

func (fakeFigures) Moves(in []blockapi.CloseIn) []blockapi.MoveLine {
	out := make([]blockapi.MoveLine, 0, len(in))
	for _, c := range in {
		out = append(out, blockapi.MoveLine{Name: c.Name, Symbol: c.Symbol, D1: fv(c.Closes[0]), D5: fv(c.Closes[1]), DataDay: c.Days[0]})
	}
	return out
}

func (fakeFigures) Glance(rows []blockapi.CompanyState) []blockapi.MoneyLine {
	out := make([]blockapi.MoneyLine, 0, len(rows))
	for _, r := range rows {
		out = append(out, blockapi.MoneyLine{Name: r.Symbol, Note: r.State + r.BandToday + r.BandThen})
	}
	return out
}

func (fakeFigures) Changes(rows []blockapi.CompanyState) []blockapi.ChangeLine {
	out := make([]blockapi.ChangeLine, 0, len(rows))
	for _, r := range rows {
		out = append(out, blockapi.ChangeLine{Symbol: r.Symbol, Text: r.State})
	}
	return out
}

var newFigures blockapi.NewPageFiguresFunc = func() blockapi.PageFigures { return fakeFigures{} }

var (
	testDay  = contracts.Day("2026-01-02")
	testVals = []blockapi.ValueIn{{Tag: "NetLiquidation", Value: "100", Currency: "USD"}}
	testObl  = []blockapi.ObligationIn{{Underlying: "AAA", Currency: "USD", Strike: "50", Multiplier: "100", Contracts: "1", PremiumIncome: "2"}}
)

func TestHouseholdContract(t *testing.T) {
	t.Parallel()
	hist := []blockapi.DayValue{{Day: testDay, NetLiquidation: "99", Currency: "USD"}}
	h := newFigures().Household(blockapi.HouseholdInput{
		Alias: "a", Whose: "w", Day: testDay, Values: testVals, MonthStart: testVals, History: hist, Obligations: testObl, UsdJpy: "150",
	})
	names := map[string]struct {
		got  []blockapi.MoneyLine
		name string
	}{
		"Overview": {h.Overview, "NetLiquidation"}, "CashAfter": {h.CashAfter, "NetLiquidation"},
		"Stress": {h.Stress, "AAAUSD"}, "Obligations": {h.Obligations, "501001"},
		"Income": {h.Income, "2"}, "MonthChange": {h.MonthChange, "150"}, "Accounts": {h.Accounts, string(testDay)},
	}
	for field, c := range names {
		if len(c.got) != 1 {
			t.Fatalf("%s = %+v", field, c.got)
		}
		want := blockapi.MoneyLine{Name: c.name, Currency: "USD", Note: "a", Amount: fv("w"), Share: compute.FigureValue{Gap: "missing_input"}}
		if c.got[0] != want {
			t.Fatalf("%s = %+v, want %+v", field, c.got[0], want)
		}
	}
	if len(h.Trend) != 1 || h.Trend[0] != hist[0] {
		t.Fatalf("Trend = %+v", h.Trend)
	}
}

func TestTradesContract(t *testing.T) {
	t.Parallel()
	exec := blockapi.ExecIn{
		Time: "t", Symbol: "S", SecType: "OPT", Right: "P", Strike: "1", Expiry: "e", Side: "BOT",
		Quantity: "2", Price: "3", Currency: "USD", PriorQuantity: "-1",
	}
	tr := newFigures().Trades(testDay, []blockapi.ExecIn{exec})
	want := blockapi.TradeLine{Account: "S", Time: "t", Kind: "open", Instrument: "OPTP1eBOT", Quantity: "2-1", Price: "3USD"}
	if len(tr) != 1 || tr[0] != want {
		t.Fatalf("Trades = %+v", tr)
	}
}

func TestAccountMoveContract(t *testing.T) {
	t.Parallel()
	m := newFigures().AccountMove(blockapi.AccountMoveIn{Alias: "a", Values: testVals, Prior: testVals, Obligations: testObl})
	want := blockapi.AccountMoveOut{
		Net: fv("a100"), DayAmount: fv("100"), DayPct: fv("50"),
		MarginBuffer: compute.FigureValue{Gap: "no_data"}, CashAfterAssignment: fv("0"),
	}
	if m != want {
		t.Fatalf("AccountMove = %+v", m)
	}
}

func TestMovesContract(t *testing.T) {
	t.Parallel()
	mv := newFigures().Moves([]blockapi.CloseIn{{Name: "N", Symbol: "Y", Closes: []string{"1", "2"}, Days: []contracts.Day{testDay}}})
	want := blockapi.MoveLine{Name: "N", Symbol: "Y", D1: fv("1"), D5: fv("2"), DataDay: testDay}
	if len(mv) != 1 || mv[0] != want {
		t.Fatalf("Moves = %+v", mv)
	}
}

func TestCompanyContract(t *testing.T) {
	t.Parallel()
	f := newFigures()
	rows := []blockapi.CompanyState{{Symbol: "Z", State: "band", BandToday: "x", BandThen: "y"}}
	if g := f.Glance(rows); len(g) != 1 || g[0].Name != "Z" || g[0].Note != "bandxy" {
		t.Fatalf("Glance = %+v", g)
	}
	if c := f.Changes(rows); len(c) != 1 || c[0] != (blockapi.ChangeLine{Symbol: "Z", Text: "band"}) {
		t.Fatalf("Changes = %+v", c)
	}
}
