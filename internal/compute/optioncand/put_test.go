package optioncand

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const runDay contracts.Day = "2026-11-09" // a Monday

func addDays(d contracts.Day, n int) contracts.Day {
	t, _ := time.Parse(dayLayout, string(d))
	return contracts.Day(t.AddDate(0, 0, n).Format(dayLayout))
}

// weekdays lists every Monday to Friday from 2026-09-01 to 2026-11-09 (no holiday).
func weekdays() []contracts.Day {
	var out []contracts.Day
	for d := contracts.Day("2026-09-01"); d <= runDay; d = addDays(d, 1) {
		t, _ := time.Parse(dayLayout, string(d))
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d)
		}
	}
	return out
}

func testParams() compute.CandidateParams {
	return compute.CandidateParams{
		MinDTE: 21, MaxDTE: 90, MinBidSize: 1, MaxSpread: "0.10", MinOpenInterest: 10,
		DivGuardFactor: "1.0", MaxRateAgeDays: 7, MaxEventAgeSessions: 2, MinReportGapDays: 60,
	}
}

func putQuote(id, strike, bid string, expiry contracts.Day) compute.OptionQuote {
	return compute.OptionQuote{
		InstrumentID: id, Expiry: expiry, Right: "P", Strike: strike, Multiplier: "100",
		Bid: bid, Ask: "", BidSize: 5, OpenInterest: 50, MarketDataType: 1, QuoteOK: true,
	}
}

// baseInput holds one put that passes every gate: strike 100, bid 2.00, buy price 98.
func baseInput() compute.CandidateInput {
	q := putQuote("P100", "100", "2.00", addDays(runDay, 30))
	q.Ask = "2.05"
	return compute.CandidateInput{
		RunDay: runDay, ChainDay: runDay, SessionDays: weekdays(), Underlying: "XYZ", Spot: "105",
		Quotes: []compute.OptionQuote{q}, BuyPrice: "98",
		Earnings: &compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: runDay},
		Rate:     "4.20", RateDay: runDay,
		Accounts: []compute.AliasFunds{{Alias: "A", Funds: "20000", Currency: "USD", CaptureDay: runDay}},
		Kinds:    []compute.CandidateKind{compute.KindShortPut},
	}
}

func eval(in compute.CandidateInput) []compute.Candidate {
	return New().Evaluate(in, testParams())
}

func only(t *testing.T, rows []compute.Candidate) compute.Candidate {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	return rows[0]
}

func gateOf(t *testing.T, c compute.Candidate, name string) compute.GateResult {
	t.Helper()
	for _, g := range c.Gates {
		if g.Gate == name {
			return g
		}
	}
	t.Fatalf("no gate %s", name)
	return compute.GateResult{}
}

func wantGate(t *testing.T, c compute.Candidate, name string, st compute.GateStatus, reason string) {
	t.Helper()
	g := gateOf(t, c, name)
	if g.Status != st || g.Reason != reason {
		t.Errorf("gate %s = %s/%q, want %s/%q", name, g.Status, g.Reason, st, reason)
	}
}

func TestBaseInputIsOK(t *testing.T) {
	t.Parallel()
	c := only(t, eval(baseInput()))
	if c.Status != "ok" || c.Rank != 1 {
		t.Fatalf("status %s rank %d gates %+v", c.Status, c.Rank, c.Gates)
	}
	if c.BreakEven != "98" || c.AnnualizedYield != "0.243333" {
		t.Errorf("break-even %q yield %q", c.BreakEven, c.AnnualizedYield)
	}
	order := []string{"buy_price", "quote", "events", "funding", "cash_rate"}
	for i, g := range c.Gates {
		if g.Gate != order[i] {
			t.Errorf("gate %d is %s, want %s", i, g.Gate, order[i])
		}
	}
	if len(c.Marks) != 1 || c.Marks[0] != "earnings_unconfirmed" {
		t.Errorf("marks %v", c.Marks)
	}
}

func TestBuyPriceBoundary(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.BuyPrice = "98.00"
	in.Quotes[0].Strike, in.Quotes[0].Bid = "100", "2.00"
	wantGate(t, only(t, eval(in)), "buy_price", compute.GatePass, "")
	in.Quotes[0].Bid = "1.99"
	c := only(t, eval(in))
	wantGate(t, c, "buy_price", compute.GateBlock, "above_buy_price")
	if c.Status != "blocked" {
		t.Errorf("status %s", c.Status)
	}
}

