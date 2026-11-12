package home_companies

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	barBasis        = "trades"
	barLookbackDays = 14
	dayChangeName   = "day change"
	fiveChangeName  = "5-day change"
)

// changeCells reads the stored daily closes of the row's instrument for the 14 days up to day and
// returns the day change and the five-day change cells. The percent comes from Figures().Moves; this
// file computes nothing. No reader or no Figures gives a no_reader gap, no instrument a no_data gap
// and no PriceBars call; a reader error is returned.
func (c Config) changeCells(ctx context.Context, r Row, day time.Time) (dayCell, fiveCell any, err error) {
	if c.Bars == nil || c.Figures == nil {
		return mkGap("no_reader", dayChangeName+": "+noReaderText), mkGap("no_reader", fiveChangeName+": "+noReaderText), nil
	}
	if r.InstrumentID == 0 {
		return mkGap("no_data", dayChangeName+": no instrument"), mkGap("no_data", fiveChangeName+": no instrument"), nil
	}
	from := contracts.Day(day.AddDate(0, 0, -barLookbackDays).Format(dayLayout))
	bars, err := c.Bars.PriceBars(ctx, r.InstrumentID, barBasis, from, contracts.Day(day.Format(dayLayout)))
	if err != nil {
		return nil, nil, fmt.Errorf("home_companies: read price bars of %s: %w", r.Symbol, err)
	}
	in := blockapi.CloseIn{Name: r.Symbol, Symbol: r.Symbol}
	for i := len(bars) - 1; i >= 0; i-- {
		in.Closes = append(in.Closes, bars[i].Close)
		in.Days = append(in.Days, contracts.Day(bars[i].SessionDay.Format(dayLayout)))
	}
	lines := c.Figures().Moves([]blockapi.CloseIn{in})
	if len(lines) == 0 {
		return mkGap("no_data", dayChangeName+": no figure"), mkGap("no_data", fiveChangeName+": no figure"), nil
	}
	return changeCell(dayChangeName, lines[0].D1), changeCell(fiveChangeName, lines[0].D5), nil
}

// changeCell copies the compute text of a value as a percent figure, or names the gap reason.
func changeCell(name string, v compute.FigureValue) any {
	if v.Gap != "" || v.Value == "" {
		return mkGap("no_data", name+": "+strings.ReplaceAll(v.Gap, "_", " "))
	}
	text := v.Value
	if !strings.HasPrefix(text, "-") && !strings.HasPrefix(text, "+") && !isZero(text) {
		text = "+" + text
	}
	return figure{Kind: "value", Value: json.Number(v.Value), Text: text + "%", Unit: "percent", Tone: "none"}
}

// isZero reports whether the decimal text has only zero digits.
func isZero(s string) bool {
	return strings.Trim(s, "0.") == ""
}
