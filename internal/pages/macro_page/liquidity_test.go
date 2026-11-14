package macro_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func num(v string, day contracts.Day) liquidityapi.Num { return liquidityapi.Num{Value: v, Day: day} }

func fullResult() liquidityapi.Result {
	return liquidityapi.Result{
		Reserves:   num("2950000000000", "2026-11-11"),
		RRP:        num("300000000", "2026-11-13"),
		Reserves4w: liquidityapi.Change{Num: num("48000000000", "2026-11-11"), Weeks: 4, From: "2026-10-14"},
		Reserves13w: liquidityapi.Change{Num: num("-112000000000", "2026-11-11"), Weeks: 13,
			From: "2026-08-12"},
		SOFR: num("3.92", "2026-11-12"), IORB: num("3.9", "2026-11-12"), SOFR99: num("3.98", "2026-11-12"),
		SofrIorbBP: num("2.0", "2026-11-12"),
		HighYield:  num("3.12", "2026-11-12"), InvestmentGrade: num("0.89", "2026-11-12"), Baa10Y: num("1.62", "2026-11-12"),
		HighYieldRank: liquidityapi.Rank{Num: num("41", "2026-11-12"), First: "2023-11-14", Days: 780},
		NFCI:          num("-0.46", "2026-11-06"), STLFSI4: num("-0.55", "2026-11-06"),
		ReservesOverAssets: num("12.34", "2026-11-11"),
		DiscountWindow:     num("5000000000", "2026-11-11"),
		NetLiquidity:       liquidityapi.Num{Value: "5820000000000", Day: "2026-11-05", Aux: "2026-11-03"},
	}
}

func fixedFunc(r liquidityapi.Result) liquidityapi.Func {
	return func(liquidityapi.Input) liquidityapi.Result { return r }
}

func liqConfig(r liquidityapi.Result) Config {
	c := cfg("DGS10")
	c.Liquidity = fixedFunc(r)
	return c
}

