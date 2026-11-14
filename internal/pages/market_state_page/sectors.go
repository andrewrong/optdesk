package market_state_page

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	// barsLookback is the calendar days of bars read before the page day: 100 weekday sessions,
	// enough for the 84 closes of the oldest rotation tail point.
	barsLookback = 140
	basisTrades  = "trades"
)

type rank struct {
	At int `json:"at"`
	Of int `json:"of"`
}

type sectorRow struct {
	Symbol   string `json:"symbol"`
	Label    string `json:"label"`
	Gap21    any    `json:"gap_21"`
	Gap63    any    `json:"gap_63"`
	Rank21   any    `json:"rank_21"`
	Rank63   any    `json:"rank_63"`
	Group    string `json:"group"`
	Rotation any    `json:"rotation"`
}

type rotationAxes struct {
	XCaption  string     `json:"x_caption"`
	YCaption  string     `json:"y_caption"`
	Note      string     `json:"note"`
	Quadrants []quadrant `json:"quadrants"`
}

type sectorBlock struct {
	Caption      string       `json:"caption"`
	EndDay       any          `json:"end_day"`
	BarsCaption  string       `json:"bars_caption"`
	RotationAxes rotationAxes `json:"rotation_axes"`
	Rows         []sectorRow  `json:"rows"`
}

// sectorResult is the block and the end day as text ("" when it has none).
type sectorResult struct {
	sectorBlock
	EndDayText string
}

// barsIn holds the closes read from the bar store; ok is false when a reader is missing.
type barsIn struct {
	ok              bool
	bench           blockapi.CloseIn
	funds, industry []blockapi.CloseIn
}

// listed splits the configured symbols into funds and industry funds; SMH is always among the
// industry funds, listed or not.
func (c Config) listed() (funds, industry []Symbol) {
	for _, s := range c.Symbols {
		switch s.Group {
		case GroupSector:
			funds = append(funds, s)
		case GroupIndustry:
			industry = append(industry, s)
		}
	}
	for _, s := range industry {
		if s.Symbol == smhSymbol {
			return funds, industry
		}
	}
	return funds, append(industry, Symbol{Symbol: smhSymbol, Label: smhLabel, Group: GroupIndustry})
}

// readBars reads the trades closes of SPY and the listed funds for barsLookback calendar days
// before d, newest first. Without a bar reader or a stock-id reader nothing is read.
func readBars(ctx context.Context, c Config, d time.Time) (barsIn, error) {
	if c.Bars == nil || c.StockIDs == nil {
		return barsIn{}, nil
	}
	funds, industry := c.listed()
	names := []string{spySymbol}
	for _, s := range append(append([]Symbol{}, funds...), industry...) {
		names = append(names, s.Symbol)
	}
	rows, err := c.StockIDs.StockIDs(ctx, names)
	if err != nil {
		return barsIn{}, fmt.Errorf("market_state_page: read stock ids: %w", err)
	}
	ids := map[string][]int64{}
	for _, r := range rows {
		ids[r.Name] = append(ids[r.Name], r.InstrumentID)
	}
	from := contracts.Day(d.AddDate(0, 0, -barsLookback).Format(dayLayout))
	to := contracts.Day(d.Format(dayLayout))
	closes := func(symbol, label string) (blockapi.CloseIn, error) {
		in := blockapi.CloseIn{Name: label, Symbol: symbol}
		switch len(ids[symbol]) {
		case 0: // no closes: no_data
		case 1:
			bars, err := c.Bars.PriceBars(ctx, ids[symbol][0], basisTrades, from, to)
			if err != nil {
				return in, fmt.Errorf("market_state_page: read bars of %s: %w", symbol, err)
			}
			for i := len(bars) - 1; i >= 0; i-- { // the reader gives ascending days
				in.Closes = append(in.Closes, bars[i].Close)
				in.Days = append(in.Days, contracts.Day(bars[i].SessionDay.Format(dayLayout)))
			}
		default: // two instruments of one name: a close that is no number, so the figures answer missing_input
			in.Closes, in.Days = []string{"?"}, []contracts.Day{to}
		}
		return in, nil
	}
	out := barsIn{ok: true}
	if out.bench, err = closes(spySymbol, spySymbol); err != nil {
		return barsIn{}, err
	}
	for _, s := range funds {
		in, err := closes(s.Symbol, s.Label)
		if err != nil {
			return barsIn{}, err
		}
		out.funds = append(out.funds, in)
	}
	for _, s := range industry {
		in, err := closes(s.Symbol, s.Label)
		if err != nil {
			return barsIn{}, err
		}
		out.industry = append(out.industry, in)
	}
	return out, nil
}

