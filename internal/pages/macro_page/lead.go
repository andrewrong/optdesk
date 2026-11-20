package macro_page

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// Design: docs/design/macro-page-v2.md#decisions 2, 3 and 10.

const (
	leadTitle = "What a put seller reads first"
	leadNote  = "display only; no rule reads this"

	// defaultWindowDays is the window the page uses when Window answers with no days; cmd/ot gives the registry default.
	defaultWindowDays = 45

	noBigEvent = "no big event stored in the next 60 days"
	sourceSet  = "(your max_dte setting)"
	sourceNone = "(default, none set)"
)

// WindowFunc returns the option window in days at the time at and whether the owner stored it. A failed read of the
// setting is answered with the default and false, never an error.
type WindowFunc func(ctx context.Context, at time.Time) (days int, stored bool)

// bigKinds are the kinds of the lead row next_event.
var bigKinds = map[string]bool{"cpi": true, "employment_situation": true, "pce": true, "gdp": true, "fomc": true, "boj": true}

type leadRow struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Text       string  `json:"text"`
	DataDay    *string `json:"data_day"`
	ChangeText *string `json:"change_text"`
	Hint       string  `json:"hint"`
	State      string  `json:"state"`
}

type lead struct {
	Title string    `json:"title"`
	Rows  []leadRow `json:"rows"`
	Note  string    `json:"note"`
}

// windowOf is the window of the page: Window's answer, the default when it gives no days.
func windowOf(ctx context.Context, c Config, at time.Time) (int, bool) {
	days, stored := c.Window(ctx, at)
	if days <= 0 {
		return defaultWindowDays, false
	}
	return days, stored
}

func sourceWords(stored bool) string {
	if stored {
		return sourceSet
	}
	return sourceNone
}

// buildLead returns the four lead rows in order. events are the stored events of the page window. An error of a series
// read is an error.
func buildLead(ctx context.Context, r store.SeriesReader, events []macroevent.Event, d time.Time, days int, stored bool) (*lead, error) {
	from := contracts.Day(d.AddDate(0, 0, -lookbackDays).Format(dayLayout))
	to := contracts.Day(d.Format(dayLayout))
	read := func(id string) ([]obsPoint, error) {
		pts, err := r.SeriesPoints(ctx, id, from, to)
		if err != nil {
			return nil, fmt.Errorf("macro_page: read %s: %w", id, err)
		}
		return observations(pts, d), nil
	}
	tbill, err := read("DGS3MO")
	if err != nil {
		return nil, err
	}
	hy, err := read("BAMLH0A0HYM2")
	if err != nil {
		return nil, err
	}
	iv := map[string][]obsPoint{}
	for _, id := range []string{"VIXCLS", "VXNCLS", "RVXCLS"} {
		if iv[id], err = read(id); err != nil {
			return nil, err
		}
	}
	return &lead{
		Title: leadTitle,
		Rows: []leadRow{
			rateRow("tbill", "3-month T-bill", "DGS3MO", tbill, hintTbill),
			rateRow("hy_spread", "High-yield spread", "BAMLH0A0HYM2", hy, hintHySpread),
			nextEventRow(events, d, days, stored),
			indexIVRow(iv),
		},
		Note: leadNote,
	}, nil
}

func gapRow(key, label, text, hint string) leadRow {
	return leadRow{Key: key, Label: label, Text: text, Hint: hint, State: "gap"}
}

// rateRow prints the latest value of a percent series with its day and its change against five observations earlier.
func rateRow(key, label, id string, obs []obsPoint, hint string) leadRow {
	if len(obs) == 0 {
		return gapRow(key, label, "no_data: "+id+" has no stored point", hint)
	}
	last := obs[len(obs)-1]
	day := last.day.Format(dayLayout)
	row := leadRow{Key: key, Label: label, Text: valueText(last.value, unitPercent), DataDay: &day, Hint: hint, State: "ok"}
	if c := changeText(obs, changeBack, unitPercent, "five sessions"); c != "" {
		row.ChangeText = &c
	}
	return row
}

// nextEventRow is the nearest stored event of the big kinds, with the days to it and the window words.
func nextEventRow(events []macroevent.Event, d time.Time, days int, stored bool) leadRow {
	const label = "Next big event"
	hint := hintNextEvent + hintWindow
	var best *macroevent.Event
	var bestDay time.Time
	for i, e := range events {
		if !bigKinds[e.Kind] {
			continue
		}
		ed, err := time.Parse(dayLayout, string(e.Date))
		if err != nil || ed.Before(d) {
			continue
		}
		if best == nil || ed.Before(bestDay) {
			best, bestDay = &events[i], ed
		}
	}
	if best == nil {
		return gapRow("next_event", label, noBigEvent, hint)
	}
	n := daysBetween(d, bestDay)
	name := best.Kind
	if c, ok := conventionOf[best.Kind]; ok {
		name = c.name
	}
	where := "inside"
	if n > days {
		where = "after"
	}
	unit := "days"
	if n == 1 {
		unit = "day"
	}
	text := fmt.Sprintf("%s, %s, in %d %s, %s the %d-day option window %s", name, best.Date, n, unit, where, days, sourceWords(stored))
	return leadRow{Key: "next_event", Label: label, Text: text, Hint: hint, State: "ok"}
}

// indexIVRow prints VIX, VXN and RVX in one line; a day is added to a part when the days differ.
func indexIVRow(iv map[string][]obsPoint) leadRow {
	const key, label = "index_iv", "Index implied volatility"
	parts := []struct{ name, id string }{{"VIX", "VIXCLS"}, {"VXN", "VXNCLS"}, {"RVX", "RVXCLS"}}
	var newest time.Time
	differ, missing := false, false
	for _, p := range parts {
		obs := iv[p.id]
		if len(obs) == 0 {
			missing = true
			continue
		}
		day := obs[len(obs)-1].day
		if !newest.IsZero() && !day.Equal(newest) {
			differ = true
		}
		if day.After(newest) {
			newest = day
		}
	}
	if newest.IsZero() {
		return gapRow(key, label, "no_data: VIXCLS, VXNCLS and RVXCLS have no stored point", hintIndexIV)
	}
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		obs := iv[p.id]
		if len(obs) == 0 {
			texts = append(texts, p.name+" no_data")
			continue
		}
		last := obs[len(obs)-1]
		t := p.name + " " + valueText(last.value, unitIndex)
		if differ {
			t += " (" + last.day.Format(dayLayout) + ")"
		}
		texts = append(texts, t)
	}
	day := newest.Format(dayLayout)
	state := "ok"
	if missing {
		state = "gap"
	}
	return leadRow{Key: key, Label: label, Text: strings.Join(texts, " · "), DataDay: &day, Hint: hintIndexIV, State: state}
}

// daysBetween is the whole days from a to b, both midnight of the same zone.
func daysBetween(a, b time.Time) int {
	return int(b.Sub(a).Hours() / 24)
}
