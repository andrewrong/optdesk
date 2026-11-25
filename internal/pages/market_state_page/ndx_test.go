package market_state_page

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeIndexIDs answers IndexIDs with one row per known name, and two rows for a name listed in dup.
type fakeIndexIDs struct {
	ids map[string]int64
	dup bool
}

func (f fakeIndexIDs) IndexIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range names {
		if id, ok := f.ids[n]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "index", Name: n})
			if f.dup {
				out = append(out, store.InstrumentName{InstrumentID: id + 1, Type: "index", Name: n})
			}
		}
	}
	return out, nil
}

func ndxBars(id int64, days []string) *fakeBars {
	fb := &fakeBars{bars: map[int64][]contracts.PriceBar{}}
	for i, d := range days {
		fb.bars[id] = append(fb.bars[id], contracts.PriceBar{InstrumentID: int(id), Basis: "trades",
			SessionDay: sdate(d), Close: fmt.Sprintf("%d.50", 1000+i)})
	}
	return fb
}

func TestNasdaqSeriesFromBars(t *testing.T) {
	t.Parallel()
	days := weekdays("2026-10-01", testDay)
	c := fullConfig(t, sampleResult())
	c.IndexIDs = fakeIndexIDs{ids: map[string]int64{"NDX": 100}}
	c.Bars = ndxBars(100, days)
	var got marketstateapi.Input
	c.Market = func(in marketstateapi.Input) marketstateapi.Result { got = in; return sampleResult() }
	build(t, c)
	if len(got.NDX) != len(days) {
		t.Fatalf("%d NDX points, want %d", len(got.NDX), len(days))
	}
	for i, p := range got.NDX {
		if string(p.Day) != days[i] || p.Value != float64(1000+i)+0.5 {
			t.Fatalf("point %d = %+v, want %s %d.5", i, p, days[i], 1000+i)
		}
	}
}

func nasdaqRowOf(t *testing.T, c Config) map[string]any {
	t.Helper()
	rows := marketRows(build(t, c))
	if len(rows) != 6 {
		t.Fatalf("%d rows", len(rows))
	}
	return obj(rows[2])
}

func TestNasdaqDrawdownRow(t *testing.T) {
	t.Parallel()
	res := sampleResult()
	res.History.NasdaqDrawdown = longLine(300, func(i int) float64 { return -float64(i) / 10 })
	row := nasdaqRowOf(t, fullConfig(t, res))
	if row["id"] != "nasdaq_drawdown" || row["label"] != "Nasdaq-100 drawdown" {
		t.Errorf("id/label = %v / %v", row["id"], row["label"])
	}
	if got := obj(row["figure"])["text"]; got != "Nasdaq-100 is 3.10% below its 60-session high of Nov 2" {
		t.Errorf("figure text = %q", got)
	}
	if row["window"] != "60 个交易日水平：最新收盘价对比最近 60 个收盘价中的最高值" || row["day"] != "2026-11-13" {
		t.Errorf("window/day = %v / %v", row["window"], row["day"])
	}
	if row["hint"] != "纳斯达克 100 距近 60 个交易日最高点的回撤。区间按它自己的历史分位给出（越低越差），试用，只是提示。" {
		t.Errorf("hint = %v", row["hint"])
	}
	rg := obj(row["range"])
	cuts := list(rg["cuts"])
	if rg["state"] != "normal" || !strings.HasSuffix(rg["state_text"].(string), "trial") || len(cuts) != 2 {
		t.Fatalf("range = %v", rg)
	}
	if c0, c1 := obj(cuts[0])["text"], obj(cuts[1])["text"]; c0 != "Caution when below \u221224.00%, the 20th percentile of 300 days" ||
		c1 != "Alarm when below \u221228.50%, the 5th percentile of 300 days" {
		t.Errorf("cuts = %q / %q", c0, c1)
	}
	if n := rg["notice"]; n != "A notice only; nothing is blocked." {
		t.Errorf("notice = %v", n)
	}
	res.NasdaqDrawdown.Value = "0"
	row = nasdaqRowOf(t, fullConfig(t, res))
	if got := obj(row["figure"])["text"]; got != "Nasdaq-100 is at its 60-session high of Nov 2" {
		t.Errorf("zero text = %q", got)
	}
}

func TestNasdaqRowGaps(t *testing.T) {
	t.Parallel()
	ok := map[string]int64{"NDX": 100}
	for name, tc := range map[string]struct {
		edit   func(*Config)
		reason string
	}{
		"no index reader": {func(c *Config) { c.IndexIDs = nil }, "no_reader"},
		"no bar reader":   {func(c *Config) { c.Bars = nil }, "no_reader"},
		"no instrument":   {func(c *Config) { c.IndexIDs = fakeIndexIDs{ids: map[string]int64{"SPX": 5}} }, "no_data"},
		"two instruments": {func(c *Config) { c.IndexIDs = fakeIndexIDs{ids: ok, dup: true} }, "missing_input"},
	} {
		base := marketRows(build(t, fullConfig(t, sampleResult())))
		c := fullConfig(t, sampleResult())
		tc.edit(&c)
		rows := marketRows(build(t, c))
		fig := obj(obj(rows[2])["figure"])
		if fig["kind"] != "gap" || fig["reason"] != tc.reason {
			t.Errorf("%s: figure = %v", name, fig)
		}
		for i := range rows {
			if i == 2 || (name == "no bar reader") {
				continue
			}
			if fmt.Sprint(rows[i]) != fmt.Sprint(base[i]) {
				t.Errorf("%s: row %d changed", name, i)
			}
		}
	}
}

func TestNasdaqBarsJoinBarsSource(t *testing.T) {
	t.Parallel()
	c := fullConfig(t, sampleResult())
	c.IndexIDs = fakeIndexIDs{ids: map[string]int64{"NDX": 100}}
	c.Bars = &fakeBars{bars: map[int64][]contracts.PriceBar{100: {
		{InstrumentID: 100, Basis: "trades", SessionDay: sdate("2026-11-12"), Close: "20000", Source: "ibkr"},
		{InstrumentID: 100, Basis: "trades", SessionDay: sdate("2026-11-13"), Close: "20100", Source: "yfinance"},
	}}}
	m := decode(t, buildBytes(t, c))
	if m["bars_source"] == nil {
		t.Fatalf("NDX bars from yfinance are not in bars_source")
	}
}
