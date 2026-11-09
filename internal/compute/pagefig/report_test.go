package pagefig

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func opt(side, qty, price, prior string) blockapi.ExecIn {
	return blockapi.ExecIn{
		Time: "2026-11-09T15:00:00Z", Symbol: "AAPL", SecType: "option", Right: "put", Strike: "150",
		Expiry: "2026-12-18", Side: side, Quantity: qty, Price: price, Currency: "USD", PriorQuantity: prior,
	}
}

func kindOf(t *testing.T, e blockapi.ExecIn) string {
	t.Helper()
	got := New().Trades(contracts.Day("2026-11-09"), []blockapi.ExecIn{e})
	if len(got) != 1 {
		t.Fatalf("lines: %+v", got)
	}
	return got[0].Kind
}

func TestSellFromZeroIsOpen(t *testing.T) {
	t.Parallel()
	if k := kindOf(t, opt("SLD", "1", "2.50", "0")); k != "open" {
		t.Fatalf("kind = %q", k)
	}
	got := New().Trades("2026-11-09", []blockapi.ExecIn{opt("SLD", "1", "2.50", "0")})[0]
	if got.Instrument != "AAPL 2026-12-18 150 put" || got.Quantity != "1" || got.Price != "2.50" {
		t.Fatalf("line: %+v", got)
	}
}

func TestBuyOnShortIsClose(t *testing.T) {
	t.Parallel()
	if k := kindOf(t, opt("BOT", "1", "1.00", "-2")); k != "close" {
		t.Fatalf("kind = %q", k)
	}
	if k := kindOf(t, opt("BOT", "1", "1.00", "2")); k != "open" {
		t.Fatalf("buy on long: %q", k)
	}
	if k := kindOf(t, opt("BOT", "2", "0", "-2")); k != "expire" {
		t.Fatalf("close at zero price: %q", k)
	}
}

func TestNoPriorGivesGap(t *testing.T) {
	t.Parallel()
	if k := kindOf(t, opt("SLD", "1", "2.50", "")); k != "" {
		t.Fatalf("kind = %q, want none", k)
	}
}

func TestMeterCashAfterAssignment(t *testing.T) {
	t.Parallel()
	in := blockapi.AccountMoveIn{
		Alias: "main",
		Values: []blockapi.ValueIn{
			{Tag: "NetLiquidation", Value: "1100", Currency: "USD"},
			{Tag: "TotalCashValue", Value: "20000", Currency: "USD"},
			{Tag: "ExcessLiquidity", Value: "275", Currency: "USD"},
		},
		Prior:       vals("1000", "0", "USD"),
		Obligations: []blockapi.ObligationIn{put("100", "100", "1", "30")},
	}
	got := New().AccountMove(in)
	want := blockapi.AccountMoveOut{
		Net:                 compute.FigureValue{Value: "1100.00"},
		DayAmount:           compute.FigureValue{Value: "100.00"},
		DayPct:              compute.FigureValue{Value: "10.00"},
		MarginBuffer:        compute.FigureValue{Value: "25.00"},
		CashAfterAssignment: compute.FigureValue{Value: "10000.00"},
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	in.Prior = nil
	in.Obligations = []blockapi.ObligationIn{{Underlying: "AAPL", Currency: "USD", Strike: "x"}}
	got = New().AccountMove(in)
	if got.DayAmount.Gap != "missing_input" || got.DayPct.Gap != "missing_input" || got.CashAfterAssignment.Gap != "missing_input" {
		t.Fatalf("gaps: %+v", got)
	}
}

func series(name string, days []string, closes ...string) blockapi.CloseIn {
	in := blockapi.CloseIn{Name: name, Symbol: name, Closes: closes}
	for _, d := range days {
		in.Days = append(in.Days, contracts.Day(d))
	}
	return in
}

var sixDays = []string{"2026-11-09", "2026-11-06", "2026-11-05", "2026-11-04", "2026-11-03", "2026-11-02"}

func TestFiveDayNeedsSixCloses(t *testing.T) {
	t.Parallel()
	got := New().Moves([]blockapi.CloseIn{series("S&P 500", sixDays[:5], "110", "100", "100", "100", "100")})
	if len(got) != 1 || got[0].D1 != (compute.FigureValue{Value: "10.00"}) || got[0].D5.Gap != "no_data" {
		t.Fatalf("five closes: %+v", got)
	}
	got = New().Moves([]blockapi.CloseIn{series("S&P 500", sixDays, "110", "100", "100", "100", "100", "125")})
	if got[0].D5 != (compute.FigureValue{Value: "-12.00"}) || got[0].DataDay != "2026-11-09" {
		t.Fatalf("six closes: %+v", got)
	}
}

func TestStaleCloseIsGap(t *testing.T) {
	t.Parallel()
	old := []string{"2026-11-02", "2026-10-30", "2026-10-29", "2026-10-28", "2026-10-27", "2026-10-26"}
	got := New().Moves([]blockapi.CloseIn{
		series("fresh", sixDays, "110", "100", "100", "100", "100", "100"),
		series("old", old, "110", "100", "100", "100", "100", "100"),
	})
	if got[0].D1.Gap != "" {
		t.Fatalf("fresh: %+v", got[0])
	}
	if got[1].D1.Gap != "stale_input" || got[1].D5.Gap != "stale_input" || got[1].DataDay != "2026-11-02" {
		t.Fatalf("old: %+v", got[1])
	}
}
