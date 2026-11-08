package figures_test

import (
	"math"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/contracttest"
	"github.com/andrewrong/optdesk/internal/compute/figures"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type rules map[string]compute.Rule

func (r rules) Rule(id string) (compute.Rule, bool) { v, ok := r[id]; return v, ok }
func (r rules) IDs() []string                       { return nil }

func build(rs ...compute.Rule) compute.Figures {
	m := rules{}
	for _, r := range rs {
		m[r.ID] = r
	}
	stats := gonumstats.New()
	return figures.New(stats, own.NewRolling(stats), own.NewSeriesOps(), m)
}

// run builds consecutive calendar days from first, one per value.
func run(first string, values ...float64) compute.Series {
	t0, _ := time.Parse(time.DateOnly, first)
	out := make(compute.Series, len(values))
	for i, v := range values {
		out[i] = compute.Point{Day: contracts.Day(t0.AddDate(0, 0, i).Format(time.DateOnly)), Value: v}
	}
	return out
}

func pt(day string, v float64) compute.Point { return compute.Point{Day: contracts.Day(day), Value: v} }

func TestContract(t *testing.T) {
	t.Parallel()
	stats := gonumstats.New()
	contracttest.Figures(contracttest.Wrap(t), figures.New, stats, own.NewRolling(stats), own.NewSeriesOps(),
		"HistoricalVol", "MovingAverageGap", "PercentileInYears", "Ratio")
}

func TestHistoricalVol(t *testing.T) {
	t.Parallel()
	rule := compute.Rule{ID: "hv", Version: 2, N: 30, TradingDays: 100, MaxAgeDays: 4}
	f := build(rule)

	// constant log returns: sd 0.
	vals := make([]float64, 31)
	vals[0] = 100
	for i := 1; i < len(vals); i++ {
		vals[i] = vals[i-1] * 1.01
	}
	closes := run("2024-01-01", vals...)
	last := closes[len(closes)-1].Day
	got := f.HistoricalVol("hv", closes, last)
	if got.Status != compute.StatusOK || math.Abs(got.Value) > 1e-9 || got.Observations != 30 || got.Version != 2 {
		t.Errorf("constant returns: %+v", got)
	}

	// alternating returns +-a: sd = a*sqrt(n/(n-1)), times sqrt(TradingDays)=10.
	a := math.Log(1.1)
	alt := make([]float64, 31)
	alt[0] = 100
	for i := 1; i < len(alt); i++ {
		alt[i] = alt[i-1] * math.Exp(a*math.Pow(-1, float64(i+1)))
	}
	got = f.HistoricalVol("hv", run("2024-01-01", alt...), last)
	want := a * math.Sqrt(30.0/29.0) * 10
	if got.Status != compute.StatusOK || math.Abs(got.Value-want) > 1e-9 {
		t.Errorf("alternating: %+v, want %v", got, want)
	}

	if got = f.HistoricalVol("hv", run("2024-01-01", vals[:30]...), "2024-01-30"); got.Reason != compute.ShortHistory {
		t.Errorf("30 closes: %+v", got)
	}
	bad := append([]float64(nil), vals...)
	bad[10] = 0
	if got = f.HistoricalVol("hv", run("2024-01-01", bad...), last); got.Reason != compute.BadInput {
		t.Errorf("zero close: %+v", got)
	}
	if got = f.HistoricalVol("hv", nil, last); got.Reason != compute.NoInput {
		t.Errorf("no closes: %+v", got)
	}
	if got = f.HistoricalVol("nope", closes, last); got.Reason != compute.BadInput || got.Status != compute.StatusMissing {
		t.Errorf("unknown id: %+v", got)
	}
}

func TestMovingAverageGap(t *testing.T) {
	t.Parallel()
	f := build(compute.Rule{ID: "ma", Version: 1, N: 200, MaxAgeDays: 4})
	ramp := make([]float64, 200)
	for i := range ramp {
		ramp[i] = float64(i + 1)
	}
	got := f.MovingAverageGap("ma", run("2023-01-01", ramp[:199]...), "2023-07-18")
	if got.Status != compute.StatusMissing || got.Reason != compute.ShortHistory {
		t.Errorf("199 closes: %+v", got)
	}
	got = f.MovingAverageGap("ma", run("2023-01-01", ramp...), "2023-07-19")
	if got.Status != compute.StatusOK || math.Abs(got.Value-(200.0/100.5-1)) > 1e-12 || got.Observations != 200 {
		t.Errorf("200 closes: %+v", got)
	}
	if got.DataDay != "2023-07-19" {
		t.Errorf("data day %q", got.DataDay)
	}
}

func TestPercentileInYears(t *testing.T) {
	t.Parallel()
	mk := func(cmp compute.Compare) compute.Figures {
		return build(compute.Rule{ID: "p", Version: 1, Years: 3, Compare: cmp, MaxAgeDays: 8})
	}
	s := compute.Series{pt("2021-01-08", 10), pt("2021-01-11", 20), pt("2022-06-01", 30), pt("2023-03-01", 40),
		pt("2023-09-01", 50), pt("2024-01-10", 40)}
	below := mk(compute.Below).PercentileInYears("p", s, "2024-01-10")
	if below.Status != compute.StatusOK || below.Value != 0.5 || below.Observations != 4 {
		t.Errorf("below: %+v", below)
	}
	atOr := mk(compute.AtOrBelow).PercentileInYears("p", s, "2024-01-10")
	if atOr.Value != 0.75 {
		t.Errorf("at or below: %+v", atOr)
	}
	// the day itself (40) is excluded from the window: 4 observations.
	short := compute.Series{pt("2022-01-03", 20), pt("2023-03-01", 40), pt("2024-01-10", 40)}
	got := mk(compute.Below).PercentileInYears("p", short, "2024-01-10")
	if got.Reason != compute.ShortHistory || got.Status != compute.StatusMissing {
		t.Errorf("two years: %+v", got)
	}
}

func TestRatio(t *testing.T) {
	t.Parallel()
	f := build(compute.Rule{ID: "r", Version: 1, MaxAgeDays: 4})
	num := compute.Series{pt("2024-03-04", 18)}
	if got := f.Ratio("r", num, compute.Series{pt("2024-03-04", 0)}, "2024-03-04"); got.Reason != compute.BadInput {
		t.Errorf("zero den: %+v", got)
	}
	got := f.Ratio("r", num, compute.Series{pt("2024-02-20", 20)}, "2024-03-04")
	if got.Reason != compute.StaleInput || got.Status != compute.StatusMissing {
		t.Errorf("stale den: %+v", got)
	}
	got = f.Ratio("r", num, compute.Series{pt("2024-03-04", 20)}, "2024-03-04")
	if got.Status != compute.StatusOK || got.Value != 0.9 {
		t.Errorf("ok: %+v", got)
	}
	got = f.Ratio("r", num, compute.Series{pt("2024-03-03", 20)}, "2024-03-04")
	if got.Reason != compute.BadInput {
		t.Errorf("different days: %+v", got)
	}
}

func TestStaleInput(t *testing.T) {
	t.Parallel()
	f := build(compute.Rule{ID: "ma", Version: 1, N: 2, MaxAgeDays: 4, Years: 1})
	s := run("2024-01-01", 1, 2, 3)
	got := f.MovingAverageGap("ma", s, "2024-01-08")
	if got.Status != compute.StatusMissing || got.Reason != compute.StaleInput || got.DataDay != "2024-01-03" {
		t.Errorf("stale: %+v", got)
	}
	if got = f.MovingAverageGap("ma", s, "2024-01-08"); got.Day != "2024-01-08" {
		t.Errorf("day: %+v", got)
	}
	// exactly MaxAgeDays old is still fresh: only older than MaxAgeDays is stale.
	if got = f.MovingAverageGap("ma", s, "2024-01-06"); got.Status != compute.StatusOK {
		t.Errorf("4 days old: %+v", got)
	}
}

func TestCompareFromRule(t *testing.T) {
	t.Parallel()
	s := compute.Series{pt("2020-01-01", 1), pt("2023-03-01", 40), pt("2023-04-01", 50), pt("2024-01-10", 40)}
	rank := func(c compute.Compare) float64 {
		got := build(compute.Rule{ID: "p", Years: 3, Compare: c, MaxAgeDays: 1}).PercentileInYears("p", s, "2024-01-10")
		if got.Status != compute.StatusOK {
			t.Fatalf("%s: %+v", c, got)
		}
		return got.Value
	}
	if b, a := rank(compute.Below), rank(compute.AtOrBelow); b != 0 || a != 0.5 {
		t.Errorf("below %v at or below %v, want 0 and 0.5", b, a)
	}
}
