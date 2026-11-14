package liquidity_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidity"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type fixture struct {
	Day    contracts.Day `json:"day"`
	Series map[string][]struct {
		Day   contracts.Day `json:"day"`
		Value float64       `json:"value"`
	} `json:"series"`
	Rank struct {
		Count  int       `json:"count"`
		Values []float64 `json:"values"`
	} `json:"rank"`
}

func load(t *testing.T) fixture {
	t.Helper()
	b, err := os.ReadFile("testdata/liquidity.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fixture) series(name string) compute.Series {
	var s compute.Series
	for _, p := range f.Series[name] {
		s = append(s, compute.Point{Day: p.Day, Value: p.Value})
	}
	return s
}

func (f fixture) input() liquidityapi.Input {
	return liquidityapi.Input{
		Day: f.Day, Params: liquidity.TrialParams(),
		WRESBAL: f.series("WRESBAL"), TLAACBW027SBOG: f.series("TLAACBW027SBOG"),
		RRPONTSYD: f.series("RRPONTSYD"), SOFR: f.series("SOFR"), IORB: f.series("IORB"),
		WLCFLPCL: f.series("WLCFLPCL"),
	}
}

func plus(d contracts.Day, n int) contracts.Day {
	t, _ := time.Parse(time.DateOnly, string(d))
	return contracts.Day(t.AddDate(0, 0, n).Format(time.DateOnly))
}

func one(day contracts.Day, v float64) compute.Series { return compute.Series{{Day: day, Value: v}} }

func TestLiquidityTrialParams(t *testing.T) {
	t.Parallel()
	if p := liquidity.TrialParams(); p.DailyMaxAgeDays != 4 || p.WeeklyMaxAgeDays != 14 || p.PercentileMinDays != 250 {
		t.Fatalf("%+v", p)
	}
}

func TestLiquidityLevelsStale(t *testing.T) {
	t.Parallel()
	f := load(t)
	for _, c := range []struct {
		age   int
		stale bool
	}{{4, false}, {5, true}} {
		in := f.input()
		in.Day = plus("2026-11-12", c.age)
		got := liquidity.Compute(in).RRP
		if c.stale != (got.Reason == compute.StaleInput) || (c.stale && got.Value != "") || (!c.stale && (got.Value != "5000000000" || got.Day != "2026-11-12")) {
			t.Errorf("daily age %d: %+v", c.age, got)
		}
	}
	for _, c := range []struct {
		age   int
		stale bool
	}{{14, false}, {15, true}} {
		in := f.input()
		in.Day = plus("2026-11-11", c.age)
		got := liquidity.Compute(in).Reserves
		if c.stale != (got.Reason == compute.StaleInput) || (c.stale && got.Value != "") || (!c.stale && got.Value != "3050000000000") {
			t.Errorf("weekly age %d: %+v", c.age, got)
		}
	}
}

func TestLiquidityChangeExactDays(t *testing.T) {
	t.Parallel()
	f := load(t)
	r := liquidity.Compute(f.input())
	if c := r.Reserves4w; c.Value != "50000000000" || c.Weeks != 4 || c.Day != "2026-11-11" || c.From != "2026-10-14" {
		t.Errorf("4w %+v", c)
	}
	if c := r.Reserves13w; c.Value != "-150000000000" || c.Weeks != 13 || c.From != "2026-08-12" {
		t.Errorf("13w %+v", c)
	}
	in := f.input()
	in.WRESBAL = in.WRESBAL[1:] // no point on 2026-08-12
	r = liquidity.Compute(in)
	if c := r.Reserves13w; c.Value != "" || c.Reason != liquidityapi.MissingInput {
		t.Errorf("missing day %+v", c)
	}
	if r.Reserves4w.Value != "50000000000" {
		t.Errorf("4w after drop %+v", r.Reserves4w)
	}
}

func TestLiquiditySofrIorbSameDay(t *testing.T) {
	t.Parallel()
	f := load(t)
	if got := liquidity.Compute(f.input()).SofrIorbBP; got.Value != "16.0" || got.Day != "2026-11-12" {
		t.Errorf("positive %+v", got)
	}
	in := f.input()
	in.SOFR = one("2026-11-12", 4.3125) // 16.25 bp, half-even
	if got := liquidity.Compute(in).SofrIorbBP; got.Value != "16.2" {
		t.Errorf("half-even down %+v", got)
	}
	in.SOFR = one("2026-11-12", 4.3135) // 16.35 bp
	if got := liquidity.Compute(in).SofrIorbBP; got.Value != "16.4" {
		t.Errorf("half-even up %+v", got)
	}
	in.SOFR = one("2026-11-12", 4.10)
	if got := liquidity.Compute(in).SofrIorbBP; got.Value != "-5.0" {
		t.Errorf("negative %+v", got)
	}
	in.IORB = one("2026-11-10", 4.15) // none on the SOFR day
	if got := liquidity.Compute(in).SofrIorbBP; got.Value != "" || got.Reason != liquidityapi.MissingInput {
		t.Errorf("no IORB on the day %+v", got)
	}
}

func TestLiquiditySpreadStale(t *testing.T) {
	t.Parallel()
	f := load(t)
	for _, c := range []struct {
		age   int
		stale bool
	}{{4, false}, {5, true}} {
		in := f.input()
		in.Day = plus("2026-11-12", c.age)
		got := liquidity.Compute(in).SofrIorbBP
		if c.stale != (got.Reason == compute.StaleInput) || (c.stale && got.Value != "") || (!c.stale && got.Value != "16.0") {
			t.Errorf("age %d: %+v", c.age, got)
		}
	}
}

func hySeries(f fixture, count int) compute.Series {
	var s compute.Series
	for i := 0; i < count; i++ {
		s = append(s, compute.Point{Day: plus(f.Day, i-count+1), Value: f.Rank.Values[i%len(f.Rank.Values)]})
	}
	return s
}

func TestLiquidityRankWindow(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	in.BAMLH0A0HYM2 = hySeries(f, f.Rank.Count)
	// the newest of 250 days is 3.5; 126 of the 250 are at or below it (ties count).
	got := liquidity.Compute(in).HighYieldRank
	if got.Value != "50" || got.Days != 250 || got.First != plus(f.Day, -249) || got.Day != f.Day || got.Reason != "" {
		t.Errorf("rank %+v", got)
	}
	in.BAMLH0A0HYM2 = hySeries(f, 249)
	got = liquidity.Compute(in).HighYieldRank
	if got.Value != "" || got.Reason != compute.ShortHistory || got.Days != 249 || got.First != plus(f.Day, -248) {
		t.Errorf("249 days %+v", got)
	}
	in.BAMLH0A0HYM2 = append(hySeries(f, 250), compute.Point{Day: plus(f.Day, 1), Value: 99})
	if got := liquidity.Compute(in).HighYieldRank; got.Days != 250 || got.Day != f.Day {
		t.Errorf("a point after Day counted: %+v", got)
	}
	in.BAMLH0A0HYM2 = nil
	if got := liquidity.Compute(in).HighYieldRank; got.Value != "" || got.Reason != compute.NoInput {
		t.Errorf("no series %+v", got)
	}
}

func TestLiquidityReservesRatio(t *testing.T) {
	t.Parallel()
	f := load(t)
	// 3.05e12 over 2.44e13 is 12.5 percent.
	if got := liquidity.Compute(f.input()).ReservesOverAssets; got.Value != "12.50" || got.Day != "2026-11-11" {
		t.Errorf("ratio %+v", got)
	}
	in := f.input()
	in.TLAACBW027SBOG = one("2026-11-04", 2.4e13) // not the day of WRESBAL
	if got := liquidity.Compute(in).ReservesOverAssets; got.Value != "" || got.Reason != liquidityapi.MissingInput {
		t.Errorf("different day %+v", got)
	}
	in.TLAACBW027SBOG = nil
	if got := liquidity.Compute(in).ReservesOverAssets; got.Value != "" || got.Reason != compute.NoInput {
		t.Errorf("no assets %+v", got)
	}
}

func TestLiquidityRatioStale(t *testing.T) {
	t.Parallel()
	f := load(t)
	for _, c := range []struct {
		age   int
		stale bool
	}{{14, false}, {15, true}} {
		in := f.input()
		in.Day = plus("2026-11-11", c.age)
		got := liquidity.Compute(in).ReservesOverAssets
		if c.stale != (got.Reason == compute.StaleInput) || (c.stale && got.Value != "") || (!c.stale && got.Value != "12.50") {
			t.Errorf("age %d: %+v", c.age, got)
		}
	}
}

func TestLiquidityDiscountStale(t *testing.T) {
	t.Parallel()
	f := load(t)
	for _, c := range []struct {
		age   int
		stale bool
	}{{14, false}, {15, true}} {
		in := f.input()
		in.Day = plus("2026-11-11", c.age)
		got := liquidity.Compute(in).DiscountWindow
		if c.stale != (got.Reason == compute.StaleInput) || (c.stale && got.Value != "") || (!c.stale && got.Value != "3000000000") {
			t.Errorf("age %d: %+v", c.age, got)
		}
	}
}

func TestLiquidityNetIsFromNetFunc(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	in.Net = func(_, _, _ compute.Series, asOf contracts.Day) liquidityapi.Num {
		if asOf != f.Day {
			t.Errorf("asOf %s", asOf)
		}
		return liquidityapi.Num{Value: "5800000000000", Day: "2026-11-11", Aux: "2026-11-10"}
	}
	want := liquidityapi.Num{Value: "5800000000000", Day: "2026-11-11", Aux: "2026-11-10"}
	if got := liquidity.Compute(in).NetLiquidity; got != want {
		t.Errorf("net %+v", got)
	}
	in.Net = nil
	if got := liquidity.Compute(in).NetLiquidity; got.Value != "" || got.Reason != compute.NoInput {
		t.Errorf("nil NetFunc %+v", got)
	}
}

func TestLiquidityNetStale(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	in.Net = func(_, _, _ compute.Series, _ contracts.Day) liquidityapi.Num {
		return liquidityapi.Num{Value: "1", Day: "2026-11-11", Aux: "2026-11-10"}
	}
	in.Day = plus("2026-11-11", 14)
	if got := liquidity.Compute(in).NetLiquidity; got.Value != "1" || got.Reason != "" {
		t.Errorf("14 days %+v", got)
	}
	in.Day = plus("2026-11-11", 15)
	if got := liquidity.Compute(in).NetLiquidity; got.Value != "" || got.Reason != compute.StaleInput {
		t.Errorf("15 days %+v", got)
	}
}

func TestLiquidityEmptyInput(t *testing.T) {
	t.Parallel()
	for _, in := range []liquidityapi.Input{
		{Day: "2026-11-12", Params: liquidity.TrialParams()},
		{},
	} {
		r := liquidity.Compute(in)
		nums := map[string]liquidityapi.Num{
			"Reserves": r.Reserves, "RRP": r.RRP, "SOFR": r.SOFR, "IORB": r.IORB, "SOFR99": r.SOFR99,
			"SofrIorbBP": r.SofrIorbBP, "HY": r.HighYield, "IG": r.InvestmentGrade, "Baa": r.Baa10Y,
			"NFCI": r.NFCI, "STLFSI4": r.STLFSI4, "Ratio": r.ReservesOverAssets, "Discount": r.DiscountWindow,
			"Net": r.NetLiquidity, "4w": r.Reserves4w.Num, "13w": r.Reserves13w.Num, "Rank": r.HighYieldRank.Num,
		}
		for name, n := range nums {
			if n.Value != "" || n.Reason == "" {
				t.Errorf("%s: %+v", name, n)
			}
		}
	}
}
