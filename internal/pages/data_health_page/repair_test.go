package data_health_page

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

var testForm = &Form{PostPath: "/owner-job-console", Expires: "x", MAC: "y"}

func utc(day string, h, m int) time.Time {
	d, _ := time.Parse("2006-01-02", day)
	return d.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

func one(state string, day contracts.Day, job *Job, form *Form, now time.Time, today contracts.Day) Cell {
	cs := []Cell{{Day: day, State: state, MissingKeys: []string{}}}
	Apply(cs, job, form, now, today)
	return cs[0]
}

func TestRepairVerdicts(t *testing.T) {
	t.Parallel()
	anyDay := &Job{ID: "fred_series", Kind: KindAnyDay}
	todayOnly := &Job{ID: "option_eod", Kind: KindTodayOnly, Zone: "America/New_York", WhyText: "the broker answers for the present only"}
	listed := &Job{ID: "option_open", Kind: KindListedOnly, WhyText: "never run from the console"}
	now := utc("2026-03-10", 18, 0) // Tuesday
	today := contracts.Day("2026-03-10")
	past := contracts.Day("2026-03-05")
	cases := []struct {
		name   string
		state  string
		day    contracts.Day
		job    *Job
		form   *Form
		repair string
		why    string
	}{
		{"ok", "ok", past, anyDay, testForm, RepairNone, ""},
		{"no_value", "no_value", past, anyDay, testForm, RepairNone, ""},
		{"missing any_day", "missing", past, anyDay, testForm, RepairButton, ""},
		{"expired any_day", "expired", past, anyDay, testForm, RepairButton, ""},
		{"not_run any_day", StateNotRun, past, anyDay, testForm, RepairButton, ""},
		{"not_run today_only past", StateNotRun, past, todayOnly, testForm, RepairNotRefetchable, todayOnly.WhyText},
		{"not_run today_only today", StateNotRun, today, todayOnly, testForm, RepairButton, ""},
		{"not_run no job", StateNotRun, past, nil, testForm, RepairNotRefetchable, lostText},
		{"not_run no form", StateNotRun, past, anyDay, nil, RepairNone, ""},
		{"today_only today", "missing", today, todayOnly, testForm, RepairButton, ""},
		{"today_only past", "missing", past, todayOnly, testForm, RepairNotRefetchable, todayOnly.WhyText},
		{"listed_only", "missing", today, listed, testForm, RepairNotRefetchable, listed.WhyText},
		{"no job", "missing", past, nil, testForm, RepairNotRefetchable, lostText},
		{"no form", "missing", past, anyDay, nil, RepairNone, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := one(tc.state, tc.day, tc.job, tc.form, now, today)
			if c.Repair != tc.repair || c.WhyText != tc.why {
				t.Fatalf("got %q %q, want %q %q", c.Repair, c.WhyText, tc.repair, tc.why)
			}
		})
	}
	// Friday cell, Now Saturday
	c := one("missing", "2026-03-06", todayOnly, testForm, utc("2026-03-07", 15, 0), "2026-03-07")
	if c.Repair != RepairNotRefetchable {
		t.Fatalf("friday cell on saturday: %q", c.Repair)
	}
}

func TestNotRunFollowsTheVerdictOfMissing(t *testing.T) {
	t.Parallel()
	now := utc("2026-03-10", 18, 0)
	today := contracts.Day("2026-03-10")
	jobs := map[string]*Job{
		"any_day":      {ID: "a", Kind: KindAnyDay},
		"today_only":   {ID: "t", Kind: KindTodayOnly, Zone: "America/New_York", WhyText: "present only", DueAt: "09:00"},
		"before slot":  {ID: "b", Kind: KindTodayOnly, Zone: "America/New_York", DueAt: "23:00"},
		"listed_only":  {ID: "l", Kind: KindListedOnly, WhyText: "never run"},
		"present_only": {ID: "p", Kind: KindAnyDay, PresentOnly: true},
		"no job":       nil,
	}
	for name, job := range jobs {
		for _, day := range []contracts.Day{"2026-03-05", today} {
			t.Run(name+" "+string(day), func(t *testing.T) {
				t.Parallel()
				m := one("missing", day, job, testForm, now, today)
				n := one(StateNotRun, day, job, testForm, now, today)
				if m.Repair != n.Repair || m.WhyText != n.WhyText || m.DueText != n.DueText {
					t.Fatalf("missing %+v, not_run %+v", m, n)
				}
			})
		}
	}
}

func TestGapDaysCountsNotRun(t *testing.T) {
	t.Parallel()
	now := utc("2026-03-10", 18, 0)
	cells := func(job *Job) []Cell {
		cs := []Cell{{Day: "2026-03-05", State: StateNotRun, MissingKeys: []string{}}}
		Apply(cs, job, testForm, now, "2026-03-10")
		return cs
	}
	if g := GapDays(cells(&Job{ID: "a", Kind: KindAnyDay})); len(g) != 1 || g[0] != "2026-03-05" {
		t.Fatalf("any_day: %v", g)
	}
	todayOnly := &Job{ID: "t", Kind: KindTodayOnly, Zone: "America/New_York", WhyText: "present only"}
	if g := GapDays(cells(todayOnly)); len(g) != 0 {
		t.Fatalf("today_only past: %v", g)
	}
	if g := GapDays(cells(&Job{ID: "l", Kind: KindListedOnly, WhyText: "never"})); len(g) != 0 {
		t.Fatalf("listed_only: %v", g)
	}
}

