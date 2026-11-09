package pagefig

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func vals(net, cash, cur string) []blockapi.ValueIn {
	return []blockapi.ValueIn{
		{Tag: "NetLiquidation", Value: net, Currency: cur},
		{Tag: "TotalCashValue", Value: cash, Currency: cur},
	}
}

func put(strike, mult, n, premium string) blockapi.ObligationIn {
	return blockapi.ObligationIn{Underlying: "AAPL", Currency: "USD", Strike: strike, Multiplier: mult, Contracts: n, PremiumIncome: premium}
}

func TestJpyNeedsRate(t *testing.T) {
	t.Parallel()
	in := blockapi.HouseholdInput{Alias: "main", Values: vals("1000000", "500000", "JPY")}
	got := New().Household(in).Overview
	if len(got) != 1 || got[0].Amount.Gap != "unsupported_currency" || got[0].Amount.Value != "" {
		t.Fatalf("without rate: %+v", got)
	}
	in.UsdJpy = "150"
	got = New().Household(in).Overview
	if got[0].Amount != (compute.FigureValue{Value: "1000000.00"}) || got[0].Note != "≈USD 6667" {
		t.Fatalf("with rate: %+v", got)
	}
}

func TestMonthChangeNeedsMonthStart(t *testing.T) {
	t.Parallel()
	in := blockapi.HouseholdInput{Values: vals("1100", "500", "USD"), Obligations: []blockapi.ObligationIn{put("100", "100", "1", "30")}}
	got := New().Household(in).MonthChange
	if len(got) != 1 || got[0].Amount.Gap != "missing_input" {
		t.Fatalf("no start: %+v", got)
	}
	in.MonthStart = vals("1000", "400", "USD")
	got = New().Household(in).MonthChange
	want := map[string]compute.FigureValue{
		"月初": {Value: "1000.00"}, "已平仓期权结果": {Value: "30.00"}, "交易结果": {Gap: "missing_quote"},
		"其他": {Value: "70.00"}, "月末": {Value: "1100.00"},
	}
	if len(got) != len(want) {
		t.Fatalf("lines: %+v", got)
	}
	for _, l := range got {
		if l.Amount != want[l.Name] {
			t.Errorf("%s = %+v, want %+v", l.Name, l.Amount, want[l.Name])
		}
	}
}

func TestCashAfterIsCashMinusObligations(t *testing.T) {
	t.Parallel()
	in := blockapi.HouseholdInput{
		Values:      vals("90000", "50000", "USD"),
		Obligations: []blockapi.ObligationIn{put("100", "100", "2", "10"), put("45.5", "100", "1", "5")},
	}
	got := New().Household(in).CashAfter
	// 50000 - 100*100*2 - 45.5*100*1 = 25450
	if len(got) != 1 || got[0].Amount != (compute.FigureValue{Value: "25450.00"}) {
		t.Fatalf("cash after: %+v", got)
	}
	in.Values = []blockapi.ValueIn{{Tag: "NetLiquidation", Value: "1", Currency: "USD"}}
	got = New().Household(in).CashAfter
	if got[0].Amount.Gap != "missing_input" {
		t.Fatalf("no cash: %+v", got)
	}
}

func TestStressStepsAreConstants(t *testing.T) {
	t.Parallel()
	for _, in := range []blockapi.HouseholdInput{{}, {Values: vals("90000", "50000", "USD"), Obligations: []blockapi.ObligationIn{put("100", "100", "1", "1")}}} {
		got := New().Household(in).Stress
		steps := map[string]bool{}
		for _, l := range got {
			if l.Amount.Gap == "missing_quote" {
				steps[l.Name] = true
			}
		}
		for _, n := range []string{"标的跌 10%", "标的跌 20%", "标的跌 30%"} {
			if !steps[n] {
				t.Errorf("missing step %q in %+v", n, got)
			}
		}
	}
	got := New().Household(blockapi.HouseholdInput{Values: vals("1", "300", "USD"), Obligations: []blockapi.ObligationIn{put("1", "100", "1", "1")}}).Stress
	if got[0].Name != "全部指派后现金" || got[0].Amount.Value != "200.00" {
		t.Fatalf("assignment row: %+v", got[0])
	}
}