func liqBuild(t *testing.T, c Config) map[string]any {
	t.Helper()
	b, err := Build(t.Context(), fakeReader{}, c, "2026-11-13", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2(t, b); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func validateV2(t *testing.T, b []byte) error {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "contracts", "schemas")
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, n := range []string{"page.macro_page.v2.json", "common.v1.json"} {
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource("https://optdesk.invalid/schemas/"+n, doc); err != nil {
			t.Fatal(err)
		}
	}
	s, err := c.Compile("https://optdesk.invalid/schemas/page.macro_page.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return s.Validate(v)
}

func liqBlock(m map[string]any) map[string]any { return m["liquidity"].(map[string]any) }

func liqLine(m map[string]any, i int) map[string]any {
	return liqBlock(m)["lines"].([]any)[i].(map[string]any)
}

func liqTap(m map[string]any, key string) map[string]any {
	for _, x := range liqBlock(m)["taps"].([]any) {
		if x.(map[string]any)["key"] == key {
			return x.(map[string]any)
		}
	}
	return nil
}

func TestLiquidityLinesFromResult(t *testing.T) {
	t.Parallel()
	m := liqBuild(t, liqConfig(fullResult()))
	want := []string{
		"Bank reserves were $2.95tn in the week to Nov 11, up $48bn over 4 weeks and down $112bn over 13 weeks; ON RRP is $0.3bn on Nov 13.",
		"SOFR was 3.92% on Nov 12, 2.0 bp above IORB 3.90% on Nov 12; its 99th percentile was 3.98% on Nov 12.",
		"High-yield spread is 3.12% on Nov 12, at the 41st percentile of our stored history since 2023-11-14 (780 days); " +
			"investment grade 0.89% on Nov 12; Baa minus 10-year 1.62% on Nov 12.",
		"Chicago Fed NFCI is -0.46 for the week to Nov 6; above 0 means tighter than average.",
	}
	keys := []string{"reserves", "money_market", "credit", "conditions"}
	for i, w := range want {
		l := liqLine(m, i)
		if l["sentence"] != w || l["key"] != keys[i] {
			t.Errorf("line %d = %v, want %q", i, l, w)
		}
	}
	if m["liquidity_title"] != "Funding and credit background" {
		t.Errorf("title %v", m["liquidity_title"])
	}
	if got := liqTap(m, "stlfsi")["note"]; !strings.Contains(got.(string), "above 0 = above-average stress") {
		t.Errorf("stlfsi note %v", got)
	}
	for _, k := range []string{"reserves_over_assets", "discount_window"} {
		tp := liqTap(m, k)
		if tp["note"] != nil {
			t.Errorf("%s has a note %v", k, tp["note"])
		}
		if r := tp["rows"].([]any)[0].(map[string]any); r["data_day"] != "2026-11-11" {
			t.Errorf("%s row %v", k, r)
		}
	}
	if r := liqTap(m, "reserves_over_assets")["rows"].([]any)[0].(map[string]any); r["text"] != "12.34%" {
		t.Errorf("ratio row %v", r)
	}
	if r := liqTap(m, "discount_window")["rows"].([]any)[0].(map[string]any); r["text"] != "$5.0bn" {
		t.Errorf("discount row %v", r)
	}
}

func TestLiquidityNetLine(t *testing.T) {
	t.Parallel()
	m := liqBuild(t, liqConfig(fullResult()))
	tp := liqTap(m, "net_liquidity")
	rows := tp["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	r := rows[0].(map[string]any)
	if r["text"] != "$5.82tn on Wednesday Nov 5, RRP of Nov 3" || r["data_day"] != "2026-11-05" {
		t.Errorf("row %v", r)
	}
	if tp["note"] != "Practitioner construct, no evidence it leads prices." {
		t.Errorf("note %v", tp["note"])
	}
	for _, x := range liqBlock(m)["taps"].([]any) {
		k := x.(map[string]any)["key"]
		if k != "net_liquidity" && k != "stlfsi" && x.(map[string]any)["note"] != nil {
			t.Errorf("tap %v has a note", k)
		}
	}
}

func isGap(v any, reason string) bool {
	g, ok := v.(map[string]any)
	return ok && g["reason"] == reason && g["text"] != ""
}

func TestLiquidityGapPerFigure(t *testing.T) {
	t.Parallel()
	res := fullResult()
	res.SOFR99 = liquidityapi.Num{Reason: compute.StaleInput}
	res.DiscountWindow = liquidityapi.Num{Reason: compute.NoInput}
	m := liqBuild(t, liqConfig(res))
	if !isGap(liqLine(m, 1)["sentence"], "stale_input") {
		t.Errorf("money line %v", liqLine(m, 1))
	}
	for _, i := range []int{0, 2, 3} {
		if _, ok := liqLine(m, i)["sentence"].(string); !ok {
			t.Errorf("line %d lost: %v", i, liqLine(m, i))
		}
	}
	r := liqTap(m, "discount_window")["rows"].([]any)[0].(map[string]any)
	if !isGap(r["text"], "no_data") || r["data_day"] != nil {
		t.Errorf("discount row %v", r)
	}
	if _, ok := liqTap(m, "stlfsi")["rows"].([]any)[0].(map[string]any)["text"].(string); !ok {
		t.Error("stlfsi row lost")
	}
}

func TestLiquidityReasonMapping(t *testing.T) {
	t.Parallel()
	cases := map[compute.Reason]string{
		compute.NoInput:           "no_data",
		liquidityapi.MissingInput: "missing_input",
		compute.ShortHistory:      "missing_input",
		compute.ZeroRange:         "missing_input",
		compute.BadInput:          "missing_input",
		compute.StaleInput:        "stale_input",
	}
	for reason, want := range cases {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()
			res := fullResult()
			res.NFCI = liquidityapi.Num{Reason: reason}
			m := liqBuild(t, liqConfig(res))
			if !isGap(liqLine(m, 3)["sentence"], want) {
				t.Errorf("line %v, want %s", liqLine(m, 3), want)
			}
			if strings.Contains(string(mustJSON(t, liqLine(m, 3))), "0.46") {
				t.Error("a number leaked")
			}
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLiquidityChangeGapWhenEitherEndMissing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*liquidityapi.Result)
		want   string
	}{
		{"newest end stale", func(r *liquidityapi.Result) { r.Reserves4w.Num = liquidityapi.Num{Reason: compute.StaleInput} }, "stale_input"},
		{"newest end missing", func(r *liquidityapi.Result) { r.Reserves13w.Num = liquidityapi.Num{Reason: compute.NoInput} }, "no_data"},
		{"earlier end missing", func(r *liquidityapi.Result) {
			r.Reserves4w.Num = liquidityapi.Num{Reason: liquidityapi.MissingInput}
		}, "missing_input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := fullResult()
			tc.mutate(&res)
			m := liqBuild(t, liqConfig(res))
			s := liqLine(m, 0)["sentence"]
			if !isGap(s, tc.want) {
				t.Fatalf("line = %v, want gap %s", s, tc.want)
			}
			if strings.Contains(string(mustJSON(t, s)), "$") {
				t.Errorf("a number in %v", s)
			}
		})
	}
}

func TestLiquidityNilFuncKeepsV1(t *testing.T) {
	t.Parallel()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{
		"DGS10": {pt(t, "2026-11-03", "2026-11-04", "percent", f64(4.25)), pt(t, "2026-11-04", "2026-11-05", "percent", f64(4.3))},
		"DGS2":  {pt(t, "2026-11-04", "2026-11-05", "percent", nil)},
	}}
	c := cfg("DGS10", "DGS2")
	c.Indicators[0].DetailHref = "/indicator/DGS10"
	c.LiquidityParams = liquidityapi.Params{DailyMaxAgeDays: 4}
	c.Net = func(_, _, _ compute.Series, _ contracts.Day) liquidityapi.Num { return liquidityapi.Num{} }
	b, err := Build(t.Context(), &recReader{inner: r}, c, "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Fatalf("got\n%s\nwant\n%s", b, golden)
	}
	if got := NewPage(c).Meta.Schema; got != "page.macro_page.v1" {
		t.Fatalf("schema %q", got)
	}
}