// buildSectors writes the block from the result in the order given: it does not sort or rank.
// live is false without a Func or a reader: every row is then the gap no_reader. With an empty
// EndDay every fund row is a gap and end_day is the Gap no_data.
func buildSectors(c Config, ms marketstateapi.Result, live bool) sectorResult {
	out := sectorResult{sectorBlock: sectorBlock{
		Caption: sectorCaption, BarsCaption: barsCaption,
		RotationAxes: rotationAxes{XCaption: axisX, YCaption: axisY, Note: rotationNote, Quadrants: quadrants()},
		Rows:         []sectorRow{},
	}}
	funds, industry := c.listed()
	labels := map[string]string{}
	for _, s := range append(append([]Symbol{}, funds...), industry...) {
		labels[s.Symbol] = s.Label
	}
	reason, why := "", ""
	switch {
	case !live:
		reason, why = reasonNoReader, gapNoReader
		out.EndDay = mkGap(reasonNoReader, "end day: "+gapNoReader)
	case ms.EndDay == "":
		reason, why = reasonNoData, gapNoData
		out.EndDay = mkGap(reasonNoData, "end day: "+gapNoClose)
	default:
		out.EndDay = string(ms.EndDay)
		out.EndDayText = string(ms.EndDay)
	}
	if reason != "" {
		for _, s := range funds {
			out.Rows = append(out.Rows, gapRow(s, reason, why))
		}
		out.Rows = append(out.Rows, gapRow(smh(industry), reason, why))
		return out
	}
	haveSMH := false
	for _, f := range ms.Funds {
		out.Rows = append(out.Rows, fundRow(f, GroupSector, labels))
	}
	for _, f := range ms.Industry {
		haveSMH = haveSMH || f.Symbol == smhSymbol
		out.Rows = append(out.Rows, fundRow(f, GroupIndustry, labels))
	}
	if !haveSMH {
		out.Rows = append(out.Rows, gapRow(smh(industry), reasonNoData, gapNoData))
	}
	return out
}

func smh(industry []Symbol) Symbol {
	for _, s := range industry {
		if s.Symbol == smhSymbol {
			return s
		}
	}
	return Symbol{Symbol: smhSymbol, Label: smhLabel, Group: GroupIndustry}
}

// gapRow is a row whose every cell is the Gap reason.
func gapRow(s Symbol, reason, why string) sectorRow {
	g := func(what string) gap { return mkGap(reason, what+": "+why) }
	return sectorRow{Symbol: s.Symbol, Label: s.Label,
		Gap21: g("21-session gap"), Gap63: g("63-session gap"),
		Rank21: g("21-session rank"), Rank63: g("63-session rank"),
		Group: s.Group, Rotation: g("rotation")}
}

func fundRow(f marketstateapi.Fund, group string, labels map[string]string) sectorRow {
	label := f.Name
	if label == "" {
		label = labels[f.Symbol]
	}
	if label == "" {
		label = f.Symbol
	}
	return sectorRow{Symbol: f.Symbol, Label: label,
		Gap21:  gapFigure(f.Gap21.Value, f.Gap21.Reason, "21-session gap"),
		Gap63:  gapFigure(f.Gap63.Value, f.Gap63.Reason, "63-session gap"),
		Rank21: rankCell(f.Rank21, group, "21-session rank"),
		Rank63: rankCell(f.Rank63, group, "63-session rank"),
		Group:  group, Rotation: rotationCell(f.Rotation)}
}
