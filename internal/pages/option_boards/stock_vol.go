package option_boards

import (
	"context"
	"fmt"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// volReadDays is how many days before the page's as-of day the stored readings are searched; it equals the
// candidate lookback of load.go.
const volReadDays = 7

const (
	volSubjectKind = "underlying"
	volNoInput     = "no_input"
	volEdgeLabel   = "edge"
)

// volFigure is one row of the view: the label written and the id of the stored figure it shows.
type volFigure struct{ label, id string }

// volFigures are the rows of the view, in the order written.
var volFigures = []volFigure{
	{"iv_name", "market.iv_ibkr"},
	{"rv20", "market.rv20"},
	{"rv60", "market.rv60"},
	{"rv120", "market.rv120"},
	{"forecast", "market.vol_forecast"},
	{volEdgeLabel, "market.vol_edge"},
	{"ivp", "market.ivp_ibkr"},
	{"forecast_err", "market.vol_forecast_err"},
}

// volReasons are the stored reason codes of a missing reading; an unknown or absent code is no_input.
var volReasons = map[int]string{1: volNoInput, 2: "short_history", 3: "stale_input", 4: "zero_range", 5: "bad_input"}

// buildVolView is the view of the name subject on day, read from the stored readings by id. A nil reader or an empty
// subject gives every row a gap: the view is never missing. A reader error is an error.
func buildVolView(ctx context.Context, r store.ReadingViewReader, subject string, day contracts.Day) (volView, error) {
	v := volView{Name: subject, Method: volMethodText, Rows: make([]volRow, 0, len(volFigures))}
	from, err := daysBack(day, volReadDays)
	if err != nil {
		return volView{}, err
	}
	edge := ""
	for _, f := range volFigures {
		row := volRow{Label: f.label, Gap: volNoInput}
		if r != nil && subject != "" {
			recs, err := r.WinnerReadings(ctx, f.id, volSubjectKind, subject, from, day)
			if err != nil {
				return volView{}, fmt.Errorf("option_boards: readings of %s: %w", f.id, err)
			}
			row = volRowOf(f.label, recs, from, day)
		}
		if f.label == volEdgeLabel {
			edge = row.Text
		}
		v.Rows = append(v.Rows, row)
	}
	v.Coaching = volSentence(edge)
	return v, nil
}

// volRowOf picks the newest ok reading inside [from, to] as a ready row; with none, the newest missing reading
// inside it gives the gap, its reason and its day; with none at all the gap is no_input and the day is empty.
func volRowOf(label string, recs []contracts.ReadingRecord, from, to contracts.Day) volRow {
	ok, missing := volRow{Label: label, Gap: volNoInput}, volRow{Label: label, Gap: volNoInput}
	haveOK := false
	for _, rec := range recs {
		d := rec.Day.Format(dayLayout)
		if d < string(from) || d > string(to) {
			continue
		}
		if rec.State == "ok" && rec.Value != nil {
			if !haveOK || d >= ok.DataDay {
				ok, haveOK = volRow{Label: label, Text: *rec.Value, DataDay: d}, true
			}
		} else if d >= missing.DataDay {
			missing = volRow{Label: label, DataDay: d, Gap: volReasonOf(rec)}
		}
	}
	if haveOK {
		return ok
	}
	return missing
}

func volReasonOf(rec contracts.ReadingRecord) string {
	if rec.ReasonCode != nil {
		if s, ok := volReasons[*rec.ReasonCode]; ok {
			return s
		}
	}
	return volNoInput
}

// volSentence is the coaching text chosen by the stored edge text: above the dead band, below it, inside it (both
// ends inside), or no edge.
func volSentence(edge string) string {
	if strings.TrimSpace(edge) == "" {
		return volNoEdge
	}
	up, err := compareDecimal(edge, volDeadBand)
	if err != nil {
		return volNoEdge
	}
	down, _ := compareDecimal(edge, "-"+volDeadBand)
	switch {
	case up > 0:
		return volAbove
	case down < 0:
		return volBelow
	}
	return volInside
}

// volSubject is the ticker of the first matching row when there is one, else the key in upper case; key is the
// company the page resolved and pageKey the one asked for (the first is empty when no candidate row was found).
func volSubject(key, pageKey string, mine []store.Candidate, names displayNames) string {
	if len(mine) > 0 {
		if t := names.ticker(mine[0].UnderlyingID); t != nil {
			return *t
		}
	}
	if key == "" {
		key = pageKey
	}
	return strings.ToUpper(key)
}
