package market_state_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	figIvRv20      = "market.iv_rv20"
	subjectKind    = "underlying"
	ivRvMaxAgeDays = 5 // the age limit of market.iv_rv20, its rule's max_age_days
	unitVolPoints  = "vol_points"
)

var volIndexes = []struct{ id, name string }{{"spx", "SPX"}, {"ndx", "NDX"}, {"rut", "RUT"}}

type volGapRow struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Figure  any    `json:"figure"`
	DayText string `json:"day_text"`
}

type volGap struct {
	Title string      `json:"title"`
	Hint  string      `json:"hint"`
	Rows  []volGapRow `json:"rows"`
}

// buildVolGap reads the stored market.iv_rv20 of SPX, NDX and RUT: the newest ok row with a value of
// the five days up to d, else the newest row of any state. Nothing is computed; the stored fraction is
// shifted by two places and rounded to one decimal as decimal text.
func buildVolGap(ctx context.Context, c Config, d time.Time) (*volGap, error) {
	vg := &volGap{Title: volGapTitle, Hint: volGapHint}
	for _, ix := range volIndexes {
		row := volGapRow{ID: ix.id, Label: ix.name}
		if c.Readings == nil {
			g := mkGap(reasonNoReader, ix.name+" "+volGapNoReader)
			row.Figure, row.DayText = g, g.Text
		} else {
			rec, err := newestIvRv(ctx, c, ix.name, d)
			if err != nil {
				return nil, err
			}
			row.Figure, row.DayText = ivRvFigure(ix.name, rec, d)
		}
		vg.Rows = append(vg.Rows, row)
	}
	return vg, nil
}

func newestIvRv(ctx context.Context, c Config, name string, d time.Time) (*contracts.ReadingRecord, error) {
	from := contracts.Day(d.AddDate(0, 0, -5).Format(dayLayout))
	rows, err := c.Readings(ctx, figIvRv20, subjectKind, name, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("market_state_page: read %s %s: %w", figIvRv20, name, err)
	}
	pick := func(keep func(*contracts.ReadingRecord) bool) *contracts.ReadingRecord {
		var best *contracts.ReadingRecord
		for i := range rows {
			r := &rows[i]
			if keep != nil && !keep(r) {
				continue
			}
			if best == nil || r.Day.After(best.Day.Time) || (r.Day.Equal(best.Day.Time) && r.Rev > best.Rev) {
				best = r
			}
		}
		return best
	}
	if best := pick(func(r *contracts.ReadingRecord) bool { return string(r.State) == stateOk && r.Value != nil }); best != nil {
		return best, nil
	}
	return pick(nil), nil
}

func dataDayOf(r *contracts.ReadingRecord) (time.Time, bool) {
	for _, p := range r.Parameters {
		if p.Param != "data_day" || p.ValueTag == nil {
			continue
		}
		if t, err := time.Parse(dayLayout, *p.ValueTag); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ivRvFigure is the figure (or Gap) of one index and its day text.
func ivRvFigure(name string, r *contracts.ReadingRecord, d time.Time) (any, string) {
	gapOf := func(reason, text string) (any, string) {
		g := mkGap(reason, name+" "+text)
		return g, g.Text
	}
	if r == nil {
		return gapOf(reasonNoData, volGapNoRow)
	}
	day, hasDay := dataDayOf(r)
	if string(r.State) == "missing_input" && r.ReasonCode != nil && *r.ReasonCode == codeStaleInput {
		return gapOf(reasonStaleInput, volGapStale)
	}
	if string(r.State) != stateOk || r.Value == nil || !hasDay {
		return gapOf(reasonMissingInput, volGapMissing)
	}
	if int(d.Sub(day).Hours()/24) > ivRvMaxAgeDays {
		return gapOf(reasonStaleInput, volGapStale)
	}
	v, err := shiftRound(*r.Value, 2, 1)
	if err != nil {
		return gapOf(reasonMissingInput, volGapMissing)
	}
	text := signed(v)
	if strings.Trim(v, "0.") == "" {
		text = v
	}
	return figure{Kind: "value", Value: json.Number(v), Text: text, Unit: unitVolPoints}, volGapDayPrefix + day.Format("01-02")
}
