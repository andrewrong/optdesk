package shortput_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/shortput"
)

const runDay contracts.Day = "2026-11-09" // a Monday

func addDays(d contracts.Day, n int) contracts.Day {
	t, _ := time.Parse("2006-01-02", string(d))
	return contracts.Day(t.AddDate(0, 0, n).Format("2006-01-02"))
}

func weekdays() []contracts.Day {
	var out []contracts.Day
	for d := contracts.Day("2026-09-01"); d <= runDay; d = addDays(d, 1) {
		t, _ := time.Parse("2006-01-02", string(d))
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d)
		}
	}
	return out
}

func candParams() compute.CandidateParams {
	return compute.CandidateParams{
		MinDTE: 21, MaxDTE: 90, MinBidSize: 1, MaxSpread: "0.10", MinOpenInterest: 10,
		DivGuardFactor: "1.0", MaxRateAgeDays: 7, MaxEventAgeSessions: 2, MinReportGapDays: 60,
	}
}

func values() shortput.Values {
	return shortput.Values{CloseMode: "expiry", EarlyKeep: "0.50", EarlyDTE: "21"}
}

func newPut(t *testing.T) strategy.Strategy {
	t.Helper()
	s, err := shortput.New(optioncand.New(), candParams(), values())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// baseSnap holds one put that passes every gate: strike 100, bid 2.00, ask 2.05, buy price 98, expiry in 30 days.
func baseSnap() strategy.Snapshot {
	return strategy.Snapshot{
		RunDay: runDay, ChainDay: runDay, SessionDays: weekdays(), Issuer: "ISS", Underlying: "XYZ", UnderlyingID: 7,
		Spot: "105",
		Quotes: []compute.OptionQuote{{
			InstrumentID: "101", Expiry: addDays(runDay, 30), Right: "P", Strike: "100", Multiplier: "100",
			Bid: "2.00", Ask: "2.05", BidSize: 5, OpenInterest: 50, Delta: "-0.25", IV: "0.31",
			MarketDataType: 1, QuoteOK: true,
		}},
		BuyPrice: "98",
		Earnings: &compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: runDay},
		Rate:     "4.20", RateDay: runDay,
		Accounts: []compute.AliasFunds{{Alias: "A", Funds: "20000", Currency: "USD", CaptureDay: runDay}},
		Context:  map[string]string{"earnings_state": "listed", "next_earnings": "2027-02-01"},
	}
}

type wantRow struct {
	status string
	rankIn int
	checks []strategy.Check
	facts  map[string]string
}

func pass(name string) strategy.Check { return strategy.Check{Name: name, Status: "pass"} }

