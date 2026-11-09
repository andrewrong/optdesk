package optiontrack

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func call(p *compute.LedgerPosition) { p.Right = "C" }

func callQuote(bid, ask, spot string) trackapi.TrackQuote {
	q := quote(bid, ask, spot)
	q.Right = "C"
	return q
}

func TestDTEDays(t *testing.T) {
	t.Parallel()
	r := one(t, pos("a", nil), quote("1", "1.2", "110"))
	if r.DTE != 39 { // 2026-11-09 to 2026-12-18
		t.Fatalf("DTE %d", r.DTE)
	}
	r = one(t, pos("a", func(p *compute.LedgerPosition) { p.Expiry = "2026-11-09" }))
	if r.DTE != 0 {
		t.Fatalf("DTE %d", r.DTE)
	}
}

func TestMoneynessPut(t *testing.T) {
	t.Parallel()
	for spot, want := range map[string]string{"95": "itm", "105": "otm"} {
		if r := one(t, pos("a", nil), quote("1", "1.2", spot)); r.Moneyness != want {
			t.Errorf("put spot %s: %s", spot, r.Moneyness)
		}
	}
}

func TestMoneynessCall(t *testing.T) {
	t.Parallel()
	for spot, want := range map[string]string{"95": "otm", "105": "itm"} {
		if r := one(t, pos("a", call), callQuote("1", "1.2", spot)); r.Moneyness != want {
			t.Errorf("call spot %s: %s", spot, r.Moneyness)
		}
	}
}

func TestAtmOnlyWhenEqual(t *testing.T) {
	t.Parallel()
	if r := one(t, pos("a", nil), quote("1", "1.2", "100.00")); r.Moneyness != "atm" || r.OtmPercent != "0.00" {
		t.Fatalf("%+v", r)
	}
	if r := one(t, pos("a", nil), quote("1", "1.2", "100.01")); r.Moneyness != "otm" {
		t.Fatalf("%+v", r)
	}
	if r := one(t, pos("a", nil)); r.Moneyness != "unknown" {
		t.Fatalf("no spot: %+v", r)
	}
}

func TestOtmPercentRounding(t *testing.T) {
	t.Parallel()
	// Decision 5: put (spot - strike) / strike, call (strike - spot) / strike, times 100;
	// positive out of the money, so a put at spot 105 over strike 100 is out of the money.
	cases := []struct {
		c      func(*compute.LedgerPosition)
		strike string
		spot   string
		want   string
	}{
		{nil, "100", "105", "5.00"},
		{nil, "100", "95", "-5.00"},
		{call, "100", "95", "5.00"},
		{call, "100", "105", "-5.00"},
		{nil, "1000", "999.95", "0.00"},  // -0.005 is a half: to even, zero
		{nil, "1000", "1000.05", "0.00"}, // 0.005
		{nil, "1000", "1000.15", "0.02"}, // 0.015 is a half: to even, 0.02
		{nil, "1000", "1000.25", "0.02"}, // 0.025 is a half: to even, 0.02
		{nil, "1000", "999.85", "-0.02"}, // -0.015
		{nil, "1000", "999.75", "-0.02"}, // -0.025
	}
	for _, c := range cases {
		p := pos("a", func(p *compute.LedgerPosition) {
			p.Strike = c.strike
			if c.c != nil {
				c.c(p)
			}
		})
		q := quote("1", "1.2", c.spot)
		if c.c != nil {
			q.Right = "C"
		}
		q.Strike = c.strike
		if r := one(t, p, q); r.OtmPercent != c.want {
			t.Errorf("strike %s spot %s: %q want %q", c.strike, c.spot, r.OtmPercent, c.want)
		}
	}
}

func TestMidRule(t *testing.T) {
	t.Parallel()
	if r := one(t, pos("a", nil), quote("1.20", "1.30", "110")); r.Mid != "1.2500" || r.MarkStatus != "ok" || r.MarkDay != asOf {
		t.Fatalf("%+v", r)
	}
	// bid zero is allowed, a negative bid or a zero ask is no mark.
	if r := one(t, pos("a", nil), quote("0", "0.10", "110")); r.Mid != "0.0500" {
		t.Fatalf("%+v", r)
	}
	for _, q := range []trackapi.TrackQuote{quote("-0.1", "0.5", "110"), quote("0", "0", "110"), quote("", "1", "110")} {
		if r := one(t, pos("a", nil), q); r.MarkStatus != "no_mark" || r.Mid != "" {
			t.Fatalf("%+v", r)
		}
	}
	// the newest quote wins
	old := quote("5", "6", "110")
	old.Day = "2026-11-06"
	if r := one(t, pos("a", nil), old, quote("1", "1.2", "110")); r.Mid != "1.1000" {
		t.Fatalf("%+v", r)
	}
}

