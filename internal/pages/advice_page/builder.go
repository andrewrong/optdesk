package advice_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	calendarID      = "XNYS"
	dayLayout       = "2006-01-02"
	timeLayout      = "2006-01-02T15:04:05-07:00"
	sessionLookback = 14 // calendar days of session days to read
	pageTitle       = "Advice review"
	textNotBuilt    = "no record of earlier advice is stored yet"
)

type gapJSON struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type pageJSON struct {
	Schema        string     `json:"schema"`
	SchemaVersion int        `json:"schema_version"`
	RunID         string     `json:"run_id"`
	BuildID       string     `json:"build_id"`
	Scope         string     `json:"scope"`
	GeneratedAt   string     `json:"generated_at"`
	DataTimes     []struct{} `json:"data_times"`
	RunStatus     string     `json:"run_status"`
	Title         string     `json:"title"`
	Sentence      string     `json:"sentence"`
	Day           string     `json:"day"`
	Rows          []struct{} `json:"rows"`
	Gap           *gapJSON   `json:"gap"`
}

// Build writes the advice_page for day; at is generated_at. day of the page is the session day before
// the request's day, or the calendar day before when there is no reader or no earlier session.
func Build(ctx context.Context, c Config, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("advice_page: no zone")
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("advice_page: bad day %q: %w", day, err)
	}
	prev := d.AddDate(0, 0, -1)
	if c.Sessions != nil {
		from := contracts.Day(d.AddDate(0, 0, -sessionLookback).Format(dayLayout))
		to := contracts.Day(prev.Format(dayLayout))
		rows, err := c.Sessions.SessionDays(ctx, calendarID, from, to)
		if err != nil {
			return nil, fmt.Errorf("advice_page: read session days: %w", err)
		}
		var best time.Time
		for _, r := range rows {
			if r.IsSession && r.SessionDay.Before(d) && r.SessionDay.After(best) {
				best = r.SessionDay.Time
			}
		}
		if !best.IsZero() {
			prev = best
		}
	}
	return json.Marshal(pageJSON{
		Schema: "advice_page", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: "advice",
		GeneratedAt: at.In(c.Zone).Format(timeLayout), DataTimes: []struct{}{}, RunStatus: "partial",
		Title: pageTitle, Sentence: textNotBuilt, Day: prev.Format(dayLayout), Rows: []struct{}{},
		Gap: &gapJSON{Reason: "not_built", Text: textNotBuilt},
	})
}
