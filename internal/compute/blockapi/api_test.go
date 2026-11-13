package blockapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
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

// fakeHoldings keeps the promises of HoldingFigures: pure, every input reaches the output, Risk never reads Mark.
type fakeHoldings struct{}

var noQuote = compute.FigureValue{Gap: "missing_quote"}

func (fakeHoldings) Holdings(in blockapi.HoldingsIn) blockapi.HoldingsResult {
	out := blockapi.HoldingsResult{TotalValue: fv(in.NetLiquidation), TotalUnrealised: fv(in.Currency)}
	for _, r := range in.Rows {
		l := blockapi.HoldingLine{Underlying: r.Underlying, SecType: r.SecType, Currency: r.Currency + in.UsdJpy}
		if r.Mark == "" {
			l.Value, l.Unrealised, l.Share = noQuote, noQuote, noQuote
		} else {
			l.Value, l.Unrealised, l.Share = fv(r.Quantity+r.Multiplier+r.Mark), fv(r.Cost+r.Currency), fv("1")
		}
		out.Rows = append(out.Rows, l)
	}
	return out
}

func (fakeHoldings) Risk(in blockapi.RiskIn) blockapi.RiskResult {
	out := blockapi.RiskResult{DeltaTotal: fv(in.Currency), VegaTotal: fv("0")}
	for _, s := range in.Stocks {
		out.Delta = append(out.Delta, blockapi.RiskLine{Key: s.Underlying + s.SecType, Value: fv(s.Quantity + s.Currency)})
	}
	for _, g := range in.Greeks {
		out.Delta = append(out.Delta, blockapi.RiskLine{Key: g.Underlying, Value: fv(g.Delta)})
		out.Vega = append(out.Vega, blockapi.RiskLine{Key: string(g.Expiry), Value: fv(g.Vega)})
	}
	return out
}

var newHoldings blockapi.NewHoldingFiguresFunc = func() blockapi.HoldingFigures { return fakeHoldings{} }

func TestHoldingsContract(t *testing.T) {
	t.Parallel()
	h := newHoldings().Holdings(blockapi.HoldingsIn{Currency: "USD", NetLiquidation: "1000", Rows: []blockapi.HoldingIn{
		{Underlying: "AAA", SecType: "STK", Currency: "USD", Quantity: "2", Multiplier: "1", Cost: "5", Mark: "6"},
		{Underlying: "BBB", SecType: "OPT", Currency: "USD", Quantity: "1", Multiplier: "100", Cost: "1"},
	}})
	want := []blockapi.HoldingLine{
		{Underlying: "AAA", SecType: "STK", Currency: "USD", Value: fv("216"), Unrealised: fv("5USD"), Share: fv("1")},
		{Underlying: "BBB", SecType: "OPT", Currency: "USD", Value: noQuote, Unrealised: noQuote, Share: noQuote},
	}
	if len(h.Rows) != 2 || h.Rows[0] != want[0] || h.Rows[1] != want[1] {
		t.Fatalf("Rows = %+v", h.Rows)
	}
	if h.TotalValue != fv("1000") || h.TotalUnrealised != fv("USD") {
		t.Fatalf("totals = %+v %+v", h.TotalValue, h.TotalUnrealised)
	}
}

func TestRiskContract(t *testing.T) {
	t.Parallel()
	r := newHoldings().Risk(blockapi.RiskIn{
		Currency: "USD",
		Stocks:   []blockapi.HoldingIn{{Underlying: "AAA", SecType: "STK", Currency: "USD", Quantity: "3"}},
		Greeks:   []blockapi.GreekIn{{Underlying: "AAA", Expiry: testDay, Delta: "-10", Vega: "4"}},
	})
	stock := blockapi.RiskLine{Key: "AAASTK", Value: fv("3USD")}
	opt := blockapi.RiskLine{Key: "AAA", Value: fv("-10")}
	if len(r.Delta) != 2 || r.Delta[0] != stock || r.Delta[1] != opt {
		t.Fatalf("Delta = %+v", r.Delta)
	}
	if len(r.Vega) != 1 || r.Vega[0] != (blockapi.RiskLine{Key: string(testDay), Value: fv("4")}) {
		t.Fatalf("Vega = %+v", r.Vega)
	}
	if r.DeltaTotal != fv("USD") || r.VegaTotal != fv("0") {
		t.Fatalf("totals = %+v %+v", r.DeltaTotal, r.VegaTotal)
	}
}

