package figures_test

import (
	"math"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/contracttest"
	"github.com/andrewrong/optdesk/internal/compute/figures"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const day = "2024-03-04"

var ivRule = compute.Rule{ID: "iv30", Version: 1, MaxAgeDays: 4}

// quote is a strike with both legs at the given IVs and mids.
func quote(k, civ, piv, cmid, pmid float64) compute.StrikeQuote {
	return compute.StrikeQuote{Strike: k, CallIV: civ, PutIV: piv, CallMid: cmid, PutMid: pmid, HasCall: true, HasPut: true}
}

func expiry(minutes int, ks ...compute.StrikeQuote) compute.ExpiryQuotes {
	return compute.ExpiryQuotes{MinutesToExpiry: minutes, Strikes: ks}
}

// atm is an expiry whose only strike has C = P, so its ATM IV is the mean of civ and piv.
func atm(minutes int, civ, piv float64) compute.ExpiryQuotes {
	return expiry(minutes, quote(100, civ, piv, 5, 5))
}

func TestContractAllMethods(t *testing.T) {
	t.Parallel()
	stats := gonumstats.New()
	contracttest.Figures(contracttest.Wrap(t), figures.New, stats, own.NewRolling(stats), own.NewSeriesOps())
}

func TestIv30(t *testing.T) {
	t.Parallel()
	f := build(ivRule)
	rate := compute.Point{Day: day, Value: 0.05}
	bad := quote(100, 0, 0.19, 5, 5)
	noCall := quote(100, 0.2, 0.2, 5, 5)
	noCall.HasCall = false
	tests := []struct {
		name   string
		chain  []compute.ExpiryQuotes
		status compute.Status
		reason compute.Reason
		value  float64
		obs    int
	}{
		{"zero and one day ignored", []compute.ExpiryQuotes{atm(0, 0.9, 0.9), atm(1440, 0.9, 0.9), atm(43200, 0.21, 0.19)}, compute.StatusOK, "", 0.2, 1},
		{"short side only", []compute.ExpiryQuotes{atm(28800, 0.2, 0.2), atm(14400, 0.2, 0.2)}, compute.StatusMissing, compute.NoInput, 0, 0},
		{"long side only", []compute.ExpiryQuotes{atm(57600, 0.2, 0.2), atm(72000, 0.2, 0.2)}, compute.StatusMissing, compute.NoInput, 0, 0},
		{"only ignored expiries", []compute.ExpiryQuotes{atm(1440, 0.2, 0.2)}, compute.StatusMissing, compute.NoInput, 0, 0},
		{"missing call IV", []compute.ExpiryQuotes{expiry(43200, quote(100, 0, 0.19, 5, 5))}, compute.StatusMissing, compute.NoInput, 0, 0},
		{"missing put IV", []compute.ExpiryQuotes{expiry(43200, quote(100, 0.21, 0, 5, 5))}, compute.StatusMissing, compute.NoInput, 0, 0},
		{"missing call leg", []compute.ExpiryQuotes{expiry(43200, noCall)}, compute.StatusMissing, compute.NoInput, 0, 0},
		{"unusable expiry is not recomputed", []compute.ExpiryQuotes{expiry(43200, bad, quote(110, 0.3, 0.3, 9, 1))}, compute.StatusMissing, compute.NoInput, 0, 0},
		{"tie in |C-P| takes lower strike", []compute.ExpiryQuotes{expiry(43200,
			quote(110, 0.30, 0.30, 3, 3), quote(100, 0.10, 0.10, 3, 3))}, compute.StatusOK, "", 0.1, 1},
		{"strike nearest the forward", []compute.ExpiryQuotes{expiry(43200,
			quote(90, 0.40, 0.40, 12, 2), quote(100, 0.20, 0.20, 5, 5), quote(110, 0.30, 0.30, 1, 11))}, compute.StatusOK, "", 0.2, 1},
		{"bracketing expiries by the Cboe weights", []compute.ExpiryQuotes{atm(28800, 0.2, 0.2), atm(57600, 0.3, 0.3)}, compute.StatusOK,
			"", math.Sqrt((28800*0.04*14400 + 57600*0.09*14400) / 28800 / 43200), 2},
		{"empty chain", nil, compute.StatusMissing, compute.NoInput, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := f.Iv30("iv30", tc.chain, rate, day)
			if got.Status != tc.status || got.Reason != tc.reason || math.Abs(got.Value-tc.value) > 1e-12 || got.Observations != tc.obs {
				t.Errorf("got %+v, want %s %q %v obs %d", got, tc.status, tc.reason, tc.value, tc.obs)
			}
			if got.Status == compute.StatusOK && got.DataDay != day {
				t.Errorf("data day %q", got.DataDay)
			}
		})
	}
}

func TestIv30UsesForward(t *testing.T) {
	t.Parallel()
	// K* = 100 with C-P = 10, so F = 100 + exp(0.05 * T) * 10 with T = 30/365: F is near 110.
	// The strike nearest F is 110, not 100.
	f := build(ivRule)
	chain := []compute.ExpiryQuotes{expiry(43200,
		quote(100, 0.10, 0.10, 15, 5), quote(110, 0.30, 0.30, 20, 3))}
	got := f.Iv30("iv30", chain, compute.Point{Day: day, Value: 0.05}, day)
	if got.Status != compute.StatusOK || math.Abs(got.Value-0.30) > 1e-12 {
		t.Errorf("got %+v", got)
	}
}

