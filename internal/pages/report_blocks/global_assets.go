package report_blocks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	barsLookback = 50 // calendar days of bars read: 21 closes with holidays
	basisTrades  = "trades"
	groupProxy   = "proxy"
	groupSector  = "sector"
)

// Asset is one listed symbol: its label and its group, proxy or sector.
type Asset struct{ Symbol, Label, Group string }

// StockIDReader reads the instruments of type stock whose name is in names, ordered by instrument id.
type StockIDReader interface {
	StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
}

type proxyRow struct {
	Label   string `json:"label"`
	Symbol  string `json:"symbol"`
	Day     any    `json:"day"`
	FiveDay any    `json:"five_day"`
}

type sectorRow struct {
	Sector cellJSON `json:"sector"`
	ETF    cellJSON `json:"etf"`
	D1     any      `json:"d1"`
	D5     any      `json:"d5"`
	D20    any      `json:"d20"`
}

type sectorJSON struct {
	Heading string      `json:"heading"`
	Columns []string    `json:"columns"`
	Rows    []sectorRow `json:"rows"`
}

type rotationJSON struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type globalBlock struct {
	head
	Proxies   any           `json:"proxies"`
	Sector    any           `json:"sector"`
	Rotation  *rotationJSON `json:"rotation,omitempty"`
	ChartAria string        `json:"chart_aria"`
}

// GlobalAssets is the global_assets block: the five proxy assets with their day and five-day change and the sector
// funds with their excess over SPY, from the stored daily trades bars, through blockapi.GlobalFigures. A nil reader,
// constructor or an empty asset list is the gap no_reader; a reader error is an error of the call; a symbol the
// reader does not know is no_data in its cells, two instruments of one name missing_input.
func GlobalAssets(c Config) BlockFunc {
	return func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error) {
		if err := c.ready(); err != nil {
			return nil, err
		}
		d, err := parseDay(day, generatedAt)
		if err != nil {
			return nil, err
		}
		if c.Bars == nil || c.StockIDs == nil || c.Global == nil || len(c.Assets) == 0 {
			return marshal(globalBlock{head: gapHead(keyGlobal, titleGlobal, subGlobal, sayGlobalGap, "no_reader"),
				Proxies: gapOf("no_reader"), Sector: gapOf("no_reader"), ChartAria: chartAriaGlobal})
		}
		series, err := c.globalSeries(ctx, d)
		if err != nil {
			return nil, err
		}
		in := blockapi.GlobalIn{Day: day, Benchmark: series[benchmarkSymbol]}
		var proxies []Asset
		for _, a := range c.Assets {
			switch a.Group {
			case groupProxy:
				proxies = append(proxies, a)
				in.Proxies = append(in.Proxies, series[a.Symbol])
			case groupSector:
				in.Sectors = append(in.Sectors, series[a.Symbol])
			}
		}
		return marshal(c.globalBlock(c.Global().Global(in), proxies))
	}
}

// globalSeries reads the stored bars of every listed symbol and of SPY once each, as newest-first closes.
func (c Config) globalSeries(ctx context.Context, d time.Time) (map[string]blockapi.CloseIn, error) {
	labels := map[string]string{benchmarkSymbol: benchmarkSymbol}
	names := []string{benchmarkSymbol}
	for _, a := range c.Assets {
		if _, ok := labels[a.Symbol]; !ok {
			names = append(names, a.Symbol)
		}
		labels[a.Symbol] = a.Label
	}
	rows, err := c.StockIDs.StockIDs(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("report_blocks: read stock ids: %w", err)
	}
	ids := map[string][]int64{}
	for _, r := range rows {
		ids[r.Name] = append(ids[r.Name], r.InstrumentID)
	}
	from := contracts.Day(d.AddDate(0, 0, -barsLookback).Format(dayLayout))
	to := contracts.Day(d.Format(dayLayout))
	out := make(map[string]blockapi.CloseIn, len(names))
	for _, n := range names {
		in := blockapi.CloseIn{Name: labels[n], Symbol: n}
		switch len(ids[n]) {
		case 0: // no closes: no_data
		case 1:
			bars, err := c.Bars.PriceBars(ctx, ids[n][0], basisTrades, from, to)
			if err != nil {
				return nil, fmt.Errorf("report_blocks: read bars of %s: %w", n, err)
			}
			for i := len(bars) - 1; i >= 0; i-- { // the reader gives ascending days
				in.Closes = append(in.Closes, bars[i].Close)
				in.Days = append(in.Days, contracts.Day(bars[i].SessionDay.Format(dayLayout)))
			}
		default: // two instruments of one name: one close that is no number, so the figures answer missing_input
			in.Closes, in.Days = []string{"?"}, []contracts.Day{contracts.Day(to)}
		}
		out[n] = in
	}
	return out, nil
}

func (c Config) globalBlock(res blockapi.GlobalResult, proxies []Asset) globalBlock {
	times := noDataTimes
	if res.DataDay != "" {
		times = dataTimesGlobal + string(res.DataDay)
	}
	h := newHead(keyGlobal, titleGlobal, subGlobal, sayGlobal, times)
	cells, gaps := 0, 0
	note := func(v any) any {
		cells++
		if g, ok := v.(gapJSON); ok {
			gaps++
			h.addGap(g)
		}
		return v
	}
	out := globalBlock{ChartAria: chartAriaGlobal}
	rowsP := make([]proxyRow, 0, len(res.Proxies))
	for i, m := range res.Proxies {
		if i >= len(proxies) {
			break
		}
		rowsP = append(rowsP, proxyRow{Label: proxies[i].Label, Symbol: proxies[i].Symbol,
			Day: note(figureCell(m.D1, "", "percent", true)), FiveDay: note(figureCell(m.D5, "", "percent", true))})
	}
	rowsS := make([]sectorRow, 0, len(res.Sectors))
	for _, s := range res.Sectors {
		rowsS = append(rowsS, sectorRow{Sector: textCell(s.Name), ETF: textCell(s.Symbol),
			D1: note(pointsCell(s.D1)), D5: note(pointsCell(s.D5)), D20: note(pointsCell(s.D20))})
	}
	if cells == 0 || gaps == cells {
		h.State = "gap"
	}
	if r := res.Rotation; r != nil {
		detail := rotationLeaders + strings.Join(r.Leaders5, ", ") + rotationLaggard + r.Laggard + " " + signed(r.LaggardD5) + unitPoints +
			rotationLeader20 + r.Leader20
		out.Rotation = &rotationJSON{Title: titleRotation, Detail: detail}
	}
	out.head, out.Proxies = h, rowsP
	out.Sector = sectorJSON{Heading: headingSector, Columns: columnsSector, Rows: rowsS}
	return out
}

// pointsCell is the sector cell of a figure in percentage points, or its Gap.
func pointsCell(f compute.FigureValue) any {
	if f.Gap != "" {
		return gapOf(f.Gap)
	}
	return cellJSON{Text: signed(f.Value) + unitPoints, Tone: tone(f.Value), Value: number(f.Value), Unit: "percentage_points"}
}
