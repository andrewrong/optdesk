package main

import "github.com/andrewrong/optdesk/internal/pages/job_console"

// Design: docs/design/job-console.md#Decisions 6 and 10.

// The kinds of a job on the console page.
const (
	// kindAnyDay: the Run uses the day it is given; a day not after today is accepted.
	kindAnyDay = "any_day"
	// kindTodayOnly: the Run answers for the day of now; only today in the job's zone is accepted.
	kindTodayOnly = "today_only"
	// kindListedOnly: listed on the page with the reason it cannot be run; no form.
	kindListedOnly = "listed_only"
)

// jobRow is the console's declaration of one job: what it can do from the page. The page builder
// gets its own copy of the fields it shows.
type jobRow struct {
	// ID is the job id the wiring builds.
	ID string
	// Kind is kindAnyDay, kindTodayOnly or kindListedOnly.
	Kind string
	// Zone is the zone whose date is "today" for a today-only job; empty for the other kinds.
	Zone string
	// Force is true when a forced run of a present key replaces or revises its rows.
	Force bool
	// ForceReason and ForceText say why a runnable job offers no force (empty when Force is true or the job is
	// listed only).
	ForceReason, ForceText string
	// Reason, Text and Needs, for a listed-only job: the reason code, why it cannot be run and what would
	// make it runnable.
	Reason, Text, Needs string
	// Note is shown beside the form of a runnable job; empty when there is none.
	Note string
}

// runnable tells whether the page can queue a run of the job.
func (r jobRow) runnable() bool { return r.Kind == kindAnyDay || r.Kind == kindTodayOnly }

// anyDay builds the row of an any-day job with its force-no reason.
func anyDay(id, reason, text string) jobRow {
	return jobRow{ID: id, Kind: kindAnyDay, ForceReason: reason, ForceText: text}
}

// todayOnly builds the row of a today-only job in the market zone with its force-no reason.
func todayOnly(id, reason, text string) jobRow {
	return jobRow{ID: id, Kind: kindTodayOnly, Zone: marketZone, ForceReason: reason, ForceText: text}
}

// listedOnly builds the row of a listed-only job.
func listedOnly(id, reason, text, needs string) jobRow {
	return jobRow{ID: id, Kind: kindListedOnly, Reason: reason, Text: text, Needs: needs}
}

// jobTable is the table of the 33 jobs, in the order of the page.
var jobTable = []jobRow{
	{ID: "fred_series", Kind: kindAnyDay, Force: true},
	{ID: "fundamentals", Kind: kindAnyDay, Force: true},
	{ID: "compute_figures", Kind: kindAnyDay, Force: true},
	{ID: "option_candidates", Kind: kindAnyDay, Force: true},
	// book_risk is insert-only under a new run id and a rerun's run id is the newest one readers take, as option_candidates.
	{ID: "book_risk", Kind: kindAnyDay, Force: true},
	todayOnly("account_capture", "append_only", job_console.ZhForceAppendOnly),
	anyDay("statement_capture", "insert_only", job_console.ZhForceInsertOnly),
	todayOnly("position_marks", "attempt_rows", job_console.ZhForceAttemptRows),
	{ID: "earnings_refresh", Kind: kindAnyDay, Force: true},
	{ID: "dividend_capture", Kind: kindAnyDay, Force: true},
	{ID: "option_eod", Kind: kindTodayOnly, Zone: marketZone, Force: true},
	{ID: "option_eod_jp", Kind: kindTodayOnly, Zone: "Asia/Tokyo", Force: true},
	{ID: "option_eod_index", Kind: kindTodayOnly, Zone: marketZone, Force: true},
	{ID: "option_candidates_jp", Kind: kindTodayOnly, Zone: "Asia/Tokyo", Force: true},
	{ID: "quote_snapshot", Kind: kindTodayOnly, Zone: marketZone, ForceReason: "moment_mix", ForceText: job_console.ZhForceMomentMix, Note: job_console.ZhNoteQuoteSnapshot},
	{ID: "calendar_refresh", Kind: kindTodayOnly, Zone: marketZone, Force: true},
	todayOnly("calendar_hours_us", "not_proved", job_console.ZhForceNotProved),
	{ID: "calendar_hours_jp", Kind: kindTodayOnly, Zone: "Asia/Tokyo", ForceReason: "not_proved", ForceText: job_console.ZhForceNotProved},
	todayOnly("underlying_resolve", "not_proved", job_console.ZhForceNotProved),
	{ID: "fx_refresh", Kind: kindAnyDay, Force: true},
	todayOnly("iv_history", "not_proved", job_console.ZhForceNotProved),
	todayOnly("option_list_preopen", "not_proved", job_console.ZhForceNotProved),
	todayOnly("option_list_zone", "not_proved", job_console.ZhForceNotProved),
	{ID: "option_list_preopen_jp", Kind: kindTodayOnly, Zone: "Asia/Tokyo", ForceReason: "not_proved", ForceText: job_console.ZhForceNotProved},
	{ID: "bars_refresh", Kind: kindAnyDay, Force: true},
	listedOnly("option_open", "no_lower_time_bound", job_console.ZhWhyNoLowerBound, job_console.ZhNeedsLowerBound),
	listedOnly("option_open_jp", "no_lower_time_bound", job_console.ZhWhyNoLowerBound, job_console.ZhNeedsLowerBound),
	listedOnly("option_open_plan", "no_day_guard", job_console.ZhWhyNoDayGuard, job_console.ZhNeedsDayGuard),
	listedOnly("option_open_plan_jp", "no_day_guard", job_console.ZhWhyNoDayGuard, job_console.ZhNeedsDayGuard),
	listedOnly("option_open_resolve", "heavy_and_shares_cap", job_console.ZhWhyHeavyAndCap, job_console.ZhNeedsHeavyAndCap),
	listedOnly("option_open_resolve_jp", "heavy_and_shares_cap", job_console.ZhWhyHeavyAndCap, job_console.ZhNeedsHeavyAndCap),
	listedOnly("option_open_resolve_gap", "heavy_and_shares_cap", job_console.ZhWhyHeavyAndCap, job_console.ZhNeedsHeavyAndCap),
	listedOnly("option_ledger", "caches_failures", job_console.ZhWhyCachesFailures, job_console.ZhNeedsCachesFailure),
	{ID: "macro_events", Kind: kindAnyDay, Force: true},
	{ID: "recon_bars", Kind: kindAnyDay, Force: true},
}

// jobRowOf returns the row of a job id.
func jobRowOf(id string) (jobRow, bool) {
	for _, r := range jobTable {
		if r.ID == id {
			return r, true
		}
	}
	return jobRow{}, false
}
