package shortput_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/shortput"
)

func managed(t *testing.T, mode string) strategy.Strategy {
	t.Helper()
	s, err := shortput.New(optioncand.New(), candParams(), shortput.Values{CloseMode: mode, EarlyKeep: "0.50", EarlyDTE: "21"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func position(expiry contracts.Day) strategy.Position {
	return strategy.Position{
		Contract: "101", Right: "P", Strike: "100", Expiry: expiry, OpenedOn: "2026-10-01",
		OpenPrice: "2.00", Quantity: "-1", Multiplier: "100",
	}
}

func snapWithAsk(ask string) strategy.Snapshot {
	return strategy.Snapshot{RunDay: runDay, Quotes: []compute.OptionQuote{{InstrumentID: "101", Ask: ask}}}
}

var hold = strategy.Action{Kind: "hold"}

func TestManageHoldIsDefault(t *testing.T) {
	t.Parallel()
	s := managed(t, "expiry")
	// ask 0.10 keeps 95% of the credit and the position expires today: expiry mode still holds.
	for _, snap := range []strategy.Snapshot{snapWithAsk("0.10"), snapWithAsk(""), {RunDay: runDay}} {
		if a := s.Manage(position(runDay), snap); a != hold {
			t.Errorf("action = %+v", a)
		}
	}
}

func TestManageEarlyKeep(t *testing.T) {
	t.Parallel()
	s := managed(t, "early")
	far := addDays(runDay, 40)
	cases := []struct {
		name string
		ask  string
		want strategy.Action
	}{
		{"exactly half kept", "1.00", strategy.Action{Kind: "close", Reason: "early_keep"}},
		{"more than half kept", "0.30", strategy.Action{Kind: "close", Reason: "early_keep"}},
		{"just under half kept", "1.01", hold},
		{"credit lost", "3.50", hold},
		{"no ask", "", hold},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if a := s.Manage(position(far), snapWithAsk(tc.ask)); a != tc.want {
				t.Errorf("action = %+v, want %+v", a, tc.want)
			}
		})
	}
	// a quote of another contract is not the position's
	other := strategy.Snapshot{RunDay: runDay, Quotes: []compute.OptionQuote{{InstrumentID: "999", Ask: "0.10"}}}
	if a := s.Manage(position(far), other); a != hold {
		t.Errorf("other contract: %+v", a)
	}
}

func TestManageEarlyDTE(t *testing.T) {
	t.Parallel()
	s := managed(t, "early")
	dte := strategy.Action{Kind: "close", Reason: "early_dte"}
	cases := []struct {
		name   string
		expiry contracts.Day
		snap   strategy.Snapshot
		want   strategy.Action
	}{
		{"21 days to expiry", addDays(runDay, 21), snapWithAsk("1.90"), dte},
		{"22 days to expiry", addDays(runDay, 22), snapWithAsk("1.90"), hold},
		{"missing quote at 21 days", addDays(runDay, 21), strategy.Snapshot{RunDay: runDay}, dte},
		{"missing quote at 22 days", addDays(runDay, 22), strategy.Snapshot{RunDay: runDay}, hold},
		{"keep rule first", addDays(runDay, 5), snapWithAsk("0.50"), strategy.Action{Kind: "close", Reason: "early_keep"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if a := s.Manage(position(tc.expiry), tc.snap); a != tc.want {
				t.Errorf("action = %+v, want %+v", a, tc.want)
			}
		})
	}
}