func hasMark(c compute.Candidate, m string) bool {
	for _, x := range c.Marks {
		if x == m {
			return true
		}
	}
	return false
}

func TestNoBuyPriceIsMarkNotBlock(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.BuyPrice = ""
	c := only(t, eval(in))
	wantGate(t, c, "buy_price", compute.GatePass, "")
	if !hasMark(c, "no_buy_price") || c.Status != "ok" || c.Rank != 1 {
		t.Errorf("marks %v status %s rank %d", c.Marks, c.Status, c.Rank)
	}
}

func TestBuyPriceSetStillBlocks(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.BuyPrice = "98.00"
	in.Quotes[0].Strike, in.Quotes[0].Bid = "100", "1.99"
	c := only(t, eval(in))
	wantGate(t, c, "buy_price", compute.GateBlock, "above_buy_price")
	if hasMark(c, "no_buy_price") {
		t.Errorf("marks %v", c.Marks)
	}
}

func TestOpenInterestAbsentIsMark(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.Quotes[0].OpenInterest = 0
	c := only(t, eval(in))
	wantGate(t, c, "quote", compute.GatePass, "")
	if !hasMark(c, "no_open_interest") || c.Status != "ok" {
		t.Errorf("marks %v status %s", c.Marks, c.Status)
	}
	in.Quotes[0].OpenInterest = 9
	c = only(t, eval(in))
	wantGate(t, c, "quote", compute.GateBlock, "low_open_interest")
	if hasMark(c, "no_open_interest") {
		t.Errorf("marks %v", c.Marks)
	}
	in.Quotes[0].OpenInterest = 10
	c = only(t, eval(in))
	wantGate(t, c, "quote", compute.GatePass, "")
	if hasMark(c, "no_open_interest") {
		t.Errorf("marks %v", c.Marks)
	}
}

func TestQuoteGate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		edit   func(*compute.OptionQuote)
		status compute.GateStatus
		reason string
	}{
		{"ok", func(*compute.OptionQuote) {}, compute.GatePass, ""},
		{"row not ok", func(q *compute.OptionQuote) { q.QuoteOK = false }, compute.GateBlock, "no_quote"},
		{"bid zero", func(q *compute.OptionQuote) { q.Bid = "0" }, compute.GateBlock, "no_bid"},
		{"bid absent", func(q *compute.OptionQuote) { q.Bid = "" }, compute.GateBlock, "no_bid"},
		{"ask absent", func(q *compute.OptionQuote) { q.Ask = "" }, compute.GateBlock, "no_ask"},
		{"small size", func(q *compute.OptionQuote) { q.BidSize = 0 }, compute.GateBlock, "small_bid_size"},
		// spread 0.30 / mid 2.15 is above 10 percent
		{"wide", func(q *compute.OptionQuote) { q.Ask = "2.30" }, compute.GateBlock, "wide_spread"},
		// spread 0.20 / mid 2.10 = 9.5 percent
		{"spread inside", func(q *compute.OptionQuote) { q.Ask = "2.20" }, compute.GatePass, ""},
		{"low open interest", func(q *compute.OptionQuote) { q.OpenInterest = 9 }, compute.GateBlock, "low_open_interest"},
		{"open interest at limit", func(q *compute.OptionQuote) { q.OpenInterest = 10 }, compute.GatePass, ""},
		{"delayed", func(q *compute.OptionQuote) { q.MarketDataType = 3 }, compute.GateUnknown, "delayed_data"},
		{"frozen", func(q *compute.OptionQuote) { q.MarketDataType = 2 }, compute.GatePass, ""},
		{"block outranks delayed", func(q *compute.OptionQuote) { q.MarketDataType = 4; q.OpenInterest = 9 }, compute.GateBlock, "low_open_interest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := baseInput()
			tc.edit(&in.Quotes[0])
			wantGate(t, only(t, eval(in)), "quote", tc.status, tc.reason)
		})
	}
}

