package main

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

// The texts of the force-no reasons, one per code.
const (
	textMomentMix      = "force comes with the late-collection design (each pull stored with its own time)"
	textAppendOnly     = "a forced run adds executions by (alias, exec_id) and never replaces the day's capture"
	textRevisionRows   = "a second fetch adds revision rows that no reader prefers over the first"
	textInsertOnly     = "a stored capture of a statement day is never replaced, so a forced run writes nothing new"
	textAttemptRows    = "a re-run adds one row under the next attempt, which replaces nothing"
	textNewRunID       = "changed inputs make a new run id and a second run of rows"
	textQuota          = "the Alpha Vantage quota is 25 a day"
	textNotProved      = "nothing read proves that a second run equals the first"
	textNoLowerBound   = "a run before its slot would store pre-open prices as the open"
	needsLowerBound    = "its Run must have a lower bound at the slot instant"
	textNoDayGuard     = "a late run makes a key that the open job answers late"
	needsDayGuard      = "the plan job must refuse the day and the time that the pull refuses"
	textHeavyAndCap    = "the budget exceeds the run bound so a run is always cut, and the per-day cap in memory is shared with the scheduler"
	needsHeavyAndCap   = "a console run must have a budget and a cap of its own"
	textCachesFailures = "a failed result is cached for the day, so a run neither retries nor can be forced"
	needsCachesFailure = "the cache must keep only successes, or Run must take a force flag"
	noteQuoteSnapshot  = "its three keys, 09:45, 12:30 and 15:30 ET, share one scheduled instant, 15:30 ET, so all three wait for it; " +
		"only the 15:30 key can produce, and only until ten minutes after it; a 09:45 or 12:30 key that is still missing " +
		"ends no value (missed slot), as the scheduler's own tick at 15:30 would write"
)

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

// jobTable is the table of the 31 jobs, in the order of the page.
var jobTable = []jobRow{
	{ID: "fred_series", Kind: kindAnyDay, Force: true},
	anyDay("fundamentals", "quota", textQuota),
	anyDay("compute_figures", "revision_rows", textRevisionRows),
	anyDay("option_candidates", "new_run_id", textNewRunID),
	todayOnly("account_capture", "append_only", textAppendOnly),
	anyDay("statement_capture", "insert_only", textInsertOnly),
	todayOnly("position_marks", "attempt_rows", textAttemptRows),
	todayOnly("earnings_refresh", "attempt_rows", textAttemptRows),
	todayOnly("dividend_capture", "attempt_rows", textAttemptRows),
	todayOnly("option_eod", "moment_mix", textMomentMix),
	{ID: "option_eod_jp", Kind: kindTodayOnly, Zone: "Asia/Tokyo", ForceReason: "moment_mix", ForceText: textMomentMix},
	todayOnly("option_eod_index", "moment_mix", textMomentMix),
	{ID: "quote_snapshot", Kind: kindTodayOnly, Zone: marketZone, ForceReason: "moment_mix", ForceText: textMomentMix, Note: noteQuoteSnapshot},
	{ID: "calendar_refresh", Kind: kindTodayOnly, Zone: marketZone, Force: true},
	todayOnly("calendar_hours_us", "not_proved", textNotProved),
	{ID: "calendar_hours_jp", Kind: kindTodayOnly, Zone: "Asia/Tokyo", ForceReason: "not_proved", ForceText: textNotProved},
	todayOnly("underlying_resolve", "not_proved", textNotProved),
	{ID: "fx_refresh", Kind: kindAnyDay, Force: true},
	todayOnly("iv_history", "not_proved", textNotProved),
	todayOnly("option_list_preopen", "not_proved", textNotProved),
	todayOnly("option_list_zone", "not_proved", textNotProved),
	{ID: "option_list_preopen_jp", Kind: kindTodayOnly, Zone: "Asia/Tokyo", ForceReason: "not_proved", ForceText: textNotProved},
	{ID: "bars_refresh", Kind: kindAnyDay, Force: true},
	listedOnly("option_open", "no_lower_time_bound", textNoLowerBound, needsLowerBound),
	listedOnly("option_open_jp", "no_lower_time_bound", textNoLowerBound, needsLowerBound),
	listedOnly("option_open_plan", "no_day_guard", textNoDayGuard, needsDayGuard),
	listedOnly("option_open_plan_jp", "no_day_guard", textNoDayGuard, needsDayGuard),
	listedOnly("option_open_resolve", "heavy_and_shares_cap", textHeavyAndCap, needsHeavyAndCap),
	listedOnly("option_open_resolve_jp", "heavy_and_shares_cap", textHeavyAndCap, needsHeavyAndCap),
	listedOnly("option_open_resolve_gap", "heavy_and_shares_cap", textHeavyAndCap, needsHeavyAndCap),
	listedOnly("option_ledger", "caches_failures", textCachesFailures, needsCachesFailure),
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