func TestTrendAscending(t *testing.T) {
	t.Parallel()
	var h []blockapi.DayValue
	for d := 95; d >= 1; d-- { // newest first, 95 days
		day := contracts.Day("2026-01-" + string(rune('0'+d/10)) + string(rune('0'+d%10)))
		h = append(h, blockapi.DayValue{Day: day, NetLiquidation: "1", Currency: "USD"})
	}
	got := New().Household(blockapi.HouseholdInput{History: h}).Trend
	if len(got) != 90 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].Day != "2026-01-06" || got[89].Day != "2026-01-95" {
		t.Fatalf("ends %s %s", got[0].Day, got[89].Day)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Day >= got[i].Day {
			t.Fatalf("not ascending at %d", i)
		}
	}
}

func TestStockCellsAreMissingQuote(t *testing.T) {
	t.Parallel()
	got := New().Household(blockapi.HouseholdInput{Values: vals("1000", "250", "USD")}).Accounts
	want := map[string]compute.FigureValue{
		"净清算值": {Value: "1000.00"}, "现金": {Value: "250.00"},
		"股票市值": {Gap: "missing_quote"}, "期权市值": {Gap: "missing_quote"},
	}
	if len(got) != len(want) {
		t.Fatalf("rows: %+v", got)
	}
	for _, l := range got {
		if l.Amount != want[l.Name] {
			t.Errorf("%s = %+v", l.Name, l.Amount)
		}
	}
}

func TestNewIsPageFigures(t *testing.T) {
	t.Parallel()
	f := New()
	if len(f.Trades("2026-01-01", nil)) != 0 || len(f.Moves(nil)) != 0 || len(f.Glance(nil)) != 0 || len(f.Changes(nil)) != 0 {
		t.Fatal("other methods are empty until entry 3")
	}
	if f.AccountMove(blockapi.AccountMoveIn{}) != (blockapi.AccountMoveOut{}) {
		t.Fatal("AccountMove is empty")
	}
}

func TestJpyIncomeNeedsRate(t *testing.T) {
	t.Parallel()
	o := blockapi.ObligationIn{Underlying: "7203", Currency: "JPY", Strike: "2000", Multiplier: "100", Contracts: "1", PremiumIncome: "5000"}
	in := blockapi.HouseholdInput{Values: vals("1000000", "500000", "JPY"), Obligations: []blockapi.ObligationIn{o}}
	got := New().Household(in).Income
	if len(got) != 1 || got[0].Amount.Gap != "unsupported_currency" {
		t.Fatalf("JPY income without rate: %+v", got)
	}
	in.UsdJpy = "150"
	got = New().Household(in).Income
	if got[0].Amount.Value != "5000.00" || got[0].Note != "≈USD 33" {
		t.Fatalf("JPY income with rate: %+v", got)
	}
}

func TestTrendDropsDaysAfterDataDay(t *testing.T) {
	t.Parallel()
	h := []blockapi.DayValue{{Day: "2026-01-03"}, {Day: "2026-01-02"}, {Day: "2026-01-01"}}
	got := New().Household(blockapi.HouseholdInput{Day: "2026-01-02", History: h}).Trend
	if len(got) != 2 || got[1].Day != "2026-01-02" {
		t.Fatalf("trend: %+v", got)
	}
}

func TestIncomeWithUnreadablePremiumIsGap(t *testing.T) {
	t.Parallel()
	in := blockapi.HouseholdInput{
		Values:      vals("90000", "50000", "USD"),
		Obligations: []blockapi.ObligationIn{put("100", "100", "1", "30"), put("50", "100", "1", "x")},
	}
	got := New().Household(in).Income
	if len(got) != 1 || got[0].Amount.Gap == "" {
		t.Fatalf("income with an unreadable premium must be a gap, got %+v", got)
	}
}
