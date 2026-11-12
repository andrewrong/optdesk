package main

import (
	"maps"
	"slices"
	"testing"
	"time"
)

// wantTwelve is the test's own list of the today-only jobs with their zones (Decision 6); no zone is read
// out of a job, because most jobs carry none in Schedule().
var wantTwelve = map[string]string{
	"account_capture":     "America/New_York",
	"position_marks":      "America/New_York",
	"earnings_refresh":    "America/New_York",
	"dividend_capture":    "America/New_York",
	"option_eod":          "America/New_York",
	"option_eod_jp":       "Asia/Tokyo",
	"quote_snapshot":      "America/New_York",
	"calendar_refresh":    "America/New_York",
	"underlying_resolve":  "America/New_York",
	"fx_refresh":          "America/New_York",
	"iv_history":          "America/New_York",
	"bars_refresh":        "America/New_York",
	"option_list_preopen": "America/New_York",
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
	if !slices.Equal(got, want) || len(got) != 25 {
		t.Fatalf("table %v\nwired %v", got, want)
	}
}

func TestForceYesIsExactlyCalendarRefresh(t *testing.T) {
	t.Parallel()
	var yes []string
	for _, r := range jobTable {
		if r.Force {
			yes = append(yes, r.ID)
		}
	}
	if !slices.Equal(yes, []string{"calendar_refresh"}) {
		t.Fatalf("force yes: %v", yes)
	}
}

func TestEveryForceNoJobHasReason(t *testing.T) {
	t.Parallel()
	codes := []string{"moment_mix", "append_only", "revision_rows", "attempt_rows", "epoch_and_caps", "new_run_id", "quota", "not_proved"}
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
	if n != 16 {
		t.Fatalf("%d force-no jobs, want 16", n)
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

func TestAnyDayIsExactlyTheFour(t *testing.T) {
	t.Parallel()
	want := []string{"compute_figures", "fred_series", "fundamentals", "option_candidates"}
	if got := idsOfKind(kindAnyDay); !slices.Equal(got, want) {
		t.Fatalf("any_day %v", got)
	}
	for _, r := range jobTable {
		if r.Kind == kindAnyDay && r.Zone != "" {
			t.Errorf("%s: an any-day job holds no zone", r.ID)
		}
	}
}

func TestTodayOnlyIsExactlyTheTwelve(t *testing.T) {
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
	if !slices.Equal(tokyo, []string{"option_eod_jp"}) {
		t.Fatalf("Tokyo jobs %v", tokyo)
	}
}
