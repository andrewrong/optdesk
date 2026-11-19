package pagefig

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func famVals(cur, net, cash string) []blockapi.ValueIn {
	v := []blockapi.ValueIn{{Tag: "NetLiquidation", Value: net, Currency: cur}}
	if cash != "" {
		v = append(v, blockapi.ValueIn{Tag: "TotalCashValue", Value: cash, Currency: cur})
	}
	return v
}

func famNL(cur, net string) []blockapi.ValueIn { return famVals(cur, net, "") }

func famAcctIn(alias, day string, v, prior, month []blockapi.ValueIn) blockapi.FamilyAccountIn {
	return blockapi.FamilyAccountIn{Alias: alias, Day: day, Values: v, Prior: prior, MonthStart: month}
}

func famFV(v string) compute.FigureValue { return compute.FigureValue{Value: v} }

func famEq(t *testing.T, name string, got, want compute.FigureValue) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %+v, want %+v", name, got, want)
	}
}

func TestFamilySumsTwoUsd(t *testing.T) {
	t.Parallel()
	r := NewFamily().Family(blockapi.FamilyIn{Accounts: []blockapi.FamilyAccountIn{
		famAcctIn("a", "2026-11-18", famVals("USD", "100.10", "10"), famNL("USD", "90"), famNL("USD", "80")),
		famAcctIn("b", "2026-11-18", famVals("USD", "200.20", "20"), famNL("USD", "210"), famNL("USD", "200")),
	}})
	famEq(t, "Net", r.Net, famFV("300.30"))
	famEq(t, "Cash", r.Cash, famFV("30.00"))
	famEq(t, "CashShare", r.CashShare, famFV("9.99"))
	famEq(t, "DayAmount", r.DayAmount, famFV("0.30"))
	famEq(t, "MonthAmount", r.MonthAmount, famFV("20.30"))
	if len(r.Included) != 2 || r.Included[0] != "a" || r.Included[1] != "b" || len(r.Rows) != 2 || len(r.Excluded) != 0 {
		t.Errorf("result = %+v", r)
	}
	if r.Currency != "USD" || r.FxRate != "" {
		t.Errorf("currency/fx = %q %q", r.Currency, r.FxRate)
	}
}

func TestFamilyExcludesByReason(t *testing.T) {
	t.Parallel()
	mk := func(alias, reason string) blockapi.FamilyAccountIn {
		return blockapi.FamilyAccountIn{Alias: alias, Reason: reason, Values: famNL("USD", "999")}
	}
	r := NewFamily().Family(blockapi.FamilyIn{Accounts: []blockapi.FamilyAccountIn{
		famAcctIn("a", "d", famNL("USD", "100"), nil, nil), mk("b", "no_data"), mk("c", "stale_input"), mk("d", "read_error"),
	}})
	famEq(t, "Net", r.Net, famFV("100.00"))
	if len(r.Excluded) != 3 || len(r.Included) != 1 {
		t.Fatalf("result = %+v", r)
	}
	for i, want := range []string{"no_data", "stale_input", "read_error"} {
		if r.Excluded[i].Reason != want {
			t.Errorf("excluded %d reason = %q", i, r.Excluded[i].Reason)
		}
	}
}

func TestFamilyJpyNeedsRate(t *testing.T) {
	t.Parallel()
	in := blockapi.FamilyIn{
		Accounts: []blockapi.FamilyAccountIn{
			famAcctIn("u", "d", famNL("USD", "100"), nil, nil), famAcctIn("y", "d", famNL("JPY", "15000"), nil, nil),
		},
		UsdJpy: "150", RateDay: "2026-11-18", RateSource: "fx",
	}
	r := NewFamily().Family(in)
	famEq(t, "Net", r.Net, famFV("200.00"))
	if r.FxRate != "150" || r.FxDay != "2026-11-18" || r.FxSource != "fx" {
		t.Errorf("fx = %q %q %q", r.FxRate, r.FxDay, r.FxSource)
	}
	in.UsdJpy = ""
	in.Accounts = append(in.Accounts, famAcctIn("e", "d", famNL("EUR", "5"), nil, nil))
	r = NewFamily().Family(in)
	famEq(t, "Net", r.Net, famFV("100.00"))
	if len(r.Excluded) != 2 || r.Excluded[0].Reason != "unsupported_currency" || r.Excluded[1].Reason != "unsupported_currency" {
		t.Errorf("excluded = %+v", r.Excluded)
	}
	if r.FxRate != "" || r.FxDay != "" || r.FxSource != "" {
		t.Errorf("fx set without a JPY account: %+v", r)
	}
}

