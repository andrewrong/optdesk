package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// globalBars is a bar store of 30 weekdays ending at last; the close of listed symbol k on weekday i is 100+k+i.
type globalBars struct {
	bar.BarStore
	last time.Time
}

func (g globalBars) Close() error { return nil }

func (g globalBars) PriceBars(_ context.Context, id int64, _ string, _, _ contracts.Day) ([]contracts.PriceBar, error) {
	var days []time.Time
	for d := g.last; len(days) < 30; d = d.AddDate(0, 0, -1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			days = append([]time.Time{d}, days...)
		}
	}
	out := make([]contracts.PriceBar, 0, len(days))
	for i, d := range days {
		out = append(out, contracts.PriceBar{
			Close: strconv.Itoa(100 + int(id) + i), SessionDay: types.SerializableDate{Time: d},
		})
	}
	return out, nil
}

// globalIDs names every listed stock with id 1..n in list order.
type globalIDs struct{}

func (globalIDs) Close() error { return nil }

func (globalIDs) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	m, err := barsrefresh.DefaultManifest()
	if err != nil {
		return nil, err
	}
	var out []store.InstrumentName
	for i, s := range m.Stocks {
		for _, n := range names {
			if n == s.Symbol {
				out = append(out, store.InstrumentName{InstrumentID: int64(i + 1), Type: "stock", Name: n})
			}
		}
	}
	return out, nil
}

// globalApp is a real-profile report app with a bar store whose last day is last and the stock ids deps.
func globalApp(t *testing.T, last string, ids bool) *app {
	t.Helper()
	a := reportApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	a.profile.Name = "real"
	d, err := time.Parse(dayLayout, last)
	if err != nil {
		t.Fatal(err)
	}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) { return &fakeView{}, nil }
	a.deps.openBar = func(context.Context, string) (bar.BarStore, error) { return globalBars{last: d}, nil }
	a.deps.ext = map[string]any{}
	if ids {
		a.deps.ext[extReportGlobal] = reportGlobalDeps{
			openStockIDs: func(context.Context, string) (stockIDStore, error) { return globalIDs{}, nil },
		}
	}
	return a
}

type globalCell struct {
	Text string `json:"text"`
}

type globalPage struct {
	Blocks map[string]json.RawMessage `json:"blocks"`
}

type globalBlock struct {
	State     string `json:"state"`
	DataTimes string `json:"data_times"`
	Gaps      []struct {
		Reason string `json:"reason"`
	} `json:"gaps"`
	Proxies json.RawMessage `json:"proxies"`
	Sector  json.RawMessage `json:"sector"`
}

type globalProxy struct {
	Symbol  string     `json:"symbol"`
	Day     globalCell `json:"day"`
	FiveDay globalCell `json:"five_day"`
}

// globalProxies decodes the proxies of a built block.
func globalProxies(t *testing.T, b globalBlock) []globalProxy {
	t.Helper()
	var out []globalProxy
	if err := json.Unmarshal(b.Proxies, &out); err != nil {
		t.Fatalf("%v: %s", err, b.Proxies)
	}
	return out
}

// globalRun runs ot page for market, slot and day and returns the whole output and its global_assets block.
func globalRun(t *testing.T, a *app, market, slot, day string) ([]byte, globalBlock) {
	t.Helper()
	var out bytes.Buffer
	a.stdout = &out
	if err := runPage(a, []string{"--name", "report_page", "--market", market, "--slot", slot, "--day", day}); err != nil {
		t.Fatal(err)
	}
	validateReport(t, out.Bytes())
	var p globalPage
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	var b globalBlock
	if err := json.Unmarshal(p.Blocks["global_assets"], &b); err != nil {
		t.Fatal(err)
	}
	return out.Bytes(), b
}

func TestReportUsCloseGlobalFigures(t *testing.T) {
	t.Parallel()
	a := globalApp(t, "2026-11-06", true)
	_, b := globalRun(t, a, "us", "close", "2026-11-06")
	if b.State != "ok" || b.DataTimes != "Closes of 2026-11-06" {
		t.Fatalf("state %s, data_times %q, gaps %v", b.State, b.DataTimes, b.Gaps)
	}
	px := globalProxies(t, b)
	if len(px) != 5 || px[0].Symbol != "SPY" {
		t.Fatalf("proxies %+v", px)
	}
	// SPY is id 1: closes 130 (c0), 129 (c1) and 125 (c5): 1/129 = 0.78 and 5/125 = 4.00.
	if px[0].Day.Text != "+0.78%" || px[0].FiveDay.Text != "+4.00%" {
		t.Errorf("SPY %+v", px[0])
	}
	var sec struct {
		Rows []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(b.Sector, &sec); err != nil || len(sec.Rows) != 11 {
		t.Fatalf("sector %v: %s", err, b.Sector)
	}
}

func TestReportUsCloseShowsFridayOnSaturday(t *testing.T) {
	t.Parallel()
	a := globalApp(t, "2026-11-06", true)
	_, b := globalRun(t, a, "us", "close", "2026-11-07") // Saturday
	if b.State != "ok" || b.DataTimes != "Closes of 2026-11-06" {
		t.Fatalf("state %s, data_times %q, gaps %v", b.State, b.DataTimes, b.Gaps)
	}
	if px := globalProxies(t, b); len(px) != 5 || px[0].Day.Text != "+0.78%" {
		t.Fatalf("proxies %+v", px)
	}
}

func TestReportUsCloseStaleAfterThreeWeekdays(t *testing.T) {
	t.Parallel()
	a := globalApp(t, "2026-11-06", true)
	_, b := globalRun(t, a, "us", "close", "2026-11-12") // four weekdays after Friday
	if b.State != "gap" || len(b.Gaps) != 1 || b.Gaps[0].Reason != "stale_input" {
		t.Fatalf("state %s, gaps %v", b.State, b.Gaps)
	}
}

func TestReportNoStockIDsIsNoReader(t *testing.T) {
	t.Parallel()
	with := globalApp(t, "2026-11-06", true)
	without := globalApp(t, "2026-11-06", false)
	wOut, _ := globalRun(t, with, "us", "close", "2026-11-06")
	nOut, b := globalRun(t, without, "us", "close", "2026-11-06")
	if b.State != "gap" || len(b.Gaps) != 1 || b.Gaps[0].Reason != "no_reader" {
		t.Fatalf("state %s, gaps %v", b.State, b.Gaps)
	}
	var wp, np globalPage
	if err := errors.Join(json.Unmarshal(wOut, &wp), json.Unmarshal(nOut, &np)); err != nil {
		t.Fatal(err)
	}
	for k, v := range wp.Blocks {
		if k != "global_assets" && string(np.Blocks[k]) != string(v) {
			t.Errorf("block %s changed", k)
		}
	}
	// a nil opener in the key is the same gap and no panic
	nilOpener := globalApp(t, "2026-11-06", false)
	nilOpener.deps.ext[extReportGlobal] = reportGlobalDeps{}
	if _, b := globalRun(t, nilOpener, "us", "close", "2026-11-06"); b.State != "gap" {
		t.Fatalf("nil opener: state %s", b.State)
	}
}

func TestReportJpAndOpenKeepNotBuilt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ market, slot string }{{"jp", "close"}, {"us", "open"}} {
		a := globalApp(t, "2026-11-06", true)
		_, b := globalRun(t, a, c.market, c.slot, "2026-11-06")
		if b.State != "gap" || len(b.Gaps) != 1 || b.Gaps[0].Reason != "not_built" {
			t.Errorf("%s %s: state %s, gaps %v", c.market, c.slot, b.State, b.Gaps)
		}
	}
}
