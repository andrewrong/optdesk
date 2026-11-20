package main

import (
	"context"
	"io"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	jobrunfig "github.com/andrewrong/optdesk/internal/compute/jobrun"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/data_health_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/ops"
)

// Design: docs/design/data-health.md#Interface (entry 6).

// siteDataHealthRoute is the address of the page data_health_page under the site root.
const siteDataHealthRoute = "data-health"

const (
	// dataHealthName is the name of the page for ot page.
	dataHealthName = "data_health_page"
	// siteDataHealthKey is the key of the coverage reader that the site opener puts in siteReaders.ext.
	siteDataHealthKey = "data_health_coverage"
	// extDataHealth is the name of the openers in the ext deps.
	extDataHealth = "data_health"
)

// dataHealthDeps are the openers of the page, in the ext deps so that wire.go holds nothing of the page.
type dataHealthDeps struct{ openCoverage coverage.OpenFunc }

func init() {
	extDeps[extDataHealth] = func() any { return dataHealthDeps{openCoverage: ops.OpenCoverage} }
	registerPage(pageFamily{name: dataHealthName, route: siteDataHealthRoute, order: 130, build: buildDataHealth, named: dataHealthPageNamed})
	siteOpeners[siteDataHealthKey] = openDataHealthCoverage
}

// openDataHealthCoverage opens the coverage reader with the pages line. Without an opener or the line it gives no
// reader and goes on: the page is then the gap coverage_unavailable.
func openDataHealthCoverage(ctx context.Context, a *app, track func(io.Closer, error) bool) (any, bool) {
	d, ok := a.deps.ext[extDataHealth].(dataHealthDeps)
	if !ok || d.openCoverage == nil {
		return nil, true
	}
	env, err := a.envFile()
	if err != nil {
		return nil, false
	}
	if env[pagesDSNName] == "" {
		return nil, true
	}
	r, err := d.openCoverage(ctx, env[pagesDSNName])
	if !track(r, err) {
		return nil, false
	}
	return r, true
}

// buildDataHealth builds data_health_page per request. With the owner edit on the page carries the form of the
// job console post; without it the form is nil and no cell has a button.
func buildDataHealth(ctx context.Context, _ *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	cov, _ := siteReaderOf[coverage.Reader](rd, siteDataHealthKey)
	var form *data_health_page.Form
	if rd.edit != nil {
		tok := ownerDummyToken()
		form = &data_health_page.Form{PostPath: "/" + jobConsolePostRoute, Expires: tok.Expires, MAC: tok.MAC}
	}
	return data_health_page.Build(ctx, data_health_page.Input{
		Now: clk.Now().UTC(), Today: day, Cov: cov, Jobs: dataHealthJobs(), Form: form,
		Runs: rd.jobRuns, Figures: jobrunfig.Figures,
	})
}

// dataHealthPageNamed gives the page to ot page: the demo profile and a missing line give no reader.
func dataHealthPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	var rd siteReaders
	closeFn := func() {}
	if a.profile.Name != "demo" {
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		if env[pagesDSNName] != "" {
			r, closeAll, err := openSiteReaders(ctx, a)
			if err != nil {
				return pages.Page{}, nil, err
			}
			rd, closeFn = r, closeAll
		}
	}
	build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		cov, _ := siteReaderOf[coverage.Reader](rd, siteDataHealthKey)
		return data_health_page.Build(ctx, data_health_page.Input{Now: now.UTC(), Today: day, Cov: cov, Jobs: dataHealthJobs(),
			Runs: rd.jobRuns, Figures: jobrunfig.Figures})
	}
	return pages.Page{Meta: pages.Meta{Name: dataHealthName, Schema: "page.data_health_page.v1"}, Build: build}, closeFn, nil
}

// dataHealthManifest is what the page reads of one job besides jobTable: the calendar, the ledger datasets its
// Plan keys name (the same for every day) and the slot of its manifest as "HHMM".
type dataHealthManifest struct {
	calendar string
	datasets []string
	slot     string
}

const (
	calUS = "XNYS"
	calJP = "XTKS"
)

