package liquidityapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidity"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
)

// TestLiquidityFuncContract: a fake Func is handed an Input and the caller reads every field of Result.
func TestLiquidityFuncContract(t *testing.T) {
	t.Parallel()
	var f liquidityapi.Func = func(in liquidityapi.Input) liquidityapi.Result {
		n := liquidityapi.Num{Value: string(in.Day), Day: in.Day, Aux: in.Day}
		gap := liquidityapi.Num{Reason: liquidityapi.MissingInput}
		return liquidityapi.Result{
			Reserves: n, RRP: n, SOFR: n, IORB: gap, SOFR99: n, SofrIorbBP: n,
			Reserves4w:  liquidityapi.Change{Num: n, Weeks: 4, From: in.Day},
			Reserves13w: liquidityapi.Change{Num: gap, Weeks: 13},
			HighYield:   n, InvestmentGrade: n, Baa10Y: n,
			HighYieldRank: liquidityapi.Rank{Num: n, First: in.Day, Days: len(in.BAMLH0A0HYM2)},
			NFCI:          n, STLFSI4: n, ReservesOverAssets: n, DiscountWindow: n,
			NetLiquidity: liquidityapi.Num{Reason: compute.NoInput},
		}
	}
	in := liquidityapi.Input{
		Day: "2026-11-12", BAMLH0A0HYM2: compute.Series{{Day: "2026-11-12", Value: 3}},
		Params: liquidityapi.Params{DailyMaxAgeDays: 4, WeeklyMaxAgeDays: 14, PercentileMinDays: 250},
	}
	r := f(in)
	if r.Reserves.Value != "2026-11-12" || r.RRP.Aux != in.Day || r.IORB.Reason != "missing_input" ||
		r.Reserves4w.Weeks != 4 || r.Reserves4w.From != in.Day || r.Reserves13w.Weeks != 13 ||
		r.SOFR.Day != in.Day || r.SOFR99.Value == "" || r.SofrIorbBP.Value == "" ||
		r.HighYield.Value == "" || r.InvestmentGrade.Value == "" || r.Baa10Y.Value == "" ||
		r.HighYieldRank.Days != 1 || r.HighYieldRank.First != in.Day || r.NFCI.Value == "" ||
		r.STLFSI4.Value == "" || r.ReservesOverAssets.Value == "" || r.DiscountWindow.Value == "" ||
		r.NetLiquidity.Reason != compute.NoInput {
		t.Fatalf("result %+v", r)
	}
}

// TestLiquidityComputeAssignableToFunc: the implementation satisfies the declared type.
func TestLiquidityComputeAssignableToFunc(t *testing.T) {
	t.Parallel()
	var f liquidityapi.Func = liquidity.Compute
	if f(liquidityapi.Input{Day: "2026-11-12"}).Reserves.Reason != compute.NoInput {
		t.Fatal("empty input is not a gap")
	}
}
