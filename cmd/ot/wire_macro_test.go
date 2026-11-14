package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/ease"
	"github.com/andrewrong/optdesk/internal/compute/liquidity"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// liquidityIDs are the fourteen series of the liquidity figures.
var liquidityIDs = []string{
	"WRESBAL", "RRPONTSYD", "WALCL", "WDTGAL", "TLAACBW027SBOG", "WLCFLPCL", "SOFR", "IORB", "SOFR99",
	"BAMLH0A0HYM2", "BAMLC0A0CM", "BAA10Y", "NFCI", "STLFSI4",
}

// liquiditySeries is a daily series of 200 days ending 2026-11-04 whose level rises by step a day.
func liquiditySeries(base, step float64) compute.Series {
	end, _ := time.Parse(dayLayout, "2026-11-04")
	var s compute.Series
	for i := 0; i < 200; i++ {
		d := end.AddDate(0, 0, i-199)
		s = append(s, compute.Point{Day: contracts.Day(d.Format(dayLayout)), Value: base + step*float64(i)})
	}
	return s
}

// liquidityFake holds the fourteen series; level is the base of each series.
type liquidityFake map[string]compute.Series

func newLiquidityFake() liquidityFake {
	f := liquidityFake{}
	for _, id := range liquidityIDs {
		f[id] = liquiditySeries(2, 0.01)
	}
	f["WRESBAL"] = liquiditySeries(3.0e12, 1e9)
	f["WALCL"] = liquiditySeries(7.0e12, 1e9)
	f["WDTGAL"] = liquiditySeries(0.8e12, 1e8)
	f["RRPONTSYD"] = liquiditySeries(0.3e12, 1e8)
	return f
}

func (f liquidityFake) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, p := range f[id] {
		d, _ := time.Parse(dayLayout, string(p.Day))
		v := p.Value
		out = append(out, contracts.SeriesPoint{
			SeriesID: id, ObservationDate: types.SerializableDate{Time: d},
			RealtimeStart: types.SerializableDate{Time: d}, Value: &v,
		})
	}
	return out, nil
}

// input is the liquidity input the page builds from the fake, for the day 2026-11-05.
func (f liquidityFake) input() liquidityapi.Input {
	return liquidityapi.Input{
		Day:     "2026-11-05",
		WRESBAL: f["WRESBAL"], RRPONTSYD: f["RRPONTSYD"], WALCL: f["WALCL"], WDTGAL: f["WDTGAL"],
		TLAACBW027SBOG: f["TLAACBW027SBOG"], WLCFLPCL: f["WLCFLPCL"],
		SOFR: f["SOFR"], IORB: f["IORB"], SOFR99: f["SOFR99"],
		BAMLH0A0HYM2: f["BAMLH0A0HYM2"], BAMLC0A0CM: f["BAMLC0A0CM"], BAA10Y: f["BAA10Y"],
		NFCI: f["NFCI"], STLFSI4: f["STLFSI4"],
		Net: ease.NetLiquidity, Params: liquidity.TrialParams(),
	}
}

// tnText is a level of at least one trillion as the page writes it: $x.xxtn.
func tnText(t *testing.T, v string) string {
	t.Helper()
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 1e12 {
		t.Fatalf("figure %q is not a level of a trillion or more: %v", v, err)
	}
	return fmt.Sprintf("$%.2ftn", f/1e12)
}

func TestMacroPageWiresRealLiquidity(t *testing.T) {
	t.Parallel()
	p := newMacroPage(time.UTC, "dev", nil)
	if p.Meta.Schema != "page.macro_page.v2" {
		t.Fatalf("schema %q", p.Meta.Schema)
	}
	f := newLiquidityFake()
	b, err := p.Build(t.Context(), f, "2026-11-05", time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Version   int `json:"schema_version"`
		Liquidity struct {
			Lines []struct {
				Key      string `json:"key"`
				Sentence any    `json:"sentence"`
			} `json:"lines"`
			Taps []struct {
				Key  string `json:"key"`
				Rows []struct {
					Text any `json:"text"`
				} `json:"rows"`
			} `json:"taps"`
		} `json:"liquidity"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Version != 2 || len(m.Liquidity.Lines) != 4 {
		t.Fatalf("version %d, %d lines: %s", m.Version, len(m.Liquidity.Lines), b)
	}
	in := f.input()
	res := liquidity.Compute(in)
	if res.Reserves.Reason != "" {
		t.Fatalf("reserves figure is a gap: %+v", res.Reserves)
	}
	if s, _ := m.Liquidity.Lines[0].Sentence.(string); !strings.Contains(s, "Bank reserves were "+tnText(t, res.Reserves.Value)) {
		t.Errorf("reserves sentence %v lacks the figure %s", m.Liquidity.Lines[0].Sentence, res.Reserves.Value)
	}
	net := ease.NetLiquidity(in.WALCL, in.WDTGAL, in.RRPONTSYD, in.Day)
	if net.Reason != "" {
		t.Fatalf("net liquidity is a gap: %+v", net)
	}
	for _, tp := range m.Liquidity.Taps {
		if tp.Key != "net_liquidity" {
			continue
		}
		if s, _ := tp.Rows[0].Text.(string); !strings.HasPrefix(s, tnText(t, net.Value)+" on Wednesday ") {
			t.Errorf("net liquidity row %v, want the figure %s", tp.Rows[0].Text, net.Value)
		}
		return
	}
	t.Fatal("no net_liquidity tap")
}
