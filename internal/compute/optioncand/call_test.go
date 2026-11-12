package optioncand

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func callQuote(id, strike, bid, ask string, expiry contracts.Day) compute.OptionQuote {
	q := putQuote(id, strike, bid, expiry)
	q.Right, q.Ask = "C", ask
	return q
}

// callInput holds one call that passes every gate: strike 110 over spot 105, bid 2.00,
// 200 shares of alias A at average cost 100, no dividend.
func callInput() compute.CandidateInput {
	in := baseInput()
	in.Quotes = []compute.OptionQuote{callQuote("C110", "110", "2.00", "2.05", addDays(runDay, 30))}
	in.Accounts = []compute.AliasFunds{{Alias: "A", Shares: 200, AvgCost: "100", CaptureDay: runDay}}
	in.Dividend = &compute.DividendInfo{State: "no_value", RowDay: runDay}
	in.Kinds = []compute.CandidateKind{compute.KindCoveredCall}
	return in
}

func TestCallBaseInputIsOK(t *testing.T) {
	t.Parallel()
	c := only(t, eval(callInput()))
	if c.Kind != compute.KindCoveredCall || c.Alias != "A" || c.Status != "ok" || c.Rank != 1 {
		t.Fatalf("row %+v", c)
	}
	if c.BreakEven != "112" || c.AnnualizedYield != "0.231746" {
		t.Errorf("break-even %q yield %q", c.BreakEven, c.AnnualizedYield)
	}
	order := []string{"shares", "strike_floor", "quote", "events", "guard"}
	for i, g := range c.Gates {
		if g.Gate != order[i] {
			t.Errorf("gate %d is %s, want %s", i, g.Gate, order[i])
		}
	}
}

func TestCallFreeShares(t *testing.T) {
	t.Parallel()
	cases := []struct {
		shares, open int
		want         compute.GateStatus
	}{
		{200, 0, compute.GatePass},
		{200, 1, compute.GatePass},
		{200, 2, compute.GateBlock},
		{100, 1, compute.GateBlock},
		{99, 0, compute.GateBlock},
	}
	for _, tc := range cases {
		in := callInput()
		in.Accounts[0].Shares, in.Accounts[0].OpenShortCalls = tc.shares, tc.open
		c := only(t, eval(in))
		reason := ""
		if tc.want == compute.GateBlock {
			reason = "no_free_shares"
		}
		wantGate(t, c, "shares", tc.want, reason)
	}
}

func TestCallStrikeFloorAvgCost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		avg    string
		want   compute.GateStatus
		reason string
	}{
		{"110", compute.GatePass, ""},
		{"110.01", compute.GateBlock, "below_floor"},
		{"", compute.GateUnknown, "no_floor"},
	}
	for _, tc := range cases {
		in := callInput()
		in.Accounts[0].AvgCost = tc.avg
		c := only(t, eval(in))
		wantGate(t, c, "strike_floor", tc.want, tc.reason)
		if tc.want == compute.GateUnknown && c.Status != "unknown" {
			t.Errorf("status %s", c.Status)
		}
	}
}

func TestCallStrikeFloorSalePrice(t *testing.T) {
	t.Parallel()
	in := callInput()
	in.SalePrice = "112"
	in.Accounts[0].AvgCost = "100"
	c := only(t, eval(in))
	wantGate(t, c, "strike_floor", compute.GateBlock, "below_floor")
	if c.Status != "blocked" || c.SalePrice != "112" {
		t.Errorf("status %s sale %q", c.Status, c.SalePrice)
	}

	in = callInput()
	in.SalePrice = "110"
	in.Accounts[0].AvgCost = "120"
	wantGate(t, only(t, eval(in)), "strike_floor", compute.GatePass, "")

	in.Accounts[0].AvgCost = ""
	wantGate(t, only(t, eval(in)), "strike_floor", compute.GatePass, "")
}

func TestCallOneRowPerAlias(t *testing.T) {
	t.Parallel()
	in := callInput()
	in.Accounts = []compute.AliasFunds{
		{Alias: "B", Shares: 100, AvgCost: "100", CaptureDay: runDay},
		{Alias: "A", Shares: 0, AvgCost: "100", CaptureDay: runDay},
	}
	c := only(t, eval(in))
	if c.Alias != "B" {
		t.Errorf("alias %q, want B", c.Alias)
	}

	in.Accounts[1].Shares = 100
	rows := eval(in)
	if len(rows) != 2 || rows[0].Alias != "A" || rows[1].Alias != "B" {
		t.Fatalf("rows %+v", rows)
	}
	if rows[0].Rank != 1 || rows[1].Rank != 2 {
		t.Errorf("ranks %d %d", rows[0].Rank, rows[1].Rank)
	}

	in.Accounts = nil
	if rows := eval(in); len(rows) != 0 {
		t.Errorf("no account gave %d rows", len(rows))
	}
}

func TestCallRankingByYield(t *testing.T) {
	t.Parallel()
	in := callInput()
	in.Quotes = append(in.Quotes, callQuote("C115", "115", "3.00", "3.05", addDays(runDay, 30)))
	rows := eval(in)
	if len(rows) != 2 || rows[0].InstrumentID != "C115" || rows[0].Rank != 1 || rows[1].Rank != 2 {
		t.Fatalf("rows %+v", rows)
	}
}

func TestCallKindsAreSeparate(t *testing.T) {
	t.Parallel()
	in := callInput()
	in.Quotes = append(in.Quotes, putQuote("P100", "100", "2.00", addDays(runDay, 30)))
	in.Kinds = []compute.CandidateKind{compute.KindShortPut}
	if c := only(t, eval(in)); c.Kind != compute.KindShortPut {
		t.Errorf("kind %s", c.Kind)
	}
	in.Kinds = nil
	if c := only(t, eval(in)); c.Kind != compute.KindShortPut {
		t.Errorf("empty kinds gave %s", c.Kind)
	}
	in.Kinds = []compute.CandidateKind{compute.KindShortPut, compute.KindCoveredCall}
	rows := eval(in)
	if len(rows) != 2 || rows[0].Kind != compute.KindShortPut || rows[1].Kind != compute.KindCoveredCall {
		t.Fatalf("rows %+v", rows)
	}
}

func TestCoveredCallZeroOpenInterestMark(t *testing.T) {
	t.Parallel()
	in := callInput()
	in.Quotes[0].OpenInterest = 0
	c := only(t, eval(in))
	wantGate(t, c, "quote", compute.GatePass, "")
	if c.Status != "ok" || !hasMark(c, "no_open_interest") {
		t.Errorf("status %s marks %v", c.Status, c.Marks)
	}
}
