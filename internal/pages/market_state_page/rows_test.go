package market_state_page

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestRowsCarryWindowAndDay(t *testing.T) {
	t.Parallel()
	rows := marketRows(build(t, fullConfig(t, sampleResult())))
	ids := []string{"sp500_vs_200d", "sp500_drawdown", "vix", "vix_term", "hy_spread"}
	days := []string{"2026-11-12", "2026-11-13", "2026-11-13", "2026-11-13", "2026-11-12"}
	texts := []string{
		"S&P 500 is 3.12% above its 200-day average",
		"S&P 500 is 2.40% below its 60-session high of Oct 29",
		"VIX 18.5 on Nov 13, at the 38th percentile of the last 3 years",
		"VIX is 0.92 of VIX3M on Nov 13",
		"High-yield spread 3.12% on Nov 12, at the 41st percentile of our stored history since 2023-11-14 (780 days)",
	}
	if len(rows) != len(ids) {
		t.Fatalf("%d rows", len(rows))
	}
	for i, r := range rows {
		row := obj(r)
		if row["id"] != ids[i] {
			t.Errorf("row %d id %v", i, row["id"])
		}
		if w, _ := row["window"].(string); w == "" {
			t.Errorf("%s: no window", ids[i])
		}
		if row["day"] != days[i] {
			t.Errorf("%s: day %v, want %s", ids[i], row["day"], days[i])
		}
		if got := obj(row["figure"])["text"]; got != texts[i] {
			t.Errorf("%s: text %q, want %q", ids[i], got, texts[i])
		}
	}
	for _, i := range []int{0, 1, 3} {
		if w := obj(rows[i])["window"].(string); !strings.Contains(w, "Level") {
			t.Errorf("%s: window %q does not say level", ids[i], w)
		}
	}
}

func TestRowDayIsGapWhenFigureIsGap(t *testing.T) {
	t.Parallel()
	c := fullConfig(t, sampleResult())
	c.Liquidity = func(liquidityapi.Input) liquidityapi.Result {
		return liquidityapi.Result{HighYield: liquidityapi.Num{Reason: "stale_input"}}
	}
	hy := obj(marketRows(build(t, c))[4])
	if !isGap(hy["figure"]) || obj(hy["figure"])["reason"] != "stale_input" || !isGap(hy["day"]) {
		t.Errorf("hy_spread = %v", hy)
	}
	if !strings.Contains(obj(hy["percentile"])["text"].(string), "no percentile") {
		t.Errorf("percentile = %v", hy["percentile"])
	}
}

func TestPercentileOnlyVixAndHY(t *testing.T) {
	t.Parallel()
	for _, r := range marketRows(build(t, fullConfig(t, sampleResult()))) {
		row := obj(r)
		_, has := row["percentile"]
		want := row["id"] == "vix" || row["id"] == "hy_spread"
		if has != want {
			t.Errorf("%v: percentile present = %v", row["id"], has)
		}
		if has {
			p := obj(row["percentile"])
			if p["text"] == "" || p["window"] == "" {
				t.Errorf("%v: percentile = %v", row["id"], p)
			}
		}
	}
}

func TestHyPercentileShortHistoryKeepsLevel(t *testing.T) {
	t.Parallel()
	c := fullConfig(t, sampleResult())
	c.Liquidity = func(liquidityapi.Input) liquidityapi.Result {
		return liquidityapi.Result{
			HighYield:     liquidityapi.Num{Value: "3.12", Day: "2026-11-12"},
			HighYieldRank: liquidityapi.Rank{Num: liquidityapi.Num{Reason: "short_history"}, Days: 100},
		}
	}
	hy := obj(marketRows(build(t, c))[4])
	if isGap(hy["figure"]) || !strings.Contains(obj(hy["percentile"])["text"].(string), "no percentile") {
		t.Errorf("hy_spread = %v", hy)
	}
}

func TestCaptionNamesFirstDayCount(t *testing.T) {
	t.Parallel()
	rows := marketRows(build(t, fullConfig(t, sampleResult())))
	h := obj(obj(rows[2])["history"])
	if h["caption"] != "2 days, 2026-11-12 to 2026-11-13" {
		t.Errorf("caption = %v", h["caption"])
	}
	pts := list(h["points"])
	if len(pts) != 2 || obj(pts[1])["day"] != "2026-11-13" || obj(pts[1])["text"] != "18.5" {
		t.Errorf("points = %v", pts)
	}
	// a stored fraction is shown as percent
	sp := obj(obj(rows[0])["history"])
	if obj(list(sp["points"])[0])["text"] != "1750.00%" {
		t.Errorf("sp500 points = %v", sp["points"])
	}
	// fewer than two points is a Gap
	res := sampleResult()
	res.History.VIX = line("2026-11-13")
	got := obj(marketRows(build(t, fullConfig(t, res)))[2])["history"]
	if !isGap(got) || obj(got)["reason"] != "no_data" {
		t.Errorf("one point: history = %v", got)
	}
}