func TestIv30StaleRate(t *testing.T) {
	t.Parallel()
	f := build(ivRule)
	chain := []compute.ExpiryQuotes{atm(43200, 0.2, 0.2)}
	if got := f.Iv30("iv30", chain, compute.Point{Day: "2024-03-03", Value: 0.05}, day); !got.StaleRate {
		t.Errorf("rate of another day: %+v", got)
	}
	if got := f.Iv30("iv30", chain, compute.Point{Day: day, Value: 0.05}, day); got.StaleRate {
		t.Errorf("rate of the day: %+v", got)
	}
}

func ivpRule(cmp compute.Compare) compute.Rule {
	return compute.Rule{ID: "ivp", Version: 1, Compare: cmp, Window: 252, MinValid: 252, MaxAgeDays: 4}
}

// ramp is 1..n on consecutive days followed by today.
func ramp(n int, today float64) (compute.Series, contracts.Day) {
	v := make([]float64, n+1)
	for i := 0; i < n; i++ {
		v[i] = float64(i + 1)
	}
	v[n] = today
	s := run("2023-01-01", v...)
	return s, s[n].Day
}

func TestIvPercentile(t *testing.T) {
	t.Parallel()
	f := build(ivpRule(compute.Below))
	s, asOf := ramp(252, 100)
	got := f.IvPercentile("ivp", s, asOf)
	// the rule says Below but ivp_v1 counts at or below: 100 of 252.
	if got.Status != compute.StatusOK || math.Abs(got.Value-100.0/252) > 1e-12 || got.Observations != 252 || got.DataDay != s[len(s)-1].Day {
		t.Errorf("252 sessions: %+v", got)
	}
	s, asOf = ramp(251, 100)
	got = f.IvPercentile("ivp", s, asOf)
	if got.Reason != compute.ShortHistory || got.Observations != 251 || got.Value != 0 {
		t.Errorf("251 sessions: %+v", got)
	}
	// today excluded: a very high today does not change the window.
	s, asOf = ramp(252, 1000)
	if got = f.IvPercentile("ivp", s, asOf); got.Value != 1 {
		t.Errorf("high today: %+v", got)
	}
	// older sessions than the window do not count.
	long := run("2022-01-01", append([]float64{5000, 5000}, valuesOf(s)...)...)
	if got = f.IvPercentile("ivp", long, long[len(long)-1].Day); got.Observations != 252 || got.Value != 1 {
		t.Errorf("long history: %+v", got)
	}
	if got = f.IvPercentile("ivp", nil, asOf); got.Reason != compute.NoInput {
		t.Errorf("empty: %+v", got)
	}
	if got = f.IvPercentile("ivp", s, "2024-01-01"); got.Reason != compute.StaleInput {
		t.Errorf("stale: %+v", got)
	}
}

func valuesOf(s compute.Series) []float64 {
	out := make([]float64, len(s))
	for i, p := range s {
		out[i] = p.Value
	}
	return out
}

func TestIvRank(t *testing.T) {
	t.Parallel()
	rule := compute.Rule{ID: "ivr", Version: 1, Window: 252, MinValid: 252, MaxAgeDays: 4}
	f := build(rule)
	s, asOf := ramp(252, 126.5)
	if got := f.IvRank("ivr", s, asOf); got.Status != compute.StatusOK || math.Abs(got.Value-0.5) > 1e-12 || got.Observations != 252 {
		t.Errorf("midpoint: %+v", got)
	}
	s, asOf = ramp(251, 100)
	if got := f.IvRank("ivr", s, asOf); got.Reason != compute.ShortHistory || got.Observations != 251 {
		t.Errorf("251: %+v", got)
	}
	flat := make([]float64, 253)
	for i := range flat {
		flat[i] = 5
	}
	fs := run("2023-01-01", flat...)
	if got := f.IvRank("ivr", fs, fs[252].Day); got.Reason != compute.ZeroRange || got.Value != 0 || got.Observations != 252 {
		t.Errorf("zero range: %+v", got)
	}
	// today excluded: the window is 1..252 whatever today is.
	s, asOf = ramp(252, 252)
	if got := f.IvRank("ivr", s, asOf); got.Value != 1 {
		t.Errorf("today at max: %+v", got)
	}
	if got := f.IvRank("ivr", nil, asOf); got.Reason != compute.NoInput {
		t.Errorf("empty: %+v", got)
	}
}

func TestAllFiguresFinished(t *testing.T) {
	t.Parallel()
	f := build(ivRule, ivpRule(compute.AtOrBelow))
	s, asOf := ramp(252, 100)
	got := []compute.Figure{
		f.Iv30("iv30", []compute.ExpiryQuotes{atm(43200, 0.2, 0.2)}, compute.Point{Day: day, Value: 0.05}, day),
		f.IvPercentile("ivp", s, asOf),
		f.IvRank("ivp", s, asOf),
	}
	for i, g := range got {
		if g.Reason == compute.BadInput {
			t.Errorf("method %d still returns the stub: %+v", i, g)
		}
	}
}