func TestShortPutEvaluateMapsEveryColumn(t *testing.T) {
	t.Parallel()
	expiry := addDays(runDay, 30) // 2026-12-09
	leg := strategy.Leg{
		InstrumentID: "101", Side: "short", Right: "P", Strike: "100", Expiry: expiry, Multiplier: "100", Quantity: 1,
	}
	okFacts := func() map[string]string {
		return map[string]string{
			"issuer": "ISS", "underlying_id": "7", "dte": "30", "bid": "2.00", "ask": "2.05", "spot": "105",
			"buy_price": "98", "break_even": "98", "annualized_yield": "0.243333", "delta": "-0.25", "iv": "0.31",
			"marks": `["earnings_unconfirmed"]`, "fits": `{"A":"fit"}`,
			"earnings_state": "listed", "next_earnings": "2027-02-01",
		}
	}
	with := func(m map[string]string, kv ...string) map[string]string {
		for i := 0; i < len(kv); i += 2 {
			if kv[i+1] == "" {
				delete(m, kv[i])
			} else {
				m[kv[i]] = kv[i+1]
			}
		}
		return m
	}
	allPass := []strategy.Check{pass("buy_price"), pass("quote"), pass("events"), pass("funding"), pass("cash_rate")}
	replace := func(i int, c strategy.Check) []strategy.Check {
		out := append([]strategy.Check(nil), allPass...)
		out[i] = c
		return out
	}
	cases := []struct {
		name string
		mod  func(*strategy.Snapshot)
		want wantRow
	}{
		{"ok", func(*strategy.Snapshot) {}, wantRow{"ok", 1, allPass, okFacts()}},
		{"buy_price blocked", func(s *strategy.Snapshot) { s.Quotes[0].Bid = "1.99"; s.Quotes[0].Ask = "2.04" },
			wantRow{"blocked", 0, replace(0, strategy.Check{Name: "buy_price", Status: "block", Reason: "above_buy_price"}),
				with(okFacts(), "bid", "1.99", "ask", "2.04", "break_even", "98.01", "annualized_yield", "0.242117")}},
		{"quote blocked", func(s *strategy.Snapshot) { s.Quotes[0].BidSize = 0 },
			wantRow{"blocked", 0, replace(1, strategy.Check{Name: "quote", Status: "block", Reason: "small_bid_size"}), okFacts()}},
		{"events blocked", func(s *strategy.Snapshot) { s.Earnings.Next = addDays(runDay, 10) },
			wantRow{"blocked", 0, replace(2, strategy.Check{Name: "events", Status: "block", Reason: "earnings_before_expiry"}),
				with(okFacts(), "marks", `[]`)}},
		{"funding blocked", func(s *strategy.Snapshot) { s.Accounts[0].Funds = "5000" },
			wantRow{"blocked", 0, replace(3, strategy.Check{Name: "funding", Status: "block", Reason: "no_fit"}),
				with(okFacts(), "fits", `{"A":"no_fit"}`)}},
		{"cash_rate blocked", func(s *strategy.Snapshot) { s.Rate = "30.00" },
			wantRow{"blocked", 0, replace(4, strategy.Check{Name: "cash_rate", Status: "block", Reason: "yield_below_cash"}), okFacts()}},
		{"unknown", func(s *strategy.Snapshot) { s.Quotes[0].MarketDataType = 3 },
			wantRow{"unknown", 0, replace(1, strategy.Check{Name: "quote", Status: "unknown", Reason: "delayed_data"}), okFacts()}},
		{"no buy price", func(s *strategy.Snapshot) { s.BuyPrice = "" },
			wantRow{"ok", 1, allPass, with(okFacts(), "buy_price", "", "marks", `["earnings_unconfirmed","no_buy_price"]`)}},
		{"no account", func(s *strategy.Snapshot) { s.Accounts = nil },
			wantRow{"ok", 1, allPass, with(okFacts(), "fits", `{}`, "marks", `["earnings_unconfirmed","no_account"]`)}},
		{"in the money", func(s *strategy.Snapshot) { s.Spot = "95" },
			wantRow{"ok", 1, allPass, with(okFacts(), "spot", "95", "marks", `["itm","earnings_unconfirmed"]`)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			snap := baseSnap()
			tc.mod(&snap)
			got := newPut(t).Evaluate(snap)
			if len(got) != 1 {
				t.Fatalf("got %d evaluations", len(got))
			}
			e := got[0]
			if e.Key != "101" || e.Status != tc.want.status || e.Rank != 0 || e.RankInUnderlying != tc.want.rankIn {
				t.Errorf("key %q status %q rank %d/%d", e.Key, e.Status, e.Rank, e.RankInUnderlying)
			}
			if !reflect.DeepEqual(e.Legs, []strategy.Leg{leg}) {
				t.Errorf("legs = %+v", e.Legs)
			}
			if !reflect.DeepEqual(e.Checks, tc.want.checks) {
				t.Errorf("checks = %+v\nwant     %+v", e.Checks, tc.want.checks)
			}
			if !reflect.DeepEqual(e.Facts, tc.want.facts) {
				t.Errorf("facts = %v\nwant    %v", e.Facts, tc.want.facts)
			}
		})
	}
}

func TestShortPutIdentityAndParams(t *testing.T) {
	t.Parallel()
	s := newPut(t)
	if s.ID() != "short_put" || s.Version() != "put_v1" {
		t.Fatalf("identity %q %q", s.ID(), s.Version())
	}
	want := map[string]string{
		"min_dte": "21", "max_dte": "90", "min_bid_size": "1", "max_spread": "0.10", "min_open_interest": "10",
		"div_guard_factor": "1.0", "max_rate_age_days": "7", "max_event_age_sessions": "2",
		"min_report_gap_days": "60", "close_mode": "expiry", "early_keep": "0.50", "early_dte": "21",
	}
	p := s.Params()
	if !reflect.DeepEqual(p.Values, want) || len(p.Hash) != 40 {
		t.Fatalf("params = %+v", p)
	}
}

func TestNewRefusesBadValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		v    shortput.Values
		key  string
	}{
		{"mode", shortput.Values{CloseMode: "sometimes-secret", EarlyKeep: "0.5", EarlyDTE: "21"}, "close_mode"},
		{"keep", shortput.Values{CloseMode: "early", EarlyKeep: "half-secret", EarlyDTE: "21"}, "early_keep"},
		{"dte", shortput.Values{CloseMode: "early", EarlyKeep: "0.5", EarlyDTE: "21.5-secret"}, "early_dte"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := shortput.New(optioncand.New(), candParams(), tc.v)
			if err == nil {
				t.Fatal("no error")
			}
			msg := err.Error()
			if !contains(msg, tc.key) || contains(msg, "secret") {
				t.Fatalf("error %q", msg)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
