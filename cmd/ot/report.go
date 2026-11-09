package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// reportName is the page name of the report: it is built from stored readings, not from a series reader.
const reportName = "report_page"

// reportDay is the day of a report; wire.go names it without importing contracts.
type reportDay = contracts.Day

// riskBlockFunc returns the JSON of the risks block for the rules, their evaluation and the reader of readings.
type riskBlockFunc func(
	ctx context.Context, rules compute.AlertRules, eval compute.EvaluateAlertsFunc, read compute.ReadingFunc,
	today reportDay, at time.Time,
) ([]byte, error)

// errReportQuery marks a request whose market or slot is missing or invalid; the site answers 400.
var errReportQuery = errors.New("report: market or slot is missing or invalid")

// reportParams turns the lower-case flag or query values into the envelope values of the page
// (US|JP, open|close). An error names the flag.
func reportParams(market, slot string) (string, string, error) {
	var m string
	switch market {
	case "us", "jp":
		m = strings.ToUpper(market)
	case "":
		return "", "", errors.New("--market is required for report_page (us or jp)")
	default:
		return "", "", errors.New("--market must be us or jp")
	}
	switch slot {
	case "open", "close":
		return m, slot, nil
	case "":
		return "", "", errors.New("--slot is required for report_page (open or close)")
	default:
		return "", "", errors.New("--slot must be open or close")
	}
}

// reportReader returns the reading-view reader of ot page and a close function: the demo
// reader with the demo profile (no store), else the view reader opened with the .env line
// pages line, which is the role ot_ro_pages.
func reportReader(ctx context.Context, a *app) (store.ReadingViewReader, func(), error) {
	if a.profile.Name == "demo" {
		return demoView{}, func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return nil, nil, err
	}
	line := store.DSNEnvPages
	dsn := env[line]
	if dsn == "" {
		return nil, nil, fmt.Errorf("%s is missing in .env", line)
	}
	v, err := a.deps.openReadingView(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open reading view with %s failed", line)
	}
	return v, func() { _ = v.Close() }, nil
}

// reportPage returns the report page of a market and slot over one opened reader, and the
// function that closes the reader. The flag values are the lower-case ones of the command.
func reportPage(ctx context.Context, a *app, market, slot string) (pages.Page, func(), error) {
	m, s, err := reportParams(market, slot)
	if err != nil {
		return pages.Page{}, nil, err
	}
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New("report_page needs the owner's zone: " + zoneEnvName + " is not set")
	}
	v, closeFn, err := reportReader(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	return reportPageOver(a, m, s, v), closeFn, nil
}

// reportPageOver is the report page for envelope values m and s whose risks block is read from v.
func reportPageOver(a *app, m, s string, v store.ReadingViewReader) pages.Page {
	risks := func(ctx context.Context, day contracts.Day, at time.Time) ([]byte, error) {
		rules, err := a.deps.alertRules()
		if err != nil {
			return nil, err
		}
		return a.deps.riskBlock(ctx, rules, a.deps.evaluateAlerts, v.WinnerReadings, day, at.In(a.profile.Zone))
	}
	return a.deps.reportPage(m, s, a.profile.Zone, risks)
}

// buildReport builds the report page of the query (market, slot) for the day of dayFlag or
// today, per request, from the reader of the site. A bad query is errReportQuery.
func buildReport(ctx context.Context, a *app, clk clock.Clock, dayFlag string, v store.ReadingViewReader, q url.Values) ([]byte, error) {
	m, s, err := reportParams(q.Get("market"), q.Get("slot"))
	if err != nil {
		return nil, errReportQuery
	}
	if a.profile.Zone == nil {
		return nil, errors.New("report_page: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return reportPageOver(a, m, s, v).Build(ctx, nil, day, clk.Now().UTC())
}

// demoView is the reading-view reader of the demo profile: one synthetic ok reading of the
// requested day range, carrying the parameter data_day, for any figure and subject.
type demoView struct{}

// WinnerReadings returns one reading dated to, with value 0.5 and data_day to; every rule
// holds a unit decimal. An empty range gives no row.
func (demoView) WinnerReadings(_ context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	if to < from {
		return nil, nil
	}
	d, err := time.Parse(dayLayout, string(to))
	if err != nil {
		return nil, err
	}
	v := "0.5"
	rec := contracts.ReadingRecord{
		Name: name, SubjectKind: subjectKind, Subject: subject,
		Day: types.SerializableDate{Time: d}, Rev: 1, Version: 1,
		State: "ok", Unit: "decimal", Value: &v,
	}
	param, err := json.Marshal([]map[string]any{{"param": "data_day", "value_num": nil, "value_tag": string(to)}})
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(param, &rec.Parameters); err != nil {
		return nil, err
	}
	return []contracts.ReadingRecord{rec}, nil
}

func (demoView) Close() error { return nil }
