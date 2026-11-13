package report_blocks_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/report_blocks"
	"github.com/andrewrong/optdesk/internal/pages/report_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeGlobal is a blockapi.GlobalFigures that answers a canned result and records its input.
type fakeGlobal struct {
	res blockapi.GlobalResult
	in  *blockapi.GlobalIn
}

func (f fakeGlobal) Global(in blockapi.GlobalIn) blockapi.GlobalResult {
	*f.in = in
	return f.res
}

// fakeStocks answers the stock instruments of names; ids maps a name to its instrument ids.
type fakeStocks struct {
	ids map[string][]int64
	err error
}

func (f fakeStocks) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []store.InstrumentName
	for _, n := range names {
		for _, id := range f.ids[n] {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

// fakeBarReader keeps ascending closes by instrument id and records every call.
type fakeBarReader struct {
	mu    sync.Mutex
	days  map[int64][]string // "day|close"
	calls []string
	err   error
}

func (f *fakeBarReader) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf("%d|%s|%s|%s", id, basis, from, to))
	if f.err != nil {
		return nil, f.err
	}
	var out []contracts.PriceBar
	for _, s := range f.days[id] {
		d, c, _ := strings.Cut(s, "|")
		tm, err := time.Parse("2006-01-02", d)
		if err != nil {
			return nil, err
		}
		out = append(out, contracts.PriceBar{InstrumentID: int(id), Close: c, SessionDay: types.SerializableDate{Time: tm}})
	}
	return out, nil
}

var assets = []report_blocks.Asset{
	{Symbol: "SPY", Label: "Stocks", Group: "proxy"},
	{Symbol: "TLT", Label: "Bonds", Group: "proxy"},
	{Symbol: "XLK", Label: "Technology", Group: "sector"},
	{Symbol: "XLE", Label: "Energy", Group: "sector"},
}

func val(v string) compute.FigureValue { return compute.FigureValue{Value: v} }
func gapv(r string) compute.FigureValue { return compute.FigureValue{Gap: r} }

func fullResult() blockapi.GlobalResult {
	return blockapi.GlobalResult{
		DataDay: "2026-11-09",
		Proxies: []blockapi.MoveLine{{D1: val("0.50"), D5: val("-1.25")}, {D1: val("0.00"), D5: val("2.00")}},
		Sectors: []blockapi.SectorLine{
			{Name: "Technology", Symbol: "XLK", D1: val("0.30"), D5: val("1.10"), D20: val("-0.40")},
			{Name: "Energy", Symbol: "XLE", D1: val("-0.20"), D5: val("-1.80"), D20: val("2.50")},
		},
		Rotation: &blockapi.RotationLine{Leaders5: []string{"Technology", "A", "B"}, Laggard: "Energy", LaggardD5: "-1.80", Leader20: "Energy"},
	}
}

type fixture struct {
	cfg  report_blocks.Config
	bars *fakeBarReader
	in   *blockapi.GlobalIn
}

func newFixture(res blockapi.GlobalResult) fixture {
	in := &blockapi.GlobalIn{}
	bars := &fakeBarReader{days: map[int64][]string{
		1: {"2026-11-06|10", "2026-11-09|11"}, 2: {"2026-11-09|5"}, 3: {"2026-11-09|7"}, 4: {"2026-11-09|8"},
	}}
	cfg := report_blocks.Config{
		Zone: zone, Assets: assets, Bars: bars,
		StockIDs: fakeStocks{ids: map[string][]int64{"SPY": {1}, "TLT": {2}, "XLK": {3}, "XLE": {4}}},
		Global:   func() blockapi.GlobalFigures { return fakeGlobal{res: res, in: in} },
	}
	return fixture{cfg, bars, in}
}

func buildGlobal(t *testing.T, c report_blocks.Config) ([]byte, map[string]any) {
	t.Helper()
	b, err := report_blocks.GlobalAssets(c)(context.Background(), day, at)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return b, m
}

func TestGlobalAssetsFullBlockValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	risks, err := os.ReadFile(filepath.Join("..", "report_page", "testdata", "risks_gap.json"))
	if err != nil {
		t.Fatal(err)
	}
	gapped := fullResult()
	gapped.Proxies[1].D5 = gapv("no_data")
	gapped.Rotation = nil
	allGap := blockapi.GlobalResult{Proxies: []blockapi.MoveLine{{D1: gapv("no_data"), D5: gapv("no_data")}, {D1: gapv("no_data"), D5: gapv("no_data")}},
		Sectors: []blockapi.SectorLine{{Name: "Technology", Symbol: "XLK", D1: gapv("no_data"), D5: gapv("no_data"), D20: gapv("no_data")}}}
	states := map[string]string{"full": "ok", "gapped": "partial", "allGap": "gap"}
	for name, res := range map[string]blockapi.GlobalResult{"full": fullResult(), "gapped": gapped, "allGap": allGap} {
		f := newFixture(res)
		_, m := buildGlobal(t, f.cfg)
		if m["state"] != states[name] {
			t.Errorf("%s state = %v, want %s", name, m["state"], states[name])
		}
		gaps := m["gaps"].([]any)
		if (name == "full") != (len(gaps) == 0) || len(gaps) > 20 {
			t.Errorf("%s gaps = %v", name, gaps)
		}
		pc := report_page.Config{Market: "US", Slot: "close", Zone: zone,
			Risks:        func(context.Context, contracts.Day, time.Time) ([]byte, error) { return risks, nil },
			GlobalAssets: report_page.BlockFunc(report_blocks.GlobalAssets(f.cfg))}
		page, err := report_page.Build(context.Background(), pc, day, at)
		if err != nil {
			t.Fatal(err)
		}
		validate(t, name, page)
	}
}

func TestGlobalAssetsFullBlockShape(t *testing.T) {
	t.Parallel()
	f := newFixture(fullResult())
	_, m := buildGlobal(t, f.cfg)
	px := m["proxies"].([]any)
	first := obj(t, px[0])
	if first["label"] != "Stocks" || first["symbol"] != "SPY" {
		t.Errorf("first proxy = %v", first)
	}
	d5 := obj(t, first["five_day"])
	if d5["text"] != "-1.25%" || d5["tone"] != "down" || d5["unit"] != "percent" {
		t.Errorf("proxy five_day = %v", d5)
	}
	if d := obj(t, obj(t, px[1])["day"]); d["text"] != "0.00%" || d["tone"] != "flat" {
		t.Errorf("flat proxy day = %v", d)
	}
	sec := obj(t, m["sector"])
	if sec["heading"] != "Sector excess over SPY" || len(sec["columns"].([]any)) != 5 {
		t.Errorf("sector head = %v", sec)
	}
	row := obj(t, sec["rows"].([]any)[0])
	if c := obj(t, row["sector"]); c["text"] != "Technology" || c["tone"] != "none" || c["unit"] != nil || c["value"] != nil {
		t.Errorf("sector cell = %v", c)
	}
	if c := obj(t, row["d1"]); c["text"] != "+0.30 pt" || c["tone"] != "up" || c["unit"] != "percentage_points" {
		t.Errorf("d1 cell = %v", c)
	}
	if r := obj(t, m["rotation"]); r["title"] != "Sector leaders" ||
		r["detail"] != "5-day leaders: Technology, A, B; laggard: Energy -1.80 pt; 20-day leader: Energy" {
		t.Errorf("rotation = %v", r)
	}
}

func TestGlobalAssetsNamesItsDay(t *testing.T) {
	t.Parallel()
	_, m := buildGlobal(t, newFixture(fullResult()).cfg)
	if m["data_times"] != "Closes of 2026-11-09" {
		t.Errorf("data_times = %v", m["data_times"])
	}
	res := fullResult()
	res.DataDay = ""
	_, m = buildGlobal(t, newFixture(res).cfg)
	if m["data_times"] != "no data yet" {
		t.Errorf("data_times without a day = %v", m["data_times"])
	}
}

func TestGlobalAssetsNoReaderIsGap(t *testing.T) {
	t.Parallel()
	mutate := map[string]func(*report_blocks.Config){
		"bars":   func(c *report_blocks.Config) { c.Bars = nil },
		"ids":    func(c *report_blocks.Config) { c.StockIDs = nil },
		"global": func(c *report_blocks.Config) { c.Global = nil },
		"assets": func(c *report_blocks.Config) { c.Assets = nil },
	}
	for name, mut := range mutate {
		f := newFixture(fullResult())
		mut(&f.cfg)
		_, m := buildGlobal(t, f.cfg)
		g := obj(t, m["gaps"].([]any)[0])
		if m["state"] != "gap" || g["reason"] != "no_reader" || obj(t, m["proxies"])["reason"] != "no_reader" {
			t.Errorf("%s nil: block = %v", name, m)
		}
		if len(f.bars.calls) != 0 {
			t.Errorf("%s nil: read %v", name, f.bars.calls)
		}
	}
}

