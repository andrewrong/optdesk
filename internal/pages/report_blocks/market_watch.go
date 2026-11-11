package report_blocks

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	seriesLookback = 40 // calendar days of the series read: six closes with holidays
	staleDays      = 5  // calendar days, about 3 session days with a weekend
)

type indexRow struct {
	Name       any    `json:"name"`
	Close      any    `json:"close"`
	Day        any    `json:"day"`
	FiveDay    any    `json:"five_day"`
	SourceText string `json:"source_text"`
}

type indicesJSON struct {
	Columns []string   `json:"columns"`
	Rows    []indexRow `json:"rows"`
}

type watchBlock struct {
	head
	Indices   any `json:"indices"`
	Companies any `json:"companies"`
}

// MarketWatch is the market_watch block: the indices row of the S&P 500 from the stored FRED series
// SP500 (the day and five-day change come from compute), and companies as the Gap missing_quote.
// A series with no point is the Gap no_data, one older than 5 calendar days stale_input.
func MarketWatch(c Config) BlockFunc {
	return func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error) {
		if err := c.ready(); err != nil {
			return nil, err
		}
		d, err := parseDay(day, generatedAt)
		if err != nil {
			return nil, err
		}
		if c.Series == nil || c.Figures == nil {
			return marshal(watchBlock{head: gapHead(keyWatch, titleWatch, subWatch, sayWatchGap, "no_reader"),
				Indices: gapOf("no_reader"), Companies: gapOf("no_reader")})
		}
		pts, err := c.Series.SeriesPoints(ctx, seriesID, contracts.Day(d.AddDate(0, 0, -seriesLookback).Format(dayLayout)), day)
		if err != nil {
			return nil, fmt.Errorf("report_blocks: read series: %w", err)
		}
		in := blockapi.CloseIn{Name: labelSP500, Symbol: seriesID}
		slices.SortFunc(pts, func(a, b contracts.SeriesPoint) int { return b.ObservationDate.Compare(a.ObservationDate.Time) })
		var newest time.Time
		for _, p := range pts {
			if p.Value == nil {
				continue
			}
			in.Closes = append(in.Closes, strconv.FormatFloat(*p.Value, 'f', -1, 64))
			in.Days = append(in.Days, contracts.Day(p.ObservationDate.Format(dayLayout)))
			if newest.IsZero() {
				newest = p.ObservationDate.Time
			}
		}
		h := newHead(keyWatch, titleWatch, subWatch, sayWatch, noDataTimes)
		h.addGap(gapJSON{Reason: "missing_quote", Text: "Focus companies: " + textMissQuote})
		out := watchBlock{Companies: gapOf("missing_quote")}
		switch {
		case len(in.Closes) == 0:
			out.Indices = gapOf("no_data")
			h.Gaps = append(h.Gaps, gapJSON{Reason: "no_data", Text: textNoSeries})
			h.State = "gap"
		case d.Sub(newest) > staleDays*24*time.Hour:
			out.Indices = gapOf("stale_input")
			h.Gaps = append(h.Gaps, gapJSON{Reason: "stale_input", Text: textStaleSeria})
			h.State = "gap"
		default:
			line := c.Figures().Moves([]blockapi.CloseIn{in})[0]
			h.DataTimes = dataTimesSer + string(line.DataDay)
			out.Indices = indicesJSON{Columns: columnsIndices, Rows: []indexRow{{
				Name: textCell(labelSP500), SourceText: srcSP500,
				Close:   cellJSON{Text: in.Closes[0], Tone: "none", Value: number(in.Closes[0]), Unit: "index_points"},
				Day:     figureCell(line.D1, "", "percent", true),
				FiveDay: figureCell(line.D5, "", "percent", true),
			}}}
			for _, f := range []string{line.D1.Gap, line.D5.Gap} {
				if f != "" {
					h.addGap(gapOf(f))
				}
			}
		}
		out.head = h
		return marshal(out)
	}
}
