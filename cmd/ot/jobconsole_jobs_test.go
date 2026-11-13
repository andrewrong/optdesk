package main

import (
	"maps"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
)

// wantTwelve is the test's own list of the today-only jobs with their zones (Decision 6); no zone is read
// out of a job, because most jobs carry none in Schedule().
var wantTwelve = map[string]string{
	"account_capture":        "America/New_York",
	"position_marks":         "America/New_York",
	"earnings_refresh":       "America/New_York",
	"dividend_capture":       "America/New_York",
	"option_eod":             "America/New_York",
	"option_eod_index":       "America/New_York",
	"option_eod_jp":          "Asia/Tokyo",
	"quote_snapshot":         "America/New_York",
	"calendar_refresh":       "America/New_York",
	"calendar_hours_us":      "America/New_York",
	"calendar_hours_jp":      "Asia/Tokyo",
	"underlying_resolve":     "America/New_York",
	"iv_history":             "America/New_York",
	"option_list_preopen":    "America/New_York",
	"option_list_zone":       "America/New_York",
	"option_list_preopen_jp": "Asia/Tokyo",
}

func idsOfKind(kind string) []string {
	var ids []string
	for _, r := range jobTable {
		if r.Kind == kind {
			ids = append(ids, r.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func TestTableIsTheWiredJobSet(t *testing.T) {
	t.Parallel()
	// The registered families plus the three jobs the wiring adds outside the registry.
	want := append(familyIDs(), "position_marks", "bars_refresh", "fx_refresh")
	var got []string
	seen := map[string]bool{}
	for _, r := range jobTable {
		if seen[r.ID] {
			t.Fatalf("%s twice", r.ID)
		}
		seen[r.ID] = true
		got = append(got, r.ID)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) || len(got) != 31 {
		t.Fatalf("table %v\nwired %v", got, want)
	}
}

func TestForceYesIsFredCalendarFxAndBars(t *testing.T) {
	t.Parallel()
	var yes []string
	for _, r := range jobTable {
		if r.Force {
			yes = append(yes, r.ID)
		}
	}
	if !slices.Equal(yes, []string{"fred_series", "calendar_refresh", "fx_refresh", "bars_refresh"}) {
		t.Fatalf("force yes: %v", yes)
	}
}

func TestEveryForceNoJobHasReason(t *testing.T) {
	t.Parallel()
	codes := []string{"moment_mix", "append_only", "revision_rows", "attempt_rows", "new_run_id", "quota", "not_proved", "insert_only"}
	n := 0
	for _, r := range jobTable {
		if !r.runnable() || r.Force {
			if r.ForceReason != "" || r.ForceText != "" {
				t.Errorf("%s carries a force reason it needs not", r.ID)
			}
			continue
		}
		n++
		if !slices.Contains(codes, r.ForceReason) || r.ForceText == "" {
			t.Errorf("%s: reason %q text %q", r.ID, r.ForceReason, r.ForceText)
		}
	}
	if n != 19 {
		t.Fatalf("%d force-no jobs, want 19", n)
	}
}

func TestZoneOfEachTodayOnlyJobIsTheJobsOwn(t *testing.T) {
	t.Parallel()
	got := map[string]string{}
	for _, r := range jobTable {
		if r.Kind == kindTodayOnly {
			got[r.ID] = r.Zone
		}
	}
	if !maps.Equal(got, wantTwelve) {
		t.Fatalf("table zones %v, want %v", got, wantTwelve)
	}
	for id, z := range wantTwelve {
		if _, err := time.LoadLocation(z); err != nil {
			t.Errorf("%s: zone %s: %v", id, z, err)
		}
	}
}

func TestAnyDayIsExactlyTheSeven(t *testing.T) {
	t.Parallel()
	want := []string{"bars_refresh", "compute_figures", "fred_series", "fundamentals", "fx_refresh", "option_candidates", "statement_capture"}
	if got := idsOfKind(kindAnyDay); !slices.Equal(got, want) {
		t.Fatalf("any_day %v", got)
	}
	for _, r := range jobTable {
		if r.Kind == kindAnyDay && r.Zone != "" {
			t.Errorf("%s: an any-day job holds no zone", r.ID)
		}
	}
}

func TestTodayOnlyIsExactlyTheFourteen(t *testing.T) {
	t.Parallel()
	want := slices.Sorted(maps.Keys(wantTwelve))
	if got := idsOfKind(kindTodayOnly); !slices.Equal(got, want) {
		t.Fatalf("today_only %v", got)
	}
}

func TestListedOnlyIsExactlyTheEight(t *testing.T) {
	t.Parallel()
	want := []string{"option_ledger", "option_open", "option_open_jp", "option_open_plan", "option_open_plan_jp",
		"option_open_resolve", "option_open_resolve_gap", "option_open_resolve_jp"}
	if got := idsOfKind(kindListedOnly); !slices.Equal(got, want) {
		t.Fatalf("listed_only %v", got)
	}
	if r, _ := jobRowOf("option_ledger"); r.Reason != "caches_failures" {
		t.Fatalf("option_ledger reason %q", r.Reason)
	}
}

func TestEveryListedOnlyJobHasReasonAndSentence(t *testing.T) {
	t.Parallel()
	codes := []string{"no_lower_time_bound", "no_day_guard", "heavy_and_shares_cap", "caches_failures"}
	for _, r := range jobTable {
		if r.Kind != kindListedOnly {
			continue
		}
		if !slices.Contains(codes, r.Reason) || r.Text == "" || r.Needs == "" || r.runnable() {
			t.Errorf("%s: %+v", r.ID, r)
		}
	}
	if r, _ := jobRowOf("quote_snapshot"); r.Note == "" {
		t.Error("quote_snapshot has no note")
	}
}

func TestOnlyOptionEodJpIsInTheTokyoZone(t *testing.T) {
	t.Parallel()
	var tokyo []string
	for _, r := range jobTable {
		if r.Zone == "Asia/Tokyo" {
			tokyo = append(tokyo, r.ID)
		}
	}
	if !slices.Equal(tokyo, []string{"option_eod_jp", "calendar_hours_jp", "option_list_preopen_jp"}) {
		t.Fatalf("Tokyo jobs %v", tokyo)
	}
}

func TestFxRefreshRunValidatesPastDayWithForce(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	post := func(job, day, force string) (jobRun, bool) {
		r := &http.Request{PostForm: url.Values{"job": {job}, "day": {day}, "force": {force}}}
		return readJobRun(r, now)
	}
	run, ok := post("fx_refresh", "2026-11-10", "on")
	if !ok || !run.force || run.day != "2026-11-10" || run.job.ID != "fx_refresh" {
		t.Fatalf("past day with force: %+v %v", run, ok)
	}
	if _, ok := post("fx_refresh", "2026-11-14", "on"); ok {
		t.Error("a day after today was accepted")
	}
	if _, ok := post("compute_figures", "2026-11-13", "on"); ok {
		t.Error("force on a force-no job was accepted")
	}
}

func TestBarsRefreshRunValidatesPastDayWithForce(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	post := func(day string) (jobRun, bool) {
		r := &http.Request{PostForm: url.Values{"job": {"bars_refresh"}, "day": {day}, "force": {"on"}}}
		return readJobRun(r, now)
	}
	run, ok := post("2026-11-10")
	if !ok || !run.force || run.day != "2026-11-10" || run.job.ID != "bars_refresh" {
		t.Fatalf("past day with force: %+v %v", run, ok)
	}
	if _, ok := post("2026-11-14"); ok {
		t.Error("a day after today was accepted")
	}
}
