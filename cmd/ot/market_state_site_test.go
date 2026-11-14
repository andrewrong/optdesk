package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/figures"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/marketstate"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const msDay = "2026-11-13"

var msClock = fakeClock{time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)}

// msApp is an app with the real figure functions in its deps, as wire.go sets them.
func msApp(t *testing.T) *app {
	t.Helper()
	a, _ := testApp(t, "")
	stats := gonumstats.New()
	rl := rules.New()
	a.deps.computeFigures = figures.New(stats, own.NewRolling(stats), own.NewSeriesOps(), rl)
	a.deps.computeRules = rl
	a.deps.computeUnits = own.NewUnits()
	return a
}

func msWeekdays(n int) []time.Time {
	var out []time.Time
	end, _ := time.Parse(time.DateOnly, msDay)
	for d := end; len(out) < n; d = d.AddDate(0, 0, -1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			out = append([]time.Time{d}, out...)
		}
	}
	return out
}

// msSeries holds the four series with a wave so the S&P 500 has a drawdown.
type msSeries struct{}

func (msSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for i, d := range msWeekdays(320) {
		v := 100 + float64(i%37)*1.3 + float64(i)/5 + float64(len(id))
		out = append(out, contracts.SeriesPoint{
			SeriesID: id, ObservationDate: types.SerializableDate{Time: d}, RealtimeStart: types.SerializableDate{Time: d},
			Value: &v, Status: "present",
		})
	}
	return out, nil
}

type msIDs map[string]int64

func (m msIDs) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range names {
		if id, ok := m[n]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

type msBars struct{}

func (msBars) PriceBars(_ context.Context, id int64, _ string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	var out []contracts.PriceBar
	for i, d := range msWeekdays(100) {
		day := contracts.Day(d.Format(time.DateOnly))
		if day < from || day > to {
			continue
		}
		out = append(out, contracts.PriceBar{
			InstrumentID: int(id), SessionDay: types.SerializableDate{Time: d},
			Close: strconv.FormatFloat(100+float64(id)*float64(i%11)/3+float64(i)*float64(id)/40, 'f', 2, 64),
		})
	}
	return out, nil
}

func msReaders() siteReaders {
	ids := msIDs{"SPY": 1}
	for i, s := range marketStateSymbols() {
		ids[s.Symbol] = int64(i + 2)
	}
	return siteReaders{
		series: msSeries{}, bars: msBars{},
		ext: map[string]any{siteMarketStateIDsKey: ids},
	}
}

type msPage struct {
	Schema string `json:"schema"`
	Market struct {
		Rows []struct {
			ID     string          `json:"id"`
			Figure json.RawMessage `json:"figure"`
			Window string          `json:"window"`
			Day    json.RawMessage `json:"day"`
		} `json:"rows"`
	} `json:"market"`
	Sectors struct {
		Rows []json.RawMessage `json:"rows"`
	} `json:"sectors"`
	Gaps []struct{ Reason string } `json:"gaps"`
}

func TestMSRouteBuildsRealFigures(t *testing.T) {
	t.Parallel()
	a := msApp(t)
	if got := len(marketStateSymbols()); got != 12 {
		t.Fatalf("manifest gives %d symbols, want 11 funds and SMH", got)
	}
	body, err := buildMarketState(t.Context(), a, msClock, msDay, msReaders())
	if err != nil {
		t.Fatal(err)
	}
	var m msPage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Market.Rows) != 5 || len(m.Sectors.Rows) != 12 {
		t.Fatalf("%d market rows, %d sector rows", len(m.Market.Rows), len(m.Sectors.Rows))
	}
	var drawdown struct{ Text string }
	for _, r := range m.Market.Rows {
		if r.Window == "" || len(r.Day) == 0 || string(r.Day) == "null" {
			t.Errorf("row %s: window %q day %s", r.ID, r.Window, r.Day)
		}
		if r.ID == "sp500_drawdown" {
			if err := json.Unmarshal(r.Figure, &drawdown); err != nil {
				t.Fatal(err)
			}
		}
	}
	sp, _ := msSeries{}.SeriesPoints(t.Context(), "SP500", "", "")
	var s compute.Series
	for _, p := range sp {
		s = append(s, compute.Point{Day: contracts.Day(p.ObservationDate.Format(time.DateOnly)), Value: *p.Value})
	}
	figs, units, rl := a.deps.computeFigures, a.deps.computeUnits, a.deps.computeRules
	res := marketstate.Compute(marketstateapi.Input{Day: msDay, SP500: s, Figures: figs, Units: units, Rules: rl, Params: marketstate.TrialParams()})
	if res.Drawdown.Reason != "" {
		t.Fatalf("drawdown reason %q", res.Drawdown.Reason)
	}
	v, err := strconv.ParseFloat(res.Drawdown.Value, 64)
	if err != nil || v >= 0 {
		t.Fatalf("drawdown %q: the fixture must have one", res.Drawdown.Value)
	}
	if want := fmt.Sprintf("S&P 500 is %.2f%% below", math.Abs(v)); !strings.HasPrefix(drawdown.Text, want) {
		t.Errorf("drawdown text %q, Compute gives %q", drawdown.Text, res.Drawdown.Value)
	}
}

func TestMarketStateNamedForOtPage(t *testing.T) {
	t.Parallel()
	a := msApp(t)
	a.profile.Name = "demo"
	page, closeFn, err := pageNamed(a, marketStateName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	body, err := page.Build(t.Context(), nil, msDay, msClock.Now())
	if err != nil {
		t.Fatalf("build with no reader: %v", err)
	}
	var m msPage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Sectors.Rows) != 12 {
		t.Fatalf("%d sector rows", len(m.Sectors.Rows))
	}
	for _, r := range m.Sectors.Rows {
		if !strings.Contains(string(r), `"no_reader"`) {
			t.Fatalf("sector row without the gap no_reader: %s", r)
		}
	}
}

func TestMarketStateRouteServes(t *testing.T) {
	t.Parallel()
	a := msApp(t)
	f, ok := pageFamilyOfRoute(siteMarketStateRoute)
	if !ok || f.name != marketStateName || f.order != 120 || f.named == nil {
		t.Fatalf("route %q: %+v", siteMarketStateRoute, f)
	}
	h := siteHandler(a, msClock, msDay, msReaders(), echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteMarketStateRoute)
	if rec.Code != 200 {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	var m msPage
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil || m.Schema != marketStateName {
		t.Fatalf("schema %q: %v", m.Schema, err)
	}
}
