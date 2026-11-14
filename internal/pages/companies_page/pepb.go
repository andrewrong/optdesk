package companies_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/company-pepb.md#Decisions (Decisions 7 and 8). This file copies what the
// pepbapi.BuildFunc gave into the page block; every figure is the text it wrote.

const (
	pepbBasisNote = "Close times the filed share count over trailing-twelve-month net income (PE) or equity (PB), " +
		"each as filed on or before the day. History, not a buy price."
	pepbForward = "forward PE: no estimates source"
	pepbJPText  = "no filings source for Japanese names"
	// pepbSpanFrom is before any stored bar: bars are read for the whole stored span.
	pepbSpanFrom = contracts.Day("1900-01-01")
)

type pepbBlock struct {
	BasisNote string    `json:"basis_note"`
	Rows      []pepbRow `json:"rows"`
}

type pepbRow struct {
	Symbol  string `json:"symbol"`
	PE      any    `json:"pe"`
	PB      any    `json:"pb"`
	Forward gap    `json:"forward"`
}

type pepbFigure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type pepbAt struct {
	Percentile int        `json:"percentile"`
	Ratio      pepbFigure `json:"ratio"`
	Price      pepbFigure `json:"price"`
	Text       string     `json:"text"`
}

type pepbPoint struct {
	Day   contracts.Day `json:"day"`
	Value float64       `json:"value"`
	Text  string        `json:"text"`
}

type pepbLine struct {
	Text         string        `json:"text"`
	Ratio        pepbFigure    `json:"ratio"`
	Day          contracts.Day `json:"day"`
	BasisEnd     contracts.Day `json:"basis_end"`
	Percentile   any           `json:"percentile"`
	AtPercentile []pepbAt      `json:"at_percentile"`
	Shares       string        `json:"shares"`
	SharesFiled  contracts.Day `json:"shares_filed"`
	WindowDays   int           `json:"window_days"`
	WindowFrom   contracts.Day `json:"window_from"`
	History      []pepbPoint   `json:"history"`
}

// pepbOn reports whether the block is built: both inputs are set.
func (c Config) pepbOn() bool { return c.Pepb != nil && c.Bars != nil }

// pepbBlockOf builds the pepb property: one row per subject, or a Gap while Facts is not set.
func (c Config) pepbBlockOf(ctx context.Context, today contracts.Day) (any, error) {
	if c.Facts == nil {
		return gap{Reason: reasonNoReader, Text: "pepb: facts reader not set"}, nil
	}
	blk := pepbBlock{BasisNote: pepbBasisNote, Rows: make([]pepbRow, 0, len(c.Subjects))}
	for _, s := range c.Subjects {
		row := pepbRow{Symbol: s.Symbol, Forward: gap{Reason: reasonNoData, Text: pepbForward}}
		source, ok := factSource(s.Issuer)
		switch {
		case !ok:
			g := gap{Reason: reasonNoData, Text: noIssuerText}
			row.PE, row.PB = g, g
		case source == "edinet":
			g := gap{Reason: "jp_slice_missing", Text: pepbJPText}
			row.PE, row.PB = g, g
		default:
			res, bars, err := c.pepbResult(ctx, s, source, today)
			if err != nil {
				return nil, err
			}
			if !bars {
				g := gap{Reason: reasonNoData, Text: "no stored bars"}
				row.PE, row.PB = g, g
				break
			}
			row.PE, row.PB = pepbCell(res.PE, "PE"), pepbCell(res.PB, "PB")
		}
		blk.Rows = append(blk.Rows, row)
	}
	return blk, nil
}

// pepbResult reads the facts and the bars of one subject and calls the function. bars is false
// when none are stored.
func (c Config) pepbResult(ctx context.Context, s Subject, source string, today contracts.Day) (pepbapi.Result, bool, error) {
	facts, err := c.Facts.FactsFiledBy(ctx, source, s.Issuer, today)
	if err != nil {
		return pepbapi.Result{}, false, fmt.Errorf("companies_page: pepb facts of %s: %w", s.Symbol, err)
	}
	bars, err := c.Bars(ctx, s.Symbol, pepbSpanFrom, today)
	if err != nil {
		return pepbapi.Result{}, false, fmt.Errorf("companies_page: pepb bars of %s: %w", s.Symbol, err)
	}
	if len(bars) == 0 {
		return pepbapi.Result{}, false, nil
	}
	return c.Pepb(pepbapi.Input{Facts: facts, Bars: bars, Today: today}), true, nil
}

// pepbCell is a line for a reading with a ratio, else the Gap that says why not.
func pepbCell(r pepbapi.Reading, label string) any {
	if r.State != pepbapi.StateOK {
		reason := "missing_input"
		if r.Reason == "price_stale" {
			reason = "stale_input"
		}
		text := r.Text
		if text == "" {
			text = label + ": no figure"
		}
		if r.Reason != "" && !strings.Contains(text, r.Reason) {
			text += " (" + r.Reason + ")"
		}
		return gap{Reason: reason, Text: clip(text)}
	}
	line := pepbLine{
		Text: r.Text, Ratio: pepbFigure{Value: json.Number(r.Ratio), Text: r.Ratio, Unit: "ratio"},
		Day: r.Day, BasisEnd: r.BasisEnd, Shares: r.Shares, SharesFiled: r.SharesFiled,
		WindowDays: r.WindowDays, WindowFrom: r.WindowFrom,
		AtPercentile: make([]pepbAt, 0, len(r.At)), History: make([]pepbPoint, 0, len(r.History)),
	}
	if r.PercentileReason != "" || r.Percentile == "" {
		line.Percentile = gap{Reason: "missing_input", Text: clip(fmt.Sprintf("no percentile: %s, %d days since %s", r.PercentileReason, r.WindowDays, r.WindowFrom))}
	} else {
		line.Percentile = pepbFigure{Value: json.Number(r.Percentile), Text: r.Percentile, Unit: "percentile"}
	}
	for _, a := range r.At {
		line.AtPercentile = append(line.AtPercentile, pepbAt{
			Percentile: a.Percentile, Text: a.Text,
			Ratio: pepbFigure{Value: json.Number(a.Ratio), Text: a.Ratio, Unit: "ratio"},
			Price: pepbFigure{Value: json.Number(a.Price), Text: a.Price, Unit: "price"},
		})
	}
	for _, p := range r.History {
		line.History = append(line.History, pepbPoint{Day: p.Day, Value: p.Value, Text: p.Text})
	}
	return line
}

// clip keeps a Gap text within the 200 characters of the schema.
func clip(s string) string {
	if r := []rune(s); len(r) > 200 {
		return string(r[:200])
	}
	return s
}
