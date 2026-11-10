package main

import (
	"context"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { checkServices["scheduler"] = checkScheduler }

const (
	schedDays      = 7 // calendar days through today
	schedDayLayout = "2006-01-02"
)

// checkScheduler is `ot check scheduler` (docs/design/local-e2e.md Decisions 4a to 4c): one line per dataset window of
// the last 7 calendar days from GapReader.Expectations, one per day without a row, one per open gap from
// GapReader.OpenGaps, and the no_view lines for what no reader gives. It reads through the gap reader only.
func checkScheduler(ctx context.Context, e *checkEnv) {
	zone := e.a.profile.Zone
	if zone == nil {
		e.c.emit(Result{Status: statusFail, Check: "scheduler", Reason: zoneEnvName + " is not set"})
		return
	}
	g, err := e.openGaps(ctx)
	if err != nil {
		e.c.emit(Result{Status: statusFail, Check: "scheduler.window", Reason: err.Error()})
		return
	}
	if g == nil {
		e.c.emit(Result{Status: statusFail, Check: "scheduler.window", Reason: "no gap reader"})
		return
	}
	defer func() { _ = g.Close() }()

	now := e.a.deps.newFwClock().Now()
	to := now.In(zone)
	from := to.AddDate(0, 0, -(schedDays - 1))
	exps, err := g.Expectations(ctx, contracts.Day(from.Format(schedDayLayout)), contracts.Day(to.Format(schedDayLayout)))
	if err != nil {
		e.c.emit(Result{Status: statusFail, Check: "scheduler.window", Reason: "read expectations failed: " + err.Error()})
	} else {
		for _, r := range windowResults(now, from, exps) {
			e.c.emit(r)
		}
	}
	gaps, err := g.OpenGaps(ctx)
	if err != nil {
		e.c.emit(Result{Status: statusFail, Check: "scheduler.gap", Reason: "read open gaps failed: " + err.Error()})
	} else {
		for _, r := range gapResults(gaps) {
			e.c.emit(r)
		}
	}
	for _, r := range noViewResults(exps) {
		e.c.emit(r)
	}
}

func checkName(kind, dataset string) string {
	if dataset == "" {
		return "scheduler." + kind
	}
	return "scheduler." + kind + "." + dataset
}

// windowResults is one line per expectation row and one per day of the range without a row (Decision 4a).
func windowResults(now, from time.Time, exps []store.Expectation) []Result {
	byDay := map[string][]store.Expectation{}
	for _, x := range exps {
		byDay[string(x.ExpectedDay)] = append(byDay[string(x.ExpectedDay)], x)
	}
	var out []Result
	for i := 0; i < schedDays; i++ {
		d := from.AddDate(0, 0, i)
		day := d.Format(schedDayLayout)
		rows := byDay[day]
		if len(rows) == 0 {
			out = append(out, noRowResult(d, byDay))
			continue
		}
		for _, x := range rows {
			out = append(out, expectationResult(now, day, x))
		}
	}
	// Rows outside the range the reader was asked for are not hidden.
	var extra []string
	for day := range byDay {
		if day < from.Format(schedDayLayout) || day > from.AddDate(0, 0, schedDays-1).Format(schedDayLayout) {
			extra = append(extra, day)
		}
	}
	sort.Strings(extra)
	for _, day := range extra {
		for _, x := range byDay[day] {
			out = append(out, expectationResult(now, day, x))
		}
	}
	return out
}

func expectationResult(now time.Time, day string, x store.Expectation) Result {
	name := checkName("window", x.DatasetID)
	switch {
	case x.Closed:
		return Result{Status: statusPass, Check: name, Day: day, Reason: "(nothing planned)"}
	case x.ExpectedDay == "":
		return Result{Status: statusFail, Check: name, Day: day, Reason: "no expectation"}
	case x.PresentDay == x.ExpectedDay:
		return Result{Status: statusPass, Check: name, Day: day, Reason: "present"}
	case x.DueBy.After(now):
		return Result{Status: statusExplained, Check: name, Day: day, Reason: "window_open"}
	}
	reason := "missing: present day " + dayOrNone(x.PresentDay) + " is before the expected day"
	if x.Gap != "" {
		reason += ", gap " + x.Gap
	}
	return Result{Status: statusFail, Check: name, Day: day, Reason: reason}
}

func dayOrNone(d contracts.Day) string {
	if d == "" {
		return "none"
	}
	return string(d)
}

// noRowResult is the line of a day with no row: a weekend day, or a day between two days with rows, is
// non_session_day; any other day is FAIL, so a dead scheduler does not read green.
func noRowResult(d time.Time, byDay map[string][]store.Expectation) Result {
	day := d.Format(schedDayLayout)
	name := "scheduler.window"
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return Result{Status: statusExplained, Check: name, Day: day, Reason: "non_session_day"}
	}
	var before, after bool
	for k := range byDay {
		before = before || k < day
		after = after || k > day
	}
	if before && after {
		return Result{Status: statusExplained, Check: name, Day: day, Reason: "non_session_day"}
	}
	return Result{Status: statusFail, Check: name, Day: day, Reason: "no window rows"}
}

// gapResults is one line per open gap: EXPLAINED when its reason is one of allowedReasons, FAIL otherwise.
func gapResults(gaps []store.Gap) []Result {
	var out []Result
	for _, g := range gaps {
		name := checkName("gap", g.DatasetID)
		day := string(g.Day)
		switch {
		case g.Reason == "":
			out = append(out, Result{Status: statusFail, Check: name, Day: day, Reason: "open gap with no reason (job " + g.JobID + ")"})
		case explainedReasonAllowed(g.Reason):
			out = append(out, Result{Status: statusExplained, Check: name, Day: day, Reason: g.Reason})
		default:
			out = append(out, Result{Status: statusFail, Check: name, Day: day, Reason: g.Reason})
		}
	}
	return out
}

// noViewResults are the facts no readable view gives (Views list of the design).
func noViewResults(exps []store.Expectation) []Result {
	cursor := ""
	for _, x := range exps {
		if x.Closed || x.ExpectedDay == "" || x.PresentDay == x.ExpectedDay {
			continue
		}
		if cursor == "" || string(x.ExpectedDay) < cursor {
			cursor = string(x.ExpectedDay)
		}
	}
	return []Result{
		{Status: statusExplained, Check: "scheduler.cursor", Day: cursor, Reason: "no_view: the cursor is in ops.scheduler_state; the oldest open expected day is shown"},
		{Status: statusExplained, Check: "scheduler.keys", Reason: "no_view: the expected key list is in ops.capture_expected_key; the check is dataset-level"},
		{Status: statusExplained, Check: "scheduler.runs", Reason: "no_view: run rows and reasons are in ops.run; only the gap reason is shown"},
	}
}