func TestStaleMark(t *testing.T) {
	t.Parallel()
	in := input([]compute.LedgerPosition{pos("a", nil)}, quote("1", "1.2", "110"))
	in.Quotes[0].Day = "2026-11-05"
	in.SessionDays = []contracts.Day{"2026-11-05", "2026-11-06", "2026-11-07", "2026-11-09"}
	// three session days after the mark: still used with limit 3
	if r := New().Track(in, defParams).Open[0]; r.MarkStatus != "ok" {
		t.Fatalf("age 3: %+v", r)
	}
	in.SessionDays = append(in.SessionDays[:3:3], "2026-11-08", "2026-11-09")
	r := New().Track(in, defParams).Open[0]
	if r.MarkStatus != "no_mark" || r.Mid != "" || r.Kept != "" || r.Spot != "" {
		t.Fatalf("age 4: %+v", r)
	}
	// a quote after the as-of day is not used
	in2 := input([]compute.LedgerPosition{pos("a", nil)}, quote("1", "1.2", "110"))
	in2.Quotes[0].Day = "2026-11-10"
	if r := New().Track(in2, defParams).Open[0]; r.MarkStatus != "no_mark" {
		t.Fatalf("future quote: %+v", r)
	}
}

func TestNoMarkIsEmptyNotZero(t *testing.T) {
	t.Parallel()
	r := one(t, pos("a", nil))
	if r.MarkStatus != "no_mark" || r.Mid != "" || r.Kept != "" || r.KeptPercent != "" || r.Extrinsic != "" ||
		r.OtmPercent != "" || r.Delta != "" || r.Iv != "" || r.MarkDay != "" {
		t.Fatalf("%+v", r)
	}
}

func TestKeptAndPercent(t *testing.T) {
	t.Parallel()
	// premium 150, short 1 x 100 at mid 0.45 -> kept 150 - 45 = 105, 70.00 percent
	r := one(t, pos("a", nil), quote("0.40", "0.50", "110"))
	if r.Kept != "105.0000" || r.KeptPercent != "70.00" || r.Commissions != "1.3000" {
		t.Fatalf("%+v", r)
	}
	// bought back above the premium: negative kept
	r = one(t, pos("a", nil), quote("2.00", "2.10", "90"))
	if r.Kept != "-55.0000" || r.KeptPercent != "-36.67" {
		t.Fatalf("%+v", r)
	}
	// a premium of zero gives no percent
	r = one(t, pos("a", func(p *compute.LedgerPosition) { p.PremiumIncome = "0.0000" }), quote("0.40", "0.50", "110"))
	if r.KeptPercent != "" {
		t.Fatalf("%+v", r)
	}
}

func TestExtrinsicBothRights(t *testing.T) {
	t.Parallel()
	cases := []struct {
		isCall bool
		spot   string
		want   string
	}{
		{false, "105", "2.5000"}, // otm put: mid 2.5, intrinsic 0
		{false, "98", "0.5000"},  // itm put: 2.5 - 2
		{false, "90", "0.0000"},  // deep itm: floor at 0
		{true, "95", "2.5000"},   // otm call
		{true, "102", "0.5000"},  // itm call: 2.5 - 2
		{true, "120", "0.0000"},
	}
	for _, c := range cases {
		var r trackapi.TrackedOption
		if c.isCall {
			r = one(t, pos("a", call), callQuote("2.4", "2.6", c.spot))
		} else {
			r = one(t, pos("a", nil), quote("2.4", "2.6", c.spot))
		}
		if r.Extrinsic != c.want {
			t.Errorf("call=%v spot %s: %q want %q", c.isCall, c.spot, r.Extrinsic, c.want)
		}
	}
}

func TestDeltaIvCopied(t *testing.T) {
	t.Parallel()
	q := quote("1", "1.2", "110")
	q.Delta, q.Iv = "-0.2000", "0.3100"
	if r := one(t, pos("a", nil), q); r.Delta != "-0.2000" || r.Iv != "0.3100" {
		t.Fatalf("%+v", r)
	}
	q.Delta, q.Iv = "", ""
	if r := one(t, pos("a", nil), q); r.Delta != "" || r.Iv != "" {
		t.Fatalf("a missing value must stay empty: %+v", r)
	}
}