func TestFamilyGapNotPartialSum(t *testing.T) {
	t.Parallel()
	r := NewFamily().Family(blockapi.FamilyIn{Accounts: []blockapi.FamilyAccountIn{
		famAcctIn("a", "d", famVals("USD", "100", "10"), famNL("USD", "90"), famNL("USD", "80")),
		famAcctIn("b", "d", famNL("USD", "200"), nil, nil),
	}})
	famEq(t, "Net", r.Net, famFV("300.00"))
	for name, g := range map[string]compute.FigureValue{"Cash": r.Cash, "DayAmount": r.DayAmount, "DayPct": r.DayPct,
		"MonthAmount": r.MonthAmount, "MonthPct": r.MonthPct, "CashShare": r.CashShare} {
		famEq(t, name, g, compute.FigureValue{Gap: "missing_input"})
	}
}

func TestFamilyNoneCounted(t *testing.T) {
	t.Parallel()
	for _, in := range []blockapi.FamilyIn{
		{},
		{Accounts: []blockapi.FamilyAccountIn{{Alias: "a", Reason: "no_data"}}},
	} {
		r := NewFamily().Family(in)
		g := compute.FigureValue{Gap: "no_data"}
		for name, got := range map[string]compute.FigureValue{"Net": r.Net, "Cash": r.Cash, "CashShare": r.CashShare,
			"DayAmount": r.DayAmount, "DayPct": r.DayPct, "MonthAmount": r.MonthAmount, "MonthPct": r.MonthPct} {
			famEq(t, name, got, g)
		}
		if r.OldestDay != "" {
			t.Errorf("OldestDay = %q", r.OldestDay)
		}
	}
}

func TestFamilyCashShare(t *testing.T) {
	t.Parallel()
	one := func(net, cash string) blockapi.FamilyResult {
		return NewFamily().Family(blockapi.FamilyIn{Accounts: []blockapi.FamilyAccountIn{famAcctIn("a", "d", famVals("USD", net, cash), nil, nil)}})
	}
	famEq(t, "share", one("300", "100").CashShare, famFV("33.33"))
	famEq(t, "negative cash", one("300", "-5").CashShare, compute.FigureValue{})
	famEq(t, "zero net", one("0", "5").CashShare, compute.FigureValue{Gap: "no_data"})
}

func TestFamilyDayGapWhenDaysDiffer(t *testing.T) {
	t.Parallel()
	r := NewFamily().Family(blockapi.FamilyIn{Accounts: []blockapi.FamilyAccountIn{
		famAcctIn("a", "2026-11-18", famNL("USD", "100"), famNL("USD", "90"), famNL("USD", "80")),
		famAcctIn("b", "2026-11-17", famNL("USD", "200"), famNL("USD", "190"), famNL("USD", "180")),
	}})
	g := compute.FigureValue{Gap: "stale_input"}
	famEq(t, "DayAmount", r.DayAmount, g)
	famEq(t, "DayPct", r.DayPct, g)
	famEq(t, "MonthAmount", r.MonthAmount, famFV("40.00"))
	if r.OldestDay != "2026-11-17" {
		t.Errorf("OldestDay = %q", r.OldestDay)
	}
}

func TestFamilyOnceForRepeatedAlias(t *testing.T) {
	t.Parallel()
	r := NewFamily().Family(blockapi.FamilyIn{Accounts: []blockapi.FamilyAccountIn{
		famAcctIn("a", "d", famNL("USD", "100"), nil, nil), famAcctIn("a", "d", famNL("USD", "500"), nil, nil),
		famAcctIn("b", "d", famNL("USD", "1"), nil, nil),
	}})
	famEq(t, "Net", r.Net, famFV("101.00"))
	if len(r.Included) != 2 || len(r.Rows) != 2 {
		t.Errorf("result = %+v", r)
	}
}

func TestFamilyMissingNetIsNoData(t *testing.T) {
	t.Parallel()
	r := NewFamily().Family(blockapi.FamilyIn{Accounts: []blockapi.FamilyAccountIn{
		famAcctIn("a", "d", famNL("USD", "100"), nil, nil),
		famAcctIn("b", "d", nil, nil, nil),
	}})
	if len(r.Excluded) != 1 || r.Excluded[0].Reason != "no_data" {
		t.Errorf("excluded = %+v, want b with no_data", r.Excluded)
	}
}
