package household_page_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeTrend answers a fixed result and records the input it was given.
type fakeTrend struct {
	res blockapi.FamilyTrendResult
	got *blockapi.FamilyTrendIn
}

func (f fakeTrend) FamilyTrend(in blockapi.FamilyTrendIn) blockapi.FamilyTrendResult {
	*f.got = in
	return f.res
}

func pts(vals ...string) []blockapi.FamilyTrendPoint {
	days := []contracts.Day{"2026-11-09", "2026-11-10", "2026-11-11"}
	var out []blockapi.FamilyTrendPoint
	for i, v := range vals {
		out = append(out, blockapi.FamilyTrendPoint{Day: days[i], Net: v})
	}
	return out
}

func trendRig(res blockapi.FamilyTrendResult, aliases ...string) (famRig, *blockapi.FamilyTrendIn) {
	var included []blockapi.FamilyAccountOut
	for _, a := range aliases {
		included = append(included, counted(a, "2026-11-10", "1000.00", "10.00"))
	}
	r := rig(famResult("1000.00", "10.00", "1", included...), aliases...)
	got := &blockapi.FamilyTrendIn{}
	r.c.FamilyTrend = func() blockapi.FamilyTrendFigure { return fakeTrend{res: res, got: got} }
	return r, got
}

func trendOf(m map[string]any) map[string]any { return m["trend"].(map[string]any) }

func TestFamilyTrendBlock(t *testing.T) {
	t.Parallel()
	res := blockapi.FamilyTrendResult{
		Family: pts("105000.00", "106000.50"),
		Accounts: []blockapi.FamilyTrendAccountOut{
			{Alias: "main", Points: pts("100000.00", "100500.50")},
			{Alias: "wife", Points: pts("5000.00", "5500.00")},
		},
		LeftOut: 2,
	}
	r, got := trendRig(res, "main", "wife")
	m, b := r.build(t)
	validate(t, b)
	tb := trendOf(m)
	if tb["gap"] != nil {
		t.Fatalf("gap: %v", tb["gap"])
	}
	rows := tb["rows"].([]any)
	if len(rows) != 2 || rows[1].(map[string]any)["day"] != "2026-11-10" ||
		rows[1].(map[string]any)["value"].(map[string]any)["value"].(float64) != 106000.5 {
		t.Fatalf("rows: %v", rows)
	}
	series := tb["series"].([]any)
	if len(series) != 3 {
		t.Fatalf("series: %v", series)
	}
	s0, s1, s2 := series[0].(map[string]any), series[1].(map[string]any), series[2].(map[string]any)
	if s0["scope"] != "household" || s0["label"] != "全家合计" || s1["scope"] != "main" || s1["label"] != "账户（main）" ||
		s2["scope"] != "wife" || len(s2["rows"].([]any)) != 2 {
		t.Fatalf("series: %v", series)
	}
	want := "全家合计：2026-11-09 至 2026-11-10，共 2 天。有 2 天没有画点：当天至少一个账户没有净值记录。"
	if tb["window"] != want || s0["window"] != want {
		t.Fatalf("window = %v", tb["window"])
	}
	if !slices.Equal([]string{got.Accounts[0].Alias, got.Accounts[1].Alias}, []string{"main", "wife"}) || len(got.Window) == 0 {
		t.Fatalf("figure input: %+v", got)
	}
	if len(got.Accounts[0].Days) != 3 || got.Accounts[0].Days[0].Day != "2026-10-30" {
		t.Fatalf("history is not ascending: %v", got.Accounts[0].Days)
	}
}

func TestFamilyTrendDifferentNewestDays(t *testing.T) {
	t.Parallel()
	res := blockapi.FamilyTrendResult{
		Family:   pts("100.00", "101.00"),
		Accounts: []blockapi.FamilyTrendAccountOut{{Alias: "main", Points: pts("60.00", "61.00")}, {Alias: "wife", Points: pts("40.00", "40.00")}},
	}
	r, _ := trendRig(res, "main", "wife")
	// the wife's newest capture is the day before the main account's
	r.acc.values["wife"] = map[string][]store.AccountValue{"2026-11-09": vals("2026-11-09", "40.00", "1.00")}
	m, b := r.build(t)
	validate(t, b)
	rows := trendOf(m)["rows"].([]any)
	last := rows[len(rows)-1].(map[string]any)["day"].(string)
	if last > "2026-11-10" {
		t.Fatalf("last trend day %s is after the earlier newest day", last)
	}
	if w := trendOf(m)["window"].(string); w != "全家合计：2026-11-09 至 2026-11-10，共 2 天。" {
		t.Fatalf("window = %q", w)
	}
}

func TestFamilyTrendGapNoData(t *testing.T) {
	t.Parallel()
	r, _ := trendRig(blockapi.FamilyTrendResult{Gap: "no_data", Family: pts("1.00")}, "main", "wife")
	m, b := r.build(t)
	validate(t, b)
	tb := trendOf(m)
	g := tb["gap"].(map[string]any)
	if g["reason"] != "no_data" || g["text"] != "全家净值记录不足 2 天，画不出走势" || len(tb["series"].([]any)) != 0 || len(tb["rows"].([]any)) != 0 {
		t.Fatalf("trend: %v", tb)
	}
}