func TestFundingPerAlias(t *testing.T) {
	t.Parallel()
	in := baseInput() // needs 100 x 100 = 10000
	in.Accounts = []compute.AliasFunds{
		{Alias: "B", Funds: "20000", Currency: "USD", CaptureDay: runDay},
		{Alias: "A", Funds: "9999.99", Currency: "USD", CaptureDay: runDay},
	}
	c := only(t, eval(in))
	wantGate(t, c, "funding", compute.GatePass, "")
	if c.Fits["A"] != "no_fit" || c.Fits["B"] != "fit" {
		t.Errorf("fits %v", c.Fits)
	}
	in.Accounts[0].Funds = "10000" // exactly enough
	if c := only(t, eval(in)); c.Fits["B"] != "fit" {
		t.Errorf("fits %v", c.Fits)
	}
	in.Accounts = []compute.AliasFunds{
		{Alias: "A", Funds: "9999", Currency: "USD", CaptureDay: runDay},
		{Alias: "B", Funds: "5000", Currency: "USD", CaptureDay: runDay},
	}
	wantGate(t, only(t, eval(in)), "funding", compute.GateBlock, "no_fit")
	// a capture two sessions old (Thursday) is stale
	in.Accounts = []compute.AliasFunds{{Alias: "A", Funds: "20000", Currency: "USD", CaptureDay: "2026-11-05"}}
	wantGate(t, only(t, eval(in)), "funding", compute.GateUnknown, "stale_account")
	in.Accounts = nil
	wantGate(t, only(t, eval(in)), "funding", compute.GateUnknown, "no_account")
}

func TestNoFxIsUnknown(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.Accounts = []compute.AliasFunds{{Alias: "A", Funds: "20000000", Currency: "JPY", CaptureDay: runDay}}
	c := only(t, eval(in))
	wantGate(t, c, "funding", compute.GateUnknown, "no_fx")
	if c.Fits["A"] != "unknown" || c.Status != "unknown" {
		t.Errorf("fits %v status %s", c.Fits, c.Status)
	}
}

// yieldInput builds a put whose yield is bid/strike x 365/73 = 5 x bid/strike.
func yieldInput(bid, rate string) compute.CandidateInput {
	in := baseInput()
	in.BuyPrice = "100"
	in.Quotes[0].Expiry = addDays(runDay, 73)
	in.Quotes[0].Bid, in.Quotes[0].Ask = bid, bid
	in.Rate = rate
	return in
}

func TestCashRatePercent(t *testing.T) {
	t.Parallel()
	// yield 0.05 against 4.20 percent passes; 0.04 blocks; equal blocks
	wantGate(t, only(t, eval(yieldInput("1.00", "4.20"))), "cash_rate", compute.GatePass, "")
	wantGate(t, only(t, eval(yieldInput("0.80", "4.20"))), "cash_rate", compute.GateBlock, "yield_below_cash")
	wantGate(t, only(t, eval(yieldInput("1.00", "5.00"))), "cash_rate", compute.GateBlock, "yield_below_cash")
	in := yieldInput("1.00", "")
	wantGate(t, only(t, eval(in)), "cash_rate", compute.GateUnknown, "no_rate")
	in = yieldInput("1.00", "4.20")
	in.RateDay = addDays(runDay, -8)
	wantGate(t, only(t, eval(in)), "cash_rate", compute.GateUnknown, "stale_rate")
	in.RateDay = addDays(runDay, -7)
	wantGate(t, only(t, eval(in)), "cash_rate", compute.GatePass, "")
}

func TestUnknownIsNotPass(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.Earnings = nil
	c := only(t, eval(in))
	if c.Status != "unknown" || c.Rank != 0 {
		t.Fatalf("status %s rank %d", c.Status, c.Rank)
	}
	wantGate(t, c, "events", compute.GateUnknown, "no_earnings")
	// a block outranks an unknown
	in.BuyPrice = "50"
	if c := only(t, eval(in)); c.Status != "blocked" {
		t.Errorf("status %s", c.Status)
	}
	in = baseInput()
	in.Quotes[0].Strike = "abc"
	if rows := eval(in); len(rows) != 0 {
		t.Errorf("bad strike gave %d rows", len(rows))
	}
}