// dataHealthManifests is by job id; a job of jobTable missing here is in dataHealthNoLedger.
var dataHealthManifests = map[string]dataHealthManifest{
	"fred_series":             {calUS, []string{"obs.series_point"}, ""},
	"fundamentals":            {calUS, []string{"obs.fundamental_fact"}, ""},
	"compute_figures":         {calUS, []string{"readings.reading"}, ""},
	"option_candidates":       {calUS, []string{"candidates.option_candidate"}, ""},
	"book_risk":               {calUS, []string{"strategy.result"}, ""},
	"account_capture":         {calUS, []string{"account.capture"}, ""},
	"statement_capture":       {calUS, []string{"account.capture"}, ""},
	"position_marks":          {calUS, []string{"account.position_mark"}, ""},
	"earnings_refresh":        {calUS, []string{"obs.earnings_date"}, ""},
	"dividend_capture":        {calUS, []string{"obs.dividend_expectation"}, ""},
	"option_eod":              {calUS, []string{"obs.option_eod"}, "1100"},
	"option_eod_jp":           {calJP, []string{"obs.option_eod_jp"}, "1030"},
	"option_eod_index":        {calUS, []string{"obs.option_eod_index"}, "1100"},
	"quote_snapshot":          {calUS, []string{"obs.quote_snapshot"}, ""},
	"calendar_refresh":        {calUS, []string{"master.session_day"}, ""},
	"calendar_hours_us":       {calUS, []string{"master.session_hours_notice"}, ""},
	"calendar_hours_jp":       {calJP, []string{"master.session_hours_notice"}, ""},
	"underlying_resolve":      {calUS, []string{"settings.underlying"}, ""},
	"fx_refresh":              {calUS, []string{"obs.fx_rate"}, ""},
	"iv_history":              {calUS, []string{"readings.reading"}, ""},
	"option_list_preopen":     {calUS, []string{"master.option_list"}, ""},
	"option_list_zone":        {calUS, []string{"master.option_list"}, ""},
	"option_list_preopen_jp":  {calJP, []string{"master.option_list.jp"}, ""},
	"bars_refresh":            {calUS, []string{"obs.price_bar"}, ""},
	"option_open":             {calUS, []string{"obs.option_open"}, ""},
	"option_open_jp":          {calJP, []string{"obs.option_open.jp"}, ""},
	"option_open_plan":        {calUS, []string{"plan.option_open_item"}, ""},
	"option_open_plan_jp":     {calJP, []string{"plan.option_open_item.jp"}, ""},
	"option_open_resolve":     {calUS, []string{"master.option_contract"}, ""},
	"option_open_resolve_jp":  {calJP, []string{"master.option_contract.jp"}, ""},
	"option_open_resolve_gap": {calUS, []string{"master.option_contract.gap"}, ""},
	"option_candidates_jp":    {calJP, []string{"candidates.option_candidate"}, ""},
	"option_ledger":           {calUS, []string{"optionledger.flex_trade_row"}, ""},
	"macro_events":            {calUS, []string{"obs.macro_event"}, ""},
}

// dataHealthNoLedger is the fixed list of the jobs of jobTable that store no ledger dataset, with the reason. recon_bars is the one that stores
// a run of its own; a job added to jobTable without a dataset goes here with its reason.
var dataHealthNoLedger = map[string]string{
	"recon_bars": "it stores a reconciliation run and no ledger dataset",
}

// dataHealthPresentOnly is the fixed list of jobs whose past-day run stores today's source under the day
// (Decision 5).
var dataHealthPresentOnly = []string{"earnings_refresh", "dividend_capture"}

// dueAtOfSlot turns a manifest slot "HHMM" into "HH:MM"; anything else gives "".
func dueAtOfSlot(slot string) string {
	if len(slot) != 4 {
		return ""
	}
	if _, err := time.Parse("1504", slot); err != nil {
		return ""
	}
	return slot[:2] + ":" + slot[2:]
}

// dataHealthTimeSlot is the slot ("HHMM") and zone of a time-anchored job whose jobTable row is any-day.
type dataHealthTimeSlot struct{ slot, zone string }

// dataHealthTimeSlots is by job id; the slots match the anchor offsets of the job.toml of each job.
var dataHealthTimeSlots = map[string]dataHealthTimeSlot{
	"fred_series":  {"0730", "America/New_York"},
	"fundamentals": {"0800", "America/New_York"},
	"macro_events": {"0600", "America/New_York"},
}

// dataHealthJobs builds the jobs of the page from jobTable, one per row in its order, and the manifests.
func dataHealthJobs() []data_health_page.Job {
	out := make([]data_health_page.Job, 0, len(jobTable))
	for _, r := range jobTable {
		m := dataHealthManifests[r.ID]
		zone, dueAt := r.Zone, dueAtOfSlot(m.slot)
		if ts, ok := dataHealthTimeSlots[r.ID]; ok {
			dueAt = dueAtOfSlot(ts.slot)
			if zone == "" {
				zone = ts.zone
			}
		}
		out = append(out, data_health_page.Job{
			ID: r.ID, Kind: r.Kind, Zone: zone, WhyText: r.Text,
			Datasets: slices.Clone(m.datasets), Calendar: m.calendar,
			PresentOnly: slices.Contains(dataHealthPresentOnly, r.ID), DueAt: dueAt,
		})
	}
	return out
}