func TestPositionFieldsContract(t *testing.T) {
	t.Parallel()
	p := blockapi.PositionIn{Underlying: "U", SecType: "option", Currency: "USD", Quantity: "-1", Multiplier: "100", Mark: "2.5"}
	in := blockapi.HouseholdInput{Positions: []blockapi.PositionIn{p}, PositionsRead: true}
	if in.Positions[0] != p || !in.PositionsRead || p.Underlying != "U" || p.SecType != "option" || p.Currency != "USD" ||
		p.Quantity != "-1" || p.Multiplier != "100" || p.Mark != "2.5" {
		t.Fatalf("%+v", in)
	}
	h := newHoldings().Holdings(blockapi.HoldingsIn{Currency: "USD", UsdJpy: "150", Rows: []blockapi.HoldingIn{
		{Underlying: "A", SecType: "STK", Currency: "JPY", Quantity: "1", Multiplier: "1", Cost: "1", Mark: "1"},
	}})
	if h.Rows[0].Currency != "JPY150" {
		t.Fatalf("Currency = %q", h.Rows[0].Currency)
	}
}

// fakeTiles echoes every input field into a distinct output field.
type fakeTiles struct{}

func (fakeTiles) Tiles(in blockapi.OptionTilesIn) blockapi.OptionTilesOut {
	return blockapi.OptionTilesOut{
		SoldCount:      len(in.Positions),
		OpenPremium:    blockapi.UsdText{Value: string(in.Day), Dollars: in.Positions[0].PositionID},
		MaxLossUsd:     blockapi.UsdText{Value: in.Structures[0].ID, Dollars: "9"},
		OpenPremiumGap: "no_data", MaxLossGap: "unsupported_currency",
		OpenCount: len(in.Structures), Uncovered: 1, Unnumbered: 2, Unreadable: 3, OtherCurrency: 4,
	}
}

func TestOptionTilesContract(t *testing.T) {
	t.Parallel()
	var mk blockapi.NewOptionTilesFunc = func() blockapi.OptionTiles { return fakeTiles{} }
	o := mk().Tiles(blockapi.OptionTilesIn{
		Day:        testDay,
		Positions:  []compute.LedgerPosition{{PositionID: "a:1:1"}},
		Structures: []compute.Structure{{ID: "s1"}},
	})
	want := blockapi.OptionTilesOut{
		SoldCount:      1,
		OpenPremium:    blockapi.UsdText{Value: string(testDay), Dollars: "a:1:1"},
		MaxLossUsd:     blockapi.UsdText{Value: "s1", Dollars: "9"},
		OpenPremiumGap: "no_data", MaxLossGap: "unsupported_currency",
		OpenCount: 1, Uncovered: 1, Unnumbered: 2, Unreadable: 3, OtherCurrency: 4,
	}
	if o != want {
		t.Fatalf("got %+v", o)
	}
}

// fakeGlobal reads every input field and echoes it into a distinct output field.
type fakeGlobal struct{}

func (fakeGlobal) Global(in blockapi.GlobalIn) blockapi.GlobalResult {
	s := in.Sectors[0]
	return blockapi.GlobalResult{
		DataDay: in.Benchmark.Days[0],
		Proxies: []blockapi.MoveLine{{Name: in.Proxies[0].Name, D1: fv(in.Proxies[0].Closes[0]), DataDay: in.Day}},
		Sectors: []blockapi.SectorLine{{Name: s.Name, Symbol: s.Symbol, D1: fv("1"), D5: fv("5"), D20: compute.FigureValue{Gap: "no_data"}}},
		Rotation: &blockapi.RotationLine{
			Leaders5: []string{s.Name}, Laggard: s.Symbol, LaggardD5: in.Benchmark.Closes[0], Leader20: in.Benchmark.Name,
		},
	}
}

func TestGlobalContract(t *testing.T) {
	t.Parallel()
	var mk blockapi.NewGlobalFiguresFunc = func() blockapi.GlobalFigures { return fakeGlobal{} }
	var _ blockapi.NewGlobalFiguresFunc = pagefig.NewGlobal
	r := mk().Global(blockapi.GlobalIn{
		Day:       "2026-11-13",
		Proxies:   []blockapi.CloseIn{{Name: "Stocks", Closes: []string{"7"}}},
		Benchmark: blockapi.CloseIn{Name: "SPY", Closes: []string{"9"}, Days: []contracts.Day{"2026-11-12"}},
		Sectors:   []blockapi.CloseIn{{Name: "Energy", Symbol: "XLE"}},
	})
	if r.DataDay != "2026-11-12" || r.Proxies[0].Name != "Stocks" || r.Proxies[0].D1.Value != "7" || r.Proxies[0].DataDay != "2026-11-13" {
		t.Fatalf("proxies %+v", r)
	}
	s := r.Sectors[0]
	if s.Name != "Energy" || s.Symbol != "XLE" || s.D1.Value != "1" || s.D5.Value != "5" || s.D20.Gap != "no_data" {
		t.Fatalf("sector %+v", s)
	}
	rot := r.Rotation
	if rot == nil || rot.Leaders5[0] != "Energy" || rot.Laggard != "XLE" || rot.LaggardD5 != "9" || rot.Leader20 != "SPY" {
		t.Fatalf("rotation %+v", rot)
	}
}