func TestRepairAllCountsNotRunDays(t *testing.T) {
	t.Parallel()
	cells := []Cell{{Day: "2026-03-05", State: StateNotRun, MissingKeys: []string{}}}
	Apply(cells, &Job{ID: "a", Kind: KindAnyDay}, testForm, utc("2026-03-10", 18, 0), "2026-03-10")
	if RepairAll(GapDays(cells), testForm) != RepairButton {
		t.Fatal("repair_all is not button")
	}
}

func TestPresentOnlyPastDay(t *testing.T) {
	t.Parallel()
	job := &Job{ID: "earnings_refresh", Kind: KindAnyDay, PresentOnly: true}
	now := utc("2026-03-10", 18, 0)
	past := one("missing", "2026-03-05", job, testForm, now, "2026-03-10")
	if past.Repair != RepairNotRefetchable || past.WhyText != lostText {
		t.Fatalf("past: %+v", past)
	}
	if c := one("missing", "2026-03-10", job, testForm, now, "2026-03-10"); c.Repair != RepairButton {
		t.Fatalf("today: %+v", c)
	}
}

func TestTodayBeforeSlot(t *testing.T) {
	t.Parallel()
	job := &Job{ID: "option_eod", Kind: KindTodayOnly, Zone: "America/New_York", DueAt: "11:00"}
	today := contracts.Day("2026-01-12")
	// 10:00 EST = 15:00 UTC
	before := one("missing", today, job, testForm, utc("2026-01-12", 15, 0), today)
	if before.Repair != RepairNone || before.DueText != "due at 11:00 EST" {
		t.Fatalf("before: %+v", before)
	}
	at := one("missing", today, job, testForm, utc("2026-01-12", 16, 0), today)
	if at.Repair != RepairButton || at.DueText != "" {
		t.Fatalf("at: %+v", at)
	}
	noSlot := &Job{ID: "x", Kind: KindTodayOnly, Zone: "America/New_York"}
	if c := one("missing", today, noSlot, testForm, utc("2026-01-12", 15, 0), today); c.DueText != "" || c.Repair != RepairButton {
		t.Fatalf("no slot: %+v", c)
	}
}

func TestGapDaysAndRepairAll(t *testing.T) {
	t.Parallel()
	job := &Job{ID: "fred_series", Kind: KindAnyDay}
	now := utc("2026-03-10", 18, 0)
	cells := []Cell{
		{Day: "2026-03-04", State: "expired", MissingKeys: []string{}},
		{Day: "2026-03-03", State: "missing", MissingKeys: []string{}},
		{Day: "2026-03-05", State: "ok", MissingKeys: []string{}},
		{Day: "2026-03-06", State: "no_value", MissingKeys: []string{}},
		{Day: "2026-03-07", State: StateNotRun, MissingKeys: []string{}},
	}
	Apply(cells, job, testForm, now, "2026-03-10")
	gaps := GapDays(cells)
	if len(gaps) != 3 || gaps[0] != "2026-03-03" || gaps[1] != "2026-03-04" || gaps[2] != "2026-03-07" {
		t.Fatalf("gaps %v", gaps)
	}
	if RepairAll(gaps, testForm) != RepairButton || RepairAll(gaps, nil) != RepairNone || RepairAll(nil, testForm) != RepairNone {
		t.Fatal("repair_all")
	}
	Apply(cells, job, nil, now, "2026-03-10")
	if g := GapDays(cells); len(g) != 0 {
		t.Fatalf("no form: %v", g)
	}
}

func TestSharedDatasetJPKeysFollowTheJPJob(t *testing.T) {
	t.Parallel()
	jobs := []Job{
		{ID: "option_candidates", Kind: KindAnyDay, Datasets: []string{"candidates.option_candidate"}, Calendar: "XNYS"},
		{ID: "option_candidates_jp", Kind: KindTodayOnly, Zone: "Asia/Tokyo", WhyText: "JP candidates are built for today only",
			Datasets: []string{"candidates.option_candidate"}, Calendar: "XTKS"},
	}
	now := utc("2026-11-24", 3, 0) // 12:00 in Tokyo
	today := contracts.Day("2026-11-24")
	jp := []string{"edinet:E00776", "edinet:E01914", "edinet:E02126", "edinet:E02367", "edinet:E02529", "edinet:E03847", "edinet:E04498"}
	cells := []Cell{
		{Day: "2026-11-05", State: "missing", MissingKeys: jp},
		{Day: "2026-11-24", State: "missing", MissingKeys: jp},
		{Day: "2026-11-04", State: "missing", MissingKeys: []string{"cik:0000320193"}},
		{Day: "2026-11-03", State: "missing", MissingKeys: append([]string{"cik:0000320193"}, jp...)},
	}
	applyDataset(cells, jobs, "candidates.option_candidate", testForm, now, today)
	if c := cells[0]; c.Repair != RepairNotRefetchable || c.WhyText == "" {
		t.Fatalf("past JP: %+v", c)
	}
	if c := cells[1]; c.Repair != RepairButton {
		t.Fatalf("today JP: %+v", c)
	}
	if c := cells[2]; c.Repair != RepairButton {
		t.Fatalf("past cik: %+v", c)
	}
	if c := cells[3]; c.Repair != RepairButton {
		t.Fatalf("past mixed: %+v", c)
	}
	gaps := GapDays(cells)
	if len(gaps) != 3 || gaps[0] != "2026-11-03" || gaps[1] != "2026-11-04" || gaps[2] != "2026-11-24" {
		t.Fatalf("gaps %v", gaps)
	}
}