func TestFamilyTrendManyAccounts(t *testing.T) {
	t.Parallel()
	res := blockapi.FamilyTrendResult{Family: pts("400.00", "410.00")}
	for _, a := range []string{"a", "b", "c", "d"} {
		res.Accounts = append(res.Accounts, blockapi.FamilyTrendAccountOut{Alias: a, Points: pts("100.00", "102.50")})
	}
	r, _ := trendRig(res, "a", "b", "c", "d")
	m, b := r.build(t)
	validate(t, b)
	series := trendOf(m)["series"].([]any)
	if len(series) != 1 || series[0].(map[string]any)["scope"] != "household" {
		t.Fatalf("series: %v", series)
	}
	if !has(strs(m["lead_notes"]), "账户超过 3 个，只画全家合计") {
		t.Fatalf("lead_notes = %v", m["lead_notes"])
	}
}

func TestFamilyTrendNilFigureKeepsNotBuilt(t *testing.T) {
	t.Parallel()
	r, _ := trendRig(blockapi.FamilyTrendResult{}, "main", "wife")
	r.c.FamilyTrend = nil
	m, _ := r.build(t)
	if g := trendOf(m)["gap"].(map[string]any); g["reason"] != "not_built" {
		t.Fatalf("gap = %v", g)
	}
}

func TestFamilyTrendYenReadsRatePerDay(t *testing.T) {
	t.Parallel()
	res := blockapi.FamilyTrendResult{Family: pts("1.00", "2.00"), YenUsed: true,
		Accounts: []blockapi.FamilyTrendAccountOut{{Alias: "main", Points: pts("1.00", "2.00")}, {Alias: "wife", Points: pts("1.00", "1.00")}}}
	r, got := trendRig(res, "main", "wife")
	fx := &fakeFx{rate: "150"}
	r.c.Fx = fx
	jpy := vals("2026-11-10", "15000000", "3000000")
	for i := range jpy {
		jpy[i].Currency = "JPY"
	}
	r.acc.values["wife"] = map[string][]store.AccountValue{"2026-11-10": jpy}
	m, _ := r.build(t)
	if got.UsdJpy["2026-11-10"] != "150" {
		t.Fatalf("rates: %v", got.UsdJpy)
	}
	if w := trendOf(m)["window"].(string); w != "全家合计：2026-11-09 至 2026-11-10，共 2 天。日元账户按当天的美元兑日元汇率折成美元。" {
		t.Fatalf("window = %q", w)
	}
	// an Fx that fails leaves the day without a rate and the page built
	r.c.Fx = &fakeFx{err: context_err{}}
	r.c.FamilyTrend = func() blockapi.FamilyTrendFigure { return fakeTrend{res: res, got: got} }
	if _, err := household_page.BuildFamily(t.Context(), r.c, day, at); err != nil || len(got.UsdJpy) != 0 {
		t.Fatalf("err %v rates %v", err, got.UsdJpy)
	}
}

type context_err struct{}

func (context_err) Error() string { return "fx down" }

func TestFamilyTrendNoRateReadWithoutYen(t *testing.T) {
	t.Parallel()
	r, _ := trendRig(blockapi.FamilyTrendResult{Family: pts("1.00", "2.00")}, "main", "wife")
	fx := &fakeFx{rate: "150"}
	r.c.Fx = fx
	r.build(t)
	if len(fx.calls) != 0 {
		t.Fatalf("rate reads: %v", fx.calls)
	}
}

func TestFamilyTrendNewLeftOutSinceFirstRecord(t *testing.T) {
	t.Parallel()
	recorded := []string{"2026-11-05", "2026-11-06", "2026-11-09", "2026-11-10"}
	cases := []struct {
		name    string
		wifeAt  []string
		want    string
		notWant []string
	}{
		{"all four days have family points", recorded,
			"全家合计：2026-11-05 至 2026-11-10，共 4 天。", []string{"86", "90"}},
		{"the wife starts one day later", recorded[1:],
			"全家合计：2026-11-06 至 2026-11-10，共 3 天。有 1 天没有画点：当天至少一个账户没有净值记录。", []string{"86", "87"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, _ := trendRig(blockapi.FamilyTrendResult{}, "main", "wife")
			r.c.FamilyTrend = pagefig.NewFamilyTrend
			r.acc.values["main"], r.acc.values["wife"] = map[string][]store.AccountValue{}, map[string][]store.AccountValue{}
			for _, d := range recorded {
				r.acc.values["main"][d] = vals(d, "100.00", "1.00")
			}
			for _, d := range tc.wifeAt {
				r.acc.values["wife"][d] = vals(d, "10.00", "1.00")
			}
			m, _ := r.build(t)
			w, _ := trendOf(m)["window"].(string)
			if w != tc.want {
				t.Fatalf("window = %q, want %q", w, tc.want)
			}
			for _, n := range tc.notWant {
				if strings.Contains(w, n) {
					t.Fatalf("window %q holds %q", w, n)
				}
			}
		})
	}
}