// recReader records what is read and can fail.
type recReader struct {
	mu    sync.Mutex
	inner fakeReader
	calls map[string]contracts.Day
	fail  string
}

func (r *recReader) SeriesPoints(ctx context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	r.mu.Lock()
	if r.calls == nil {
		r.calls = map[string]contracts.Day{}
	}
	r.calls[id] = from
	r.mu.Unlock()
	if id == r.fail {
		return nil, errors.New("boom")
	}
	return r.inner.SeriesPoints(ctx, id, from, to)
}

func TestLiquidityNilFuncReadsNoLiquiditySeries(t *testing.T) {
	t.Parallel()
	rr := &recReader{}
	if _, err := Build(t.Context(), rr, cfg("DGS10"), "2026-11-05", fixedAt); err != nil {
		t.Fatal(err)
	}
	if len(rr.calls) != 1 {
		t.Fatalf("reads %v, want only DGS10", rr.calls)
	}
}

func TestLiquidityFuncMakesV2(t *testing.T) {
	t.Parallel()
	var got []liquidityapi.Input
	c := cfg("DGS10")
	c.LiquidityParams = liquidityapi.Params{DailyMaxAgeDays: 4, WeeklyMaxAgeDays: 14, PercentileMinDays: 250}
	c.Net = func(_, _, _ compute.Series, _ contracts.Day) liquidityapi.Num { return liquidityapi.Num{} }
	c.Liquidity = func(in liquidityapi.Input) liquidityapi.Result {
		got = append(got, in)
		return fullResult()
	}
	rr := &recReader{inner: fakeReader{points: map[string][]contracts.SeriesPoint{
		"WRESBAL": {pt(t, "2026-11-04", "2026-11-04", "usd", f64(2.9e12)), pt(t, "2026-11-04", "2026-11-20", "usd", f64(1))},
	}}}
	b, err := Build(t.Context(), rr, c, "2026-11-13", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2(t, b); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Func called %d times", len(got))
	}
	in := got[0]
	if in.Day != "2026-11-13" || in.Params != c.LiquidityParams || in.Net == nil {
		t.Errorf("input %+v", in)
	}
	if len(in.WRESBAL) != 1 || in.WRESBAL[0] != (compute.Point{Day: "2026-11-04", Value: 2.9e12}) {
		t.Errorf("WRESBAL %v", in.WRESBAL)
	}
	if len(rr.calls) != 15 { // fourteen series and the indicator
		t.Errorf("reads %v", rr.calls)
	}
	if rr.calls["BAMLH0A0HYM2"] != "2000-01-01" || rr.calls["SOFR"] != "2025-10-09" {
		t.Errorf("from days %v", rr.calls)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["schema_version"] != float64(2) || m["liquidity_title"] != "Funding and credit background" || m["liquidity"] == nil {
		t.Errorf("v2 fields missing: %v", m["schema_version"])
	}
	if m["sentence"] != "Funding and credit background from stored series; events: no data yet." {
		t.Errorf("sentence %v", m["sentence"])
	}
	if NewPage(c).Meta.Schema != "page.macro_page.v2" {
		t.Error("meta is not v2")
	}
}

func TestLiquidityReadErrorFailsBuild(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"WRESBAL", "BAMLH0A0HYM2", "STLFSI4"} {
		rr := &recReader{fail: id}
		_, err := Build(t.Context(), rr, liqConfig(fullResult()), "2026-11-13", fixedAt)
		if err == nil || !strings.Contains(err.Error(), id) {
			t.Errorf("%s: err = %v", id, err)
		}
	}
}

func TestLiquidityNoScoreWords(t *testing.T) {
	t.Parallel()
	m := liqBuild(t, liqConfig(fullResult()))
	delete(m, "ease")
	b := strings.ToLower(string(mustJSON(t, m)))
	for _, w := range []string{"score", "regime", "signal", "bullish", "bearish", "looser", "green", "amber"} {
		if strings.Contains(b, w) {
			t.Errorf("the page says %q", w)
		}
	}
	if n := strings.Count(b, "tighter"); n != 1 {
		t.Errorf("tighter appears %d times, want the one zero-line clause", n)
	}
}