func TestGlobalAssetsReaderErrorIsError(t *testing.T) {
	t.Parallel()
	f := newFixture(fullResult())
	f.bars.err = errors.New("boom")
	if _, err := report_blocks.GlobalAssets(f.cfg)(context.Background(), day, at); err == nil {
		t.Error("a bar reader error must be an error of the call")
	}
	f = newFixture(fullResult())
	f.cfg.StockIDs = fakeStocks{err: errors.New("boom")}
	if _, err := report_blocks.GlobalAssets(f.cfg)(context.Background(), day, at); err == nil {
		t.Error("an id reader error must be an error of the call")
	}
}

func TestGlobalAssetsUnknownSymbolIsNoData(t *testing.T) {
	t.Parallel()
	f := newFixture(fullResult())
	f.cfg.StockIDs = fakeStocks{ids: map[string][]int64{"SPY": {1}, "TLT": {2}, "XLK": {3}}}
	buildGlobal(t, f.cfg)
	for _, s := range f.in.Sectors {
		if s.Symbol == "XLE" && len(s.Closes) != 0 {
			t.Errorf("unknown symbol closes = %v, want none", s.Closes)
		}
		if s.Symbol == "XLK" && len(s.Closes) != 1 {
			t.Errorf("known symbol closes = %v", s.Closes)
		}
	}
	for _, c := range f.bars.calls {
		if strings.HasPrefix(c, "4|") {
			t.Errorf("read bars of an unknown symbol: %s", c)
		}
	}
}

func TestGlobalAssetsAmbiguousNameIsMissingInput(t *testing.T) {
	t.Parallel()
	f := newFixture(fullResult())
	f.cfg.StockIDs = fakeStocks{ids: map[string][]int64{"SPY": {1}, "TLT": {2, 9}, "XLK": {3}, "XLE": {4}}}
	buildGlobal(t, f.cfg)
	tlt := f.in.Proxies[1]
	if len(tlt.Closes) != 1 || tlt.Closes[0] != "?" || tlt.Days[0] != day {
		t.Errorf("ambiguous symbol = %+v, want one unreadable close on the page day", tlt)
	}
	for _, c := range f.bars.calls {
		if strings.HasPrefix(c, "2|") || strings.HasPrefix(c, "9|") {
			t.Errorf("read bars of an ambiguous symbol: %s", c)
		}
	}
}

func TestGlobalAssetsReadsEachSymbolOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(fullResult())
	buildGlobal(t, f.cfg)
	if len(f.bars.calls) != 4 {
		t.Fatalf("calls = %v, want one per symbol (SPY is read once though it is proxy and benchmark)", f.bars.calls)
	}
	want := "1|trades|2026-09-21|2026-11-10"
	if f.bars.calls[0] != want {
		t.Errorf("first call = %s, want %s", f.bars.calls[0], want)
	}
	spy := f.in.Benchmark
	if len(spy.Closes) != 2 || spy.Closes[0] != "11" || spy.Closes[1] != "10" || spy.Days[0] != "2026-11-09" || spy.Days[1] != "2026-11-06" {
		t.Errorf("benchmark = %+v, want newest first", spy)
	}
	if f.in.Day != day || len(f.in.Proxies) != 2 || len(f.in.Sectors) != 2 || f.in.Proxies[1].Name != "Bonds" {
		t.Errorf("input = %+v", *f.in)
	}
}

func TestGlobalAssetsRotationOmitted(t *testing.T) {
	t.Parallel()
	res := fullResult()
	res.Rotation = nil
	b, m := buildGlobal(t, newFixture(res).cfg)
	if _, ok := m["rotation"]; ok {
		t.Errorf("rotation present: %s", b)
	}
}

func TestGlobalAssetsTextsAreEnglishAndShort(t *testing.T) {
	t.Parallel()
	res := fullResult()
	res.Proxies[0].D1 = gapv("stale_input")
	res.Proxies[1].D5 = gapv("missing_input")
	res.Sectors[0].D20 = gapv("no_data")
	blocks := []map[string]any{}
	_, m := buildGlobal(t, newFixture(res).cfg)
	blocks = append(blocks, m)
	f := newFixture(res)
	f.cfg.Bars = nil
	_, m = buildGlobal(t, f.cfg)
	blocks = append(blocks, m)
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case string:
			if len(x) < 1 || len(x) > 200 {
				t.Errorf("%s: length %d", path, len(x))
			}
			for _, r := range x {
				if r > 127 {
					t.Errorf("%s: %q is not ASCII", path, x)
					break
				}
			}
		case map[string]any:
			for k, e := range x {
				walk(path+"."+k, e)
			}
		case []any:
			for i, e := range x {
				walk(fmt.Sprintf("%s[%d]", path, i), e)
			}
		}
	}
	for i, b := range blocks {
		walk(fmt.Sprintf("block%d", i), b)
	}
}
