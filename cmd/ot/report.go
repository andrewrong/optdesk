package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/report_blocks"
	"github.com/andrewrong/optdesk/internal/pages/report_page"
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
	rd, closeRd, err := reportBlockReaders(ctx, a, m, v)
	if err != nil {
		closeFn()
		return pages.Page{}, nil, err
	}
	return reportPageOver(a, m, s, v, rd), func() { closeRd(); closeFn() }, nil
}

// reportBlockReaders returns the readers the actions, account_moves and market_watch blocks read, as
// siteReaders (view is v), and a function that closes them: the demo readers with the demo profile, else the
// series and account stores opened with the pages line when deps has an opener (a missing opener is a nil
// reader, which the block shows as the gap no_reader). No reader of master.session_day is opened today, as in
// openSiteReaders.
func reportBlockReaders(ctx context.Context, a *app, m string, v store.ReadingViewReader) (siteReaders, func(), error) {
	if m == "JP" {
		return siteReaders{view: v}, func() {}, nil // the builder never calls the blocks of JP
	}
	if a.profile.Name == "demo" {
		return siteReaders{view: v, series: demoReader{}, noticeAccount: demoNoticeAccount{}}, func() {}, nil
	}
	rd := siteReaders{view: v}
	if a.deps.openSeries == nil && a.deps.openAccount == nil {
		return rd, func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return siteReaders{}, nil, err
	}
	dsn := env[pagesDSNName]
	var closers []func() error
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	if a.deps.openSeries != nil {
		ser, err := a.deps.openSeries(ctx, dsn)
		if err != nil {
			closeAll()
			return siteReaders{}, nil, fmt.Errorf("open stores with %s failed", pagesDSNName)
		}
		closers = append(closers, ser.Close)
		rd.series = ser
	}
	if a.deps.openAccount != nil {
		acc, err := a.deps.openAccount(ctx, dsn)
		if err != nil {
			closeAll()
			return siteReaders{}, nil, fmt.Errorf("open stores with %s failed", pagesDSNName)
		}
		closers = append(closers, acc.Close)
		rd.noticeAccount = acc
	}
	return rd, closeAll, nil
}

// reportBlocksConfig is the one report_blocks.Config of the report: the zone, the aliases (main first) of
// noticeAliases and the readers of rd. The option ledger fields stay nil, so the blocks that need them show
// no_reader. An error of noticeAliases leaves the aliases empty, which is the same gap.
func reportBlocksConfig(a *app, rd siteReaders) report_blocks.Config {
	c := report_blocks.Config{
		Zone: a.profile.Zone, Account: rd.noticeAccount, Series: rd.series, Sessions: rd.sessions,
		Figures: pagefig.New,
	}
	if main, labels, err := noticeAliases(a); err == nil {
		c.Aliases = append([]string{main}, slices.DeleteFunc(slices.Clone(labels), func(l string) bool { return l == main })...)
	}
	return c
}

// reportPageOver is the report page for envelope values m and s whose risks block is read from v and whose
// actions, account_moves and market_watch blocks are built by report_blocks from the readers rd. A nil
// reader of rd keeps the gap of its block; market JP never calls them.
func reportPageOver(a *app, m, s string, v store.ReadingViewReader, rd siteReaders) pages.Page {
	risks := func(ctx context.Context, day contracts.Day, at time.Time) ([]byte, error) {
		rules, err := a.deps.alertRules()
		if err != nil {
			return nil, err
		}
		return a.deps.riskBlock(ctx, rules, a.deps.evaluateAlerts, v.WinnerReadings, day, at.In(a.profile.Zone))
	}
	bc := reportBlocksConfig(a, rd)
	return a.deps.reportPage(report_page.Config{
		Market: m, Slot: s, Zone: a.profile.Zone, Risks: risks,
		Actions:      report_page.BlockFunc(report_blocks.Actions(bc)),
		AccountMoves: report_page.BlockFunc(report_blocks.AccountMoves(bc)),
		MarketWatch:  report_page.BlockFunc(report_blocks.MarketWatch(bc)),
	})
}

// buildReport builds the report page of the query (market, slot) for the day of dayFlag or
// today, per request, from the reading reader of the site and the block readers of reportBlockReaders, opened
// and closed for the request. A bad query is errReportQuery.
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
	rd, closeRd, err := reportBlockReaders(ctx, a, m, v)
	if err != nil {
		return nil, err
	}
	defer closeRd()
	return reportPageOver(a, m, s, v, rd).Build(ctx, nil, day, clk.Now().UTC())
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
