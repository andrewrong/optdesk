package market_state_page

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/marketstate"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeIDs struct{ ids map[string]int64 }

func (f fakeIDs) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range names {
		if id, ok := f.ids[n]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

// fakeBars filters its bars by the days asked, ascending, as the real reader does.
type fakeBars struct {
	bars map[int64][]contracts.PriceBar
}

func (f *fakeBars) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	if basis != "trades" {
		return nil, fmt.Errorf("basis %q", basis)
	}
	var out []contracts.PriceBar
	for _, b := range f.bars[id] {
		day := b.SessionDay.Format(dayLayout)
		if day >= string(from) && day <= string(to) {
			out = append(out, b)
		}
	}
	return out, nil
}

// weekdays are the weekdays from..to inclusive, ascending.
func weekdays(from, to string) []string {
	f, _ := time.Parse(dayLayout, from)
	e, _ := time.Parse(dayLayout, to)
	var out []string
	for d := f; !d.After(e); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			out = append(out, d.Format(dayLayout))
		}
	}
	return out
}

// realConfig computes with marketstate on bars of exactly 140 calendar days before testDay; a
// symbol in sessions has only its newest n sessions.
func realConfig(t *testing.T, sessions map[string]int) Config {
	t.Helper()
	all := weekdays("2026-06-26", testDay) // testDay minus 140 days, a Friday, to testDay
	ids, bars := map[string]int64{}, map[int64][]contracts.PriceBar{}
	for k, sym := range []string{"SPY", "XLK", "XLE", "XLF", "SMH"} {
		id := int64(k + 1)
		ids[sym] = id
		days := all
		if n, ok := sessions[sym]; ok {
			days = all[len(all)-n:]
		}
		for i, d := range days {
			step := 0.2 + 0.07*float64(k)*float64((i%5)-2)/2 + 0.05*float64(k)
			bars[id] = append(bars[id], contracts.PriceBar{InstrumentID: int(id), Basis: "trades",
				SessionDay: sdate(d), Close: fmt.Sprintf("%.2f", 100+float64(i)*step)})
		}
	}
	c := fullConfig(t, sampleResult())
	c.Market = marketstate.Compute
	c.MarketParams = marketstate.TrialParams()
	c.Bars = &fakeBars{bars: bars}
	c.StockIDs = fakeIDs{ids: ids}
	return c
}

func rotationOf(t *testing.T, m map[string]any, symbol string) any {
	t.Helper()
	for _, r := range sectorRows(m) {
		if obj(r)["symbol"] == symbol {
			return obj(r)["rotation"]
		}
	}
	t.Fatalf("no row %s", symbol)
	return nil
}

func TestRotationCopiedFromResult(t *testing.T) {
	t.Parallel()
	m := build(t, fullConfig(t, sampleResult()))
	rot := list(rotationOf(t, m, "XLK"))
	if len(rot) != 2 {
		t.Fatalf("rotation = %v", rot)
	}
	first, second := obj(rot[0]), obj(rot[1])
	if first["day"] != "2026-11-06" || first["x"].(interface{ String() string }).String() != "4.00" ||
		first["y"].(interface{ String() string }).String() != "1.30" || first["text"] != "2026-11-06: 63-session +4.00, pace +1.30" {
		t.Errorf("first = %v", first)
	}
	if second["day"] != "2026-11-13" || second["text"] != "2026-11-13: 63-session +5.00, pace -0.17" {
		t.Errorf("second = %v", second)
	}
	res := sampleResult()
	res.Funds[1].Rotation = nil
	g := rotationOf(t, build(t, fullConfig(t, res)), "XLE")
	if !isGap(g) || obj(g)["reason"] != "missing_input" {
		t.Errorf("empty rotation = %v", g)
	}
}

func TestRotationTailNotCarried(t *testing.T) {
	t.Parallel()
	// the result holds today's spot and the 10-session one only: the page writes those two, never a
	// point for the sessions between
	res := sampleResult()
	res.Funds[0].Rotation = []marketstateapi.Spot{spot("2026-10-30", "1", "1"), spot("2026-11-13", "5", "1")}
	rot := list(rotationOf(t, build(t, fullConfig(t, res)), "XLK"))
	if len(rot) != 2 || obj(rot[0])["day"] != "2026-10-30" || obj(rot[1])["day"] != "2026-11-13" {
		t.Errorf("rotation = %v", rot)
	}
}

func TestRotationOldestTail140Days(t *testing.T) {
	t.Parallel()
	m := build(t, realConfig(t, nil))
	all := weekdays("2026-06-26", testDay)
	if len(all) < 84 {
		t.Fatalf("%d sessions", len(all))
	}
	oldest := all[len(all)-1-20]
	for _, s := range []string{"XLK", "XLE", "XLF", "SMH"} {
		rot, ok := rotationOf(t, m, s).([]any)
		if !ok || len(rot) != 5 {
			t.Fatalf("%s rotation = %v", s, rotationOf(t, m, s))
		}
		if obj(rot[0])["day"] != oldest || obj(rot[4])["day"] != testDay {
			t.Errorf("%s: days %v .. %v, want %s .. %s", s, obj(rot[0])["day"], obj(rot[4])["day"], oldest, testDay)
		}
	}
}

func TestRotationShortHistory(t *testing.T) {
	t.Parallel()
	m := build(t, realConfig(t, map[string]int{"XLE": 80, "XLF": 60}))
	if rot := list(rotationOf(t, m, "XLE")); len(rot) != 4 {
		t.Errorf("80 sessions: %d points, want 4 (the oldest tail point dropped)", len(rot))
	}
	g := rotationOf(t, m, "XLF")
	if !isGap(g) || obj(g)["reason"] != "missing_input" {
		t.Errorf("60 sessions: rotation = %v", g)
	}
	if rot := list(rotationOf(t, m, "XLK")); len(rot) != 5 {
		t.Errorf("full history: %d points", len(rot))
	}
}

func TestRotationPointCarriesDay(t *testing.T) {
	t.Parallel()
	m := build(t, realConfig(t, nil))
	for _, r := range sectorRows(m) {
		for _, p := range list(obj(r)["rotation"]) {
			pt := obj(p)
			if !strings.HasPrefix(pt["text"].(string), pt["day"].(string)+":") {
				t.Errorf("%v: text %q does not start with its day %v", obj(r)["symbol"], pt["text"], pt["day"])
			}
		}
	}
	note := obj(obj(m["sectors"])["rotation_axes"])["note"].(string)
	for _, n := range []string{"21", "63", "5,", "10,", "15 and 20"} {
		if !strings.Contains(note, n) {
			t.Errorf("note lacks %q: %s", n, note)
		}
	}
}
