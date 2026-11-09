package optioncand

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func dividendInput(bid string) compute.CandidateInput {
	in := callInput()
	in.Spot = "110"
	in.Quotes = []compute.OptionQuote{callQuote("C100", "100", bid, "10.65", addDays(runDay, 30))}
	in.Dividend = &compute.DividendInfo{State: "present", NextExDate: addDays(runDay, 10), Amount: "0.50", RowDay: runDay}
	return in
}

func TestGuardExtrinsicBelowDividend(t *testing.T) {
	t.Parallel()
	in := dividendInput("10.40")
	in.Quotes[0].Ask = "10.45"
	c := only(t, eval(in))
	if c.Extrinsic != "0.4" {
		t.Errorf("extrinsic %q, want 0.4", c.Extrinsic)
	}
	wantGate(t, c, "guard", compute.GateBlock, "early_assignment_risk")
	if c.Status != "blocked" {
		t.Errorf("status %s", c.Status)
	}

	in = dividendInput("10.60")
	c = only(t, eval(in))
	if c.Extrinsic != "0.6" {
		t.Errorf("extrinsic %q, want 0.6", c.Extrinsic)
	}
	wantGate(t, c, "guard", compute.GatePass, "")

	// An ex-date after the expiry is outside the contract.
	in = dividendInput("10.40")
	in.Quotes[0].Ask = "10.45"
	in.Dividend.NextExDate = addDays(runDay, 31)
	wantGate(t, only(t, eval(in)), "guard", compute.GatePass, "")

	// The factor scales the amount: 0.6 < 0.5 x 1.5.
	in = dividendInput("10.60")
	p := testParams()
	p.DivGuardFactor = "1.5"
	c = only(t, New().Evaluate(in, p))
	wantGate(t, c, "guard", compute.GateBlock, "early_assignment_risk")
}

func TestGuardNoValuePasses(t *testing.T) {
	t.Parallel()
	in := callInput()
	in.Dividend = &compute.DividendInfo{State: "no_value", RowDay: runDay}
	c := only(t, eval(in))
	wantGate(t, c, "guard", compute.GatePass, "")
	found := false
	for _, m := range c.Marks {
		found = found || m == "no_dividend"
	}
	if !found || c.Status != "ok" {
		t.Errorf("marks %v status %s", c.Marks, c.Status)
	}
}

func TestGuardUnknownStaleRow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		div    *compute.DividendInfo
		reason string
	}{
		{"no row", nil, "no_dividend_info"},
		{"error", &compute.DividendInfo{State: "error", RowDay: runDay}, "no_dividend_info"},
		{"unsupported", &compute.DividendInfo{State: "unsupported", RowDay: runDay}, "no_dividend_info"},
		{"stale", &compute.DividendInfo{State: "no_value", RowDay: "2026-11-02"}, "stale_dividend"},
		{"past ex-date", &compute.DividendInfo{State: "present", NextExDate: "2026-11-02", Amount: "0.5", RowDay: runDay}, "stale_dividend"},
		{"no amount", &compute.DividendInfo{State: "present", NextExDate: addDays(runDay, 5), RowDay: runDay}, "no_dividend_info"},
	}
	for _, tc := range cases {
		in := callInput()
		in.Dividend = tc.div
		c := only(t, eval(in))
		wantGate(t, c, "guard", compute.GateUnknown, tc.reason)
		if c.Status != "unknown" {
			t.Errorf("%s: status %s", tc.name, c.Status)
		}
	}
}

func TestGuardHeldCall(t *testing.T) {
	t.Parallel()
	held := func(bid string) compute.CandidateInput {
		in := dividendInput(bid)
		in.Kinds = []compute.CandidateKind{compute.KindHeldCallGuard}
		in.OpenShortIDs = []string{"C100"}
		// 3 days to expiry: outside the DTE window, still judged.
		in.Quotes[0].Expiry = addDays(runDay, 3)
		in.Dividend.NextExDate = addDays(runDay, 2)
		in.Quotes = append(in.Quotes, callQuote("C105", "105", "5.00", "5.05", addDays(runDay, 30)))
		in.Accounts = []compute.AliasFunds{
			{Alias: "B", Shares: 100, OpenShortCalls: 1, CaptureDay: runDay},
			{Alias: "A", Shares: 100, CaptureDay: runDay},
		}
		return in
	}
	c := only(t, eval(held("10.40")))
	if c.Kind != compute.KindHeldCallGuard || c.GuardVerdict != "risk" || c.Alias != "B" || c.Status != "blocked" {
		t.Fatalf("row %+v", c)
	}
	if len(c.Gates) != 1 || c.Gates[0].Gate != "guard" || c.DTE != 3 {
		t.Errorf("gates %+v dte %d", c.Gates, c.DTE)
	}

	c = only(t, eval(held("10.60")))
	if c.GuardVerdict != "ok" || c.Status != "ok" {
		t.Errorf("verdict %s status %s", c.GuardVerdict, c.Status)
	}

	in := held("10.60")
	in.Dividend = nil
	c = only(t, eval(in))
	if c.GuardVerdict != "unknown" || c.Status != "unknown" {
		t.Errorf("verdict %s status %s", c.GuardVerdict, c.Status)
	}

	in = held("10.60")
	in.Quotes[0].Bid = ""
	if c = only(t, eval(in)); c.GuardVerdict != "unknown" {
		t.Errorf("no bid verdict %s", c.GuardVerdict)
	}
}
