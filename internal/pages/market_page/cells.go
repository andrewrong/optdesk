package market_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const dayLayout = "2006-01-02"

// Reasons of the Gap of common.v2.json used here.
const (
	reasonNoData       = "no_data"
	reasonNotBuilt     = "not_built"
	reasonStale        = "stale"
	reasonMissingInput = "missing_input"
	reasonStaleInput   = "stale_input"
)

// Units of the figures.
const (
	unitSharePercent = "share_percent"
	unitIndexPoints  = "index_points"
	unitVolPoints    = "vol_points"
)

// States of a reading.
const (
	stateOk           = "ok"
	stateMissingInput = "missing_input"
)

// Codes of the numeric reason_code of a reading (the rules of compute_figures).
const (
	codeShortHistory = 2
	codeStaleInput   = 3
)

type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

func mkGap(reason, text string) gap { return gap{Kind: "gap", Reason: reason, Text: text} }

type figure struct {
	Kind  string      `json:"kind"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

func mkFigure(value, text, unit string) figure {
	return figure{Kind: "value", Value: number(value), Text: text, Unit: unit}
}

// reading is the newest stored row of one figure, or none.
type reading struct {
	rec     *contracts.ReadingRecord
	dataDay time.Time // zero when the row has no usable data_day
}

// readNewest asks for the six days day-5 to day and takes the newest row.
func readNewest(ctx context.Context, c Config, name, subject string, d time.Time) (reading, error) {
	return readNewestKind(ctx, c, name, kindMarket, subject, d)
}

// readNewestKind is readNewest for a subject kind other than market.
func readNewestKind(ctx context.Context, c Config, name, kind, subject string, d time.Time) (reading, error) {
	from := contracts.Day(d.AddDate(0, 0, -5).Format(dayLayout))
	rows, err := c.Readings(ctx, name, kind, subject, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return reading{}, fmt.Errorf("market_page: read %s: %w", name, err)
	}
	var best *contracts.ReadingRecord
	for i := range rows {
		r := &rows[i]
		if best == nil || r.Day.After(best.Day.Time) || (r.Day.Equal(best.Day.Time) && r.Rev > best.Rev) {
			best = r
		}
	}
	if best == nil {
		return reading{}, nil
	}
	out := reading{rec: best}
	for _, p := range best.Parameters {
		if p.Param != "data_day" || p.ValueTag == nil {
			continue
		}
		if t, err := time.Parse(dayLayout, *p.ValueTag); err == nil {
			out.dataDay = t
		}
	}
	return out, nil
}

// failure classifies a reading that has no figure: "none", "short", "stale" or "missing".
func (r reading) failure() string {
	if r.rec == nil {
		return "none"
	}
	if string(r.rec.State) == stateMissingInput && r.rec.ReasonCode != nil {
		switch *r.rec.ReasonCode {
		case codeShortHistory:
			return "short"
		case codeStaleInput:
			return "stale"
		}
	}
	return "missing"
}

// usable returns the value text of an ok row that has one.
func (r reading) usable() (string, bool) {
	if r.rec == nil || string(r.rec.State) != stateOk || r.rec.Value == nil {
		return "", false
	}
	return *r.rec.Value, true
}

// percentileCell is the rank_3y cell of a reading.
func percentileCell(r reading) any {
	if v, ok := r.usable(); ok {
		if !inUnitRange(v) {
			return mkGap(reasonMissingInput, pctMissing)
		}
		text, err := shiftRound(v, 2, 0)
		if err != nil {
			return mkGap(reasonMissingInput, pctMissing)
		}
		return mkFigure(text, text+"%", unitSharePercent)
	}
	switch r.failure() {
	case "none":
		return mkGap(reasonNoData, pctNoRow)
	case "short":
		return mkGap(reasonNoData, pctShortHistory)
	case "stale":
		if r.dataDay.IsZero() {
			return mkGap(reasonStaleInput, pctStaleNoDay)
		}
		return mkGap(reasonStaleInput, pctStale+r.dataDay.Format("01-02"))
	}
	return mkGap(reasonMissingInput, pctMissing)
}

// ivCell is the iv cell of one index: the stored IBKR value in vol points, or a Gap
// that names the index.
func ivCell(name string, r reading) any {
	if r.rec == nil {
		return mkGap(reasonNoData, name+" "+ivNoRow)
	}
	v, ok := r.usable()
	if !ok {
		return mkGap(reasonMissingInput, name+" "+ivMissing)
	}
	text, err := shiftRound(v, 2, 1)
	if err != nil || strings.HasPrefix(text, "-") || strings.Trim(text, "0.") == "" {
		return mkGap(reasonMissingInput, name+" "+ivMissing)
	}
	return mkFigure(text, text, unitVolPoints)
}

// rv20Cell is the rv20 cell of one index: the stored 20-day realised volatility in vol
// points, one decimal. A stored zero is a value; only a missing or negative input is a Gap.
func rv20Cell(name string, r reading) any {
	if r.rec == nil {
		return mkGap(reasonNoData, name+" "+rv20NoRow)
	}
	v, ok := r.usable()
	if !ok {
		return mkGap(reasonMissingInput, name+" "+rv20Missing)
	}
	text, err := shiftRound(v, 2, 1)
	if err != nil || strings.HasPrefix(text, "-") {
		return mkGap(reasonMissingInput, name+" "+rv20Missing)
	}
	return mkFigure(text, text, unitVolPoints)
}

// weatherSentence is the sentence of the weather block and whether it carries the figure.
func weatherSentence(r reading) (string, bool) {
	if v, ok := r.usable(); ok {
		text, err := shiftRound(v, 2, 2)
		if err != nil {
			return weatherMissing, false
		}
		sign := ""
		if text[0] != '-' && text != "0.00" {
			sign = "+"
		}
		return weatherSentenceFmt + sign + text + "%", true
	}
	switch r.failure() {
	case "none":
		return weatherNoRow, false
	case "stale":
		if !r.dataDay.IsZero() {
			return weatherStale + r.dataDay.Format("01-02"), false
		}
	}
	return weatherMissing, false
}

// termSentence is the sentence of the term block; a missing or bad ratio is a fixed text.
func termSentence(r reading) string {
	if v, ok := r.usable(); ok {
		if text, err := shiftRound(v, 0, 2); err == nil {
			return termSentenceFmt + text
		}
	}
	return termSentenceNone
}

type indexRow struct {
	Name        string `json:"name"`
	IV          any    `json:"iv"`
	RV20        any    `json:"rv20"`
	IVMinusRV20 any    `json:"iv_minus_rv20"`
	Rank3Y      any    `json:"rank_3y"`
}

type indexSpec struct{ name, figure, series string }

// indexSpecs are the four rows in page order; N225 has no figure.
var indexSpecs = []indexSpec{
	{"SPX", figVixPctile, seriesVIX},
	{"NDX", figVxnPctile, seriesVXN},
	{"RUT", figRvxPctile, seriesRVX},
	{"N225", "", ""},
}

type indices struct {
	Title    string     `json:"title"`
	Sentence string     `json:"sentence"`
	Columns  []string   `json:"columns"`
	Rows     []indexRow `json:"rows"`
	Note     string     `json:"note"`
}

// buildIndices fills the four rows and fills days with the data days of the readings;
// failed counts the ranks of SPX, NDX and RUT that are Gaps.
func buildIndices(ctx context.Context, c Config, d time.Time, days dataDays) (indices, int, error) {
	out := indices{Title: indicesTitle, Sentence: indicesSentence, Columns: indicesColumns, Note: indicesNote, Rows: []indexRow{}}
	failed := 0
	for _, s := range indexSpecs {
		row := indexRow{
			Name:        s.name,
			IV:          mkGap(reasonNotBuilt, s.name+" "+ivNotBuilt),
			RV20:        mkGap(reasonNotBuilt, s.name+" "+rv20NotBuilt),
			IVMinusRV20: mkGap(reasonNotBuilt, s.name+" "+ivRvNotBuilt),
			Rank3Y:      mkGap(reasonNoData, n225NoData),
		}
		if s.figure != "" {
			r, err := readNewest(ctx, c, s.figure, s.series, d)
			if err != nil {
				return indices{}, 0, err
			}
			days.add(s.series, r.dataDay)
			iv, err := readNewestKind(ctx, c, figIVIbkr, kindUnderlying, s.name, d)
			if err != nil {
				return indices{}, 0, err
			}
			row.IV = ivCell(s.name, iv)
			rv, err := readNewestKind(ctx, c, figRv20, kindUnderlying, s.name, d)
			if err != nil {
				return indices{}, 0, err
			}
			row.RV20 = rv20Cell(s.name, rv)
			row.Rank3Y = percentileCell(r)
			if _, isGap := row.Rank3Y.(gap); isGap {
				failed++
			}
		}
		out.Rows = append(out.Rows, row)
	}
	return out, failed, nil
}

// dataDays holds the newest data day per series.
type dataDays map[string]time.Time

func (m dataDays) add(series string, day time.Time) {
	if day.IsZero() {
		return
	}
	if cur, ok := m[series]; !ok || day.After(cur) {
		m[series] = day
	}
}