func TestHistoryCaptionWhenCapped(t *testing.T) {
	t.Parallel()
	res := sampleResult()
	l := res.History.VIX
	l.Points = nil
	for i := 0; i < 800; i++ {
		l.Points = append(l.Points, line("2026-11-13").Points[0])
		l.Points[i].Day = dayAfter(i)
	}
	l.Total = 900
	res.History.VIX = l
	h := obj(obj(marketRows(build(t, fullConfig(t, res)))[2])["history"])
	if c := h["caption"].(string); c != "newest 800 of 900 days, "+string(dayAfter(0))+" to "+string(dayAfter(799)) {
		t.Errorf("caption = %q", c)
	}
	if len(list(h["points"])) != 800 {
		t.Errorf("%d points", len(list(h["points"])))
	}
}

func TestHistoryPointTextUsesTypographicMinus(t *testing.T) {
	t.Parallel()
	l := marketstateapi.Line{Total: 2, Points: []marketstateapi.Point{
		{Day: contracts.Day("2026-11-12"), Value: "-0.0209"},
		{Day: contracts.Day("2026-11-13"), Value: "0.015"},
	}}
	h, ok := historyCell(l, "x", fmtPercentShift).(history)
	if !ok || len(h.Points) != 2 {
		t.Fatalf("historyCell = %#v", h)
	}
	if h.Points[0].Text != "−2.09%" || h.Points[1].Text != "1.50%" || string(h.Points[0].Value) != "-2.09" {
		t.Errorf("points = %+v", h.Points)
	}
	l.Points[0].Value = "-0.00001"
	h, _ = historyCell(l, "x", fmtPercentShift).(history)
	if h.Points[0].Text != "0.00%" {
		t.Errorf("zero text = %q", h.Points[0].Text)
	}
}

// longLine is n points of value v(i), one per day.
func longLine(n int, v func(i int) float64) marketstateapi.Line {
	l := marketstateapi.Line{Total: n}
	for i := 0; i < n; i++ {
		l.Points = append(l.Points, marketstateapi.Point{Day: dayAfter(i), Value: fmt.Sprintf("%.4f", v(i))})
	}
	return l
}

func TestRowsCarryRangeAndHint(t *testing.T) {
	t.Parallel()
	res := sampleResult()
	up := func(i int) float64 { return 10 + float64(i)/100 }
	res.History = marketstateapi.History{
		Sp500Vs200d: longLine(300, func(i int) float64 { return float64(i-100) / 1000 }),
		Drawdown:    longLine(300, func(i int) float64 { return -float64(i) / 10 }),
		VIX:         longLine(300, up), VIXTerm: longLine(300, up), HighYield: longLine(300, up),
	}
	b := buildBytes(t, fullConfig(t, res))
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := compileSchema(t).Validate(inst); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	rows := marketRows(decode(t, b))
	for _, r := range rows {
		row := obj(r)
		hint, _ := row["hint"].(string)
		if hint == "" {
			t.Errorf("%v: no hint", row["id"])
		}
		rg := obj(row["range"])
		if isGap(rg) {
			t.Errorf("%v: range is a Gap: %v", row["id"], rg)
			continue
		}
		if !strings.HasSuffix(rg["state_text"].(string), "trial") || len(list(rg["cuts"])) != 2 {
			t.Errorf("%v: range = %v", row["id"], rg)
		}
	}
	if st := obj(obj(rows[2])["range"])["state"]; st != "normal" {
		t.Errorf("vix 18.5 state = %v", st)
	}
	// a row with a Gap figure has a Gap range and still carries the hint
	c := fullConfig(t, res)
	c.Liquidity = func(liquidityapi.Input) liquidityapi.Result {
		return liquidityapi.Result{HighYield: liquidityapi.Num{Reason: "stale_input"}}
	}
	hy := obj(marketRows(build(t, c))[4])
	if !isGap(hy["range"]) || hy["hint"] == "" {
		t.Errorf("hy_spread = %v", hy)
	}
}

func buildBytes(t *testing.T, c Config) []byte {
	t.Helper()
	b, err := Build(context.Background(), c, fakeSeries{}, testDay, time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
