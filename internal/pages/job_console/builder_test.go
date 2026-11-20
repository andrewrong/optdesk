package job_console

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

var fixedNow = time.Date(2026, 11, 12, 14, 6, 0, 0, time.UTC)

type fakeReader struct {
	recent    []jobrequest.Row
	recentErr error
	byID      map[int64]jobrequest.Row
	getErr    error
}

func (f *fakeReader) Recent(_ context.Context, limit int) ([]jobrequest.Row, error) {
	if f.recentErr != nil {
		return nil, f.recentErr
	}
	if len(f.recent) > limit {
		return f.recent[:limit], nil
	}
	return f.recent, nil
}

func (f *fakeReader) Get(_ context.Context, id int64) (jobrequest.Row, bool, error) {
	if f.getErr != nil {
		return jobrequest.Row{}, false, f.getErr
	}
	r, ok := f.byID[id]
	return r, ok, nil
}

func (f *fakeReader) Close() error { return nil }

type page struct {
	Gaps     []map[string]any `json:"gaps"`
	Sentence string           `json:"sentence"`
	Form     map[string]any   `json:"form"`
	Jobs     []struct {
		ID, Kind, Today string
		ForceWhy        string  `json:"force_why"`
		WhyText         string  `json:"why_text"`
		NeedsText       string  `json:"needs_text"`
		Note            *string `json:"note"`
		Force           bool    `json:"force"`
	} `json:"jobs"`
	Requests []struct {
		ID        int64
		Job       string
		State     string
		StateText string `json:"state_text"`
		AtText    string `json:"at_text"`
		Reason    string
		Keys      []struct {
			Dataset, Subject, Outcome, Present, Reason string
			OutcomeText                                string `json:"outcome_text"`
		}
	} `json:"requests"`
	Selected       *int64 `json:"selected"`
	RefreshSeconds int    `json:"refresh_seconds"`
}

func schema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	name := "page.job_console.v1.json"
	f, err := os.Open(filepath.Join("..", "..", "..", "contracts", "schemas", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("https://optdesk.invalid/schemas/"+name, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("https://optdesk.invalid/schemas/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// build runs Build, validates the output against the schema and decodes it.
func build(t *testing.T, r jobrequest.Reader, form *Form, selected int64, jobs []JobInfo) page {
	t.Helper()
	out, err := Build(context.Background(), r, fixedNow, form, selected, jobs)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema(t).Validate(doc); err != nil {
		t.Fatalf("schema: %v\n%s", err, out)
	}
	var p page
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func row(id int64, state jobrequest.State, lines ...jobrequest.KeyLine) jobrequest.Row {
	return jobrequest.Row{
		ID: id, Request: jobrequest.Request{JobID: "compute_figures", Day: "2026-11-12"},
		RequestedAt: fixedNow.Add(-time.Minute), State: state, Lines: lines,
	}
}

func rowReason(id int64, state jobrequest.State, reason string, lines ...jobrequest.KeyLine) jobrequest.Row {
	r := row(id, state, lines...)
	r.Reason = reason
	return r
}

func line(group, status string) jobrequest.KeyLine {
	return jobrequest.KeyLine{Dataset: "figures", Subject: "US:AAPL", Group: group, Status: status}
}

func one(t *testing.T, r jobrequest.Row) page {
	t.Helper()
	return build(t, &fakeReader{recent: []jobrequest.Row{r}}, nil, 0, nil)
}

func TestEveryStoredLineHasOneOutcome(t *testing.T) {
	t.Parallel()
	due := time.Date(2026, 11, 12, 15, 30, 0, 0, time.UTC)
	mk := func(group, status, errc, reason, mark string, returned, usable int) jobrequest.KeyLine {
		l := line(group, status)
		l.ErrorCode, l.ReasonCode, l.Mark, l.Returned, l.Usable = errc, reason, mark, returned, usable
		return l
	}
	notDue := mk("not_due", "", "", "", "", 0, 0)
	notDue.DueAt = due
	cases := []struct {
		name            string
		l               jobrequest.KeyLine
		outcome, reason string
		present         string
	}{
		{"already_present", mk("already_present", "", "", "", "present", 0, 0), "already_present", "", presentNow},
		{"not_due", notDue, "waiting", "not_due", presentMissing},
		{"lease_held", mk("lease_held", "", "", "", "", 0, 0), "skipped", "run_in_progress", presentMissing},
		{"not_reached", mk("not_reached", "", "", "slot_near", "", 0, 0), "not_run", "slot_near", presentMissing},
		{"rerun_error", mk("rerun", "", "timeout", "", "present", 0, 0), "failed", "timeout", presentNow},
		{"rerun_no_value", mk("rerun", "", "", "no_value", "present", 0, 0), "nothing_found", "no_value", presentNow},
		{"rerun_usable", mk("rerun", "", "", "", "present", 5, 5), "unconfirmed", "rerun_unknown", presentNow},
		{"rerun_empty", mk("rerun", "", "", "", "present", 0, 0), "nothing_found", "no_value_or_empty", presentNow},
		{"ran_abandoned", mk("ran", "abandoned", "", "", "", 0, 0), "failed", "cut", presentMissing},
		{"ran_partial", mk("ran", "partial", "", "", "", 7, 3), "partial", "cut", presentMissing},
		{"ran_error", mk("ran", "done", "vendor_down", "", "", 0, 0), "failed", "vendor_down", presentMissing},
		{"ran_produced", mk("ran", "done", "", "", "present", 12, 12), "produced", "", presentNow},
		{"ran_unconfirmed", mk("ran", "done", "", "", "", 12, 12), "unconfirmed", "postcondition", presentMissing},
		{"ran_empty_before_close", mk("ran", "done", "", "empty_before_close", "", 0, 0), "nothing_found", "empty_before_close", presentMissing},
		{"ran_nothing", mk("ran", "done", "", "", "no_value", 0, 0), "nothing_found", "no_value_or_empty", presentNoValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := one(t, row(1, jobrequest.StateDone, c.l))
			k := p.Requests[0].Keys[0]
			if k.Outcome != c.outcome || k.Reason != c.reason || k.Present != c.present {
				t.Errorf("got outcome %q reason %q present %q, want %q %q %q", k.Outcome, k.Reason, k.Present, c.outcome, c.reason, c.present)
			}
			if k.OutcomeText == "" {
				t.Error("empty outcome text")
			}
		})
	}
	if len(cases) != 15 {
		t.Fatalf("%d cases, the table has fifteen rows", len(cases))
	}
}

func TestUnknownLineIsAnError(t *testing.T) {
	t.Parallel()
	_, err := Build(context.Background(), &fakeReader{recent: []jobrequest.Row{row(1, jobrequest.StateDone, line("mystery", ""))}}, fixedNow, nil, 0, nil)
	if err == nil {
		t.Fatal("unknown group accepted")
	}
}

func TestRerunThatFoundNothing(t *testing.T) {
	t.Parallel()
	l := line("rerun", "")
	l.Mark = "present"
	k := one(t, row(1, jobrequest.StateDone, l)).Requests[0].Keys[0]
	if k.Outcome != "nothing_found" || !strings.Contains(k.OutcomeText, zhRerunStays) {
		t.Errorf("got %q %q", k.Outcome, k.OutcomeText)
	}
}

func TestRerunNeverReadsProduced(t *testing.T) {
	t.Parallel()
	for _, usable := range []int{0, 1, 12} {
		rr := line("rerun", "")
		rr.Mark, rr.Returned, rr.Usable = "present", usable, usable
		k := one(t, row(1, jobrequest.StateDone, rr)).Requests[0].Keys[0]
		if k.Outcome == "produced" {
			t.Errorf("rerun with %d rows is produced", usable)
		}
		if usable > 0 && (k.Outcome != "unconfirmed" || k.Reason != "rerun_unknown" ||
			k.OutcomeText != zhOutUnconfirmed+": "+fmt.Sprintf(zhRerunUnknownFmt, usable)) {
			t.Errorf("rerun %d rows: %q %q %q", usable, k.Outcome, k.Reason, k.OutcomeText)
		}
	}
	ran := line("ran", "done")
	ran.Mark, ran.Returned, ran.Usable = "present", 12, 12
	k := one(t, row(1, jobrequest.StateDone, ran)).Requests[0].Keys[0]
	if k.Outcome != "produced" || k.OutcomeText != zhOutProduced+": "+fmt.Sprintf(zhRanReturnedFmt, 12) {
		t.Errorf("ran: %q %q", k.Outcome, k.OutcomeText)
	}
}

func TestNotDueLineShowsTime(t *testing.T) {
	t.Parallel()
	l := line("not_due", "")
	l.DueAt = time.Date(2026, 11, 12, 15, 30, 0, 0, time.UTC)
	p := one(t, rowReason(1, jobrequest.StateRefused, "not_due", l))
	if got := p.Requests[0].Keys[0].OutcomeText; got != zhOutWaiting+": "+fmt.Sprintf(zhLineRunsAtFmt, "15:30") {
		t.Errorf("line text %q", got)
	}
	if got := p.Requests[0].StateText; got != fmt.Sprintf(zhRefusedNotDueAtFmt, "15:30") {
		t.Errorf("state text %q", got)
	}
}

func TestEmptyBeforeCloseText(t *testing.T) {
	t.Parallel()
	l := line("ran", "done")
	l.ReasonCode = "empty_before_close"
	k := one(t, row(1, jobrequest.StateDone, l)).Requests[0].Keys[0]
	if k.OutcomeText != zhOutNothingFound+": "+zhRanEmptyBefore {
		t.Errorf("text %q", k.OutcomeText)
	}
}

func TestNothingWasProducedLine(t *testing.T) {
	t.Parallel()
	l := line("ran", "done")
	if got := one(t, rowReason(7, jobrequest.StateDone, "completed", l)).Sentence; !strings.Contains(got, fmt.Sprintf(zhNothingProducedFmt, 7, "compute_figures", "2026-11-12")) {
		t.Errorf("sentence %q", got)
	}
	l.Mark, l.Returned, l.Usable = "present", 3, 3
	if got := one(t, rowReason(7, jobrequest.StateDone, "completed", l)).Sentence; strings.Contains(got, fmt.Sprintf(zhNothingProducedFmt, 7, "compute_figures", "2026-11-12")) {
		t.Errorf("a produced line still says nothing: %q", got)
	}
	if got := one(t, rowReason(7, jobrequest.StateRefused, "no_plan")).Sentence; strings.Contains(got, fmt.Sprintf(zhNothingProducedFmt, 7, "compute_figures", "2026-11-12")) {
		t.Errorf("a refused request says nothing was produced: %q", got)
	}
}

func TestRequestStateText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		state  jobrequest.State
		reason string
		want   string
	}{
		{jobrequest.StateRunning, "", fmt.Sprintf(zhRunningSinceFmt, "2026-11-12 14:05 UTC")},
		{jobrequest.StateDone, "completed", zhStateDone},
		{jobrequest.StateRefused, "no_key", zhRefusedNoKey},
		{jobrequest.StateRefused, "not_due", zhRefusedNotDue},
		{jobrequest.StateRefused, "no_plan", zhRefusedNoPlan},
		{jobrequest.StateRefused, "left_out", zhRefusedLeftOut},
		{jobrequest.StateRefused, "not_runnable", zhRefusedNotRunnable},
		{jobrequest.StateRefused, "not_today", zhRefusedNotToday},
		{jobrequest.StateRefused, "job_running", zhRefusedJobRunning},
		{jobrequest.StateFailed, "slot_near", zhFailedSlotNear},
		{jobrequest.StateFailed, "lease_held", zhFailedLeaseHeld},
		{jobrequest.StateFailed, "plan_failed", zhFailedPlanFailed},
		{jobrequest.StateFailed, "error", zhFailedError},
		{jobrequest.StateFailed, "restart", zhFailedRestart},
		{jobrequest.StateInterrupted, "stopped", zhInterruptedStopped},
		{jobrequest.StateWithdrawn, "withdrawn", zhStateWithdrawn},
	}
	for _, c := range cases {
		r := rowReason(1, c.state, c.reason)
		r.StartedAt = fixedNow.Add(-time.Minute)
		if got := one(t, r).Requests[0].StateText; got != c.want {
			t.Errorf("%s %s: %q, want %q", c.state, c.reason, got, c.want)
		}
	}
}

func TestBrokerSilentFailedText(t *testing.T) {
	t.Parallel()
	r := rowReason(1, jobrequest.StateFailed, "broker_silent")
	r.StartedAt = fixedNow.Add(-time.Minute)
	if got := one(t, r).Requests[0].StateText; got != zhBrokerSilent {
		t.Errorf("text %q, want %q", got, zhBrokerSilent)
	}
}

func TestQueuedHeldText(t *testing.T) {
	t.Parallel()
	r := row(1, jobrequest.StateQueued)
	r.HoldJob, r.HoldAt = "quote_snapshot", time.Date(2026, 11, 12, 15, 30, 0, 0, time.UTC)
	if got := one(t, r).Requests[0].StateText; got != fmt.Sprintf(zhQueuedStartsAfterFmt, "15:30", "quote_snapshot") {
		t.Errorf("text %q", got)
	}
}

func TestQueuedWithdrawalPendingText(t *testing.T) {
	t.Parallel()
	r := row(1, jobrequest.StateQueued)
	r.WithdrawnAt = fixedNow
	got := one(t, r).Requests[0].StateText
	if got != zhQueuedWithdrawing {
		t.Errorf("text %q", got)
	}
	if !strings.Contains(got, "up to two hours") {
		t.Errorf("no two-hour bound: %q", got)
	}
}

func TestQueuedStaleText(t *testing.T) {
	t.Parallel()
	r := row(1, jobrequest.StateQueued)
	r.RequestedAt = fixedNow.Add(-3 * time.Minute)
	if got := one(t, r).Requests[0].StateText; got != zhQueuedStale {
		t.Errorf("old: %q", got)
	}
	if got := one(t, row(1, jobrequest.StateQueued)).Requests[0].StateText; got == zhQueuedStale {
		t.Errorf("fresh: %q", got)
	}
}

func TestInterruptedShowsRowsUnknown(t *testing.T) {
	t.Parallel()
	l := line("ran", "done")
	l.Mark, l.Returned, l.Usable = "present", 12, 12
	p := one(t, rowReason(1, jobrequest.StateInterrupted, "stopped", l))
	if p.Requests[0].StateText != zhInterruptedStopped {
		t.Errorf("text %q", p.Requests[0].StateText)
	}
	if len(p.Requests[0].Keys) != 1 || p.Requests[0].Keys[0].Outcome != "produced" {
		t.Errorf("the lines so far are not shown: %+v", p.Requests[0].Keys)
	}
}

func TestNilReaderGap(t *testing.T) {
	t.Parallel()
	p := build(t, nil, nil, 0, nil)
	if len(p.Gaps) != 1 || p.Gaps[0]["reason"] != "job_requests_unavailable" || len(p.Requests) != 0 {
		t.Errorf("gaps %v requests %d", p.Gaps, len(p.Requests))
	}
	if p.Form != nil {
		t.Errorf("form %v for a nil form", p.Form)
	}
}

func TestReaderErrorGivesUnavailableGap(t *testing.T) {
	t.Parallel()
	p := build(t, &fakeReader{recentErr: errors.New("dial tcp: postgres://u:p@h/db")}, nil, 0, nil)
	if len(p.Gaps) != 1 || p.Gaps[0]["reason"] != "job_requests_unavailable" || len(p.Requests) != 0 {
		t.Fatalf("gaps %v", p.Gaps)
	}
	if strings.Contains(fmt.Sprint(p.Gaps), "postgres") {
		t.Errorf("the error text leaked: %v", p.Gaps)
	}
}

func TestRefreshSeconds(t *testing.T) {
	t.Parallel()
	for state, want := range map[jobrequest.State]int{
		jobrequest.StateQueued: 5, jobrequest.StateRunning: 5, jobrequest.StateDone: 0,
		jobrequest.StateRefused: 0, jobrequest.StateFailed: 0, jobrequest.StateInterrupted: 0, jobrequest.StateWithdrawn: 0,
	} {
		if got := one(t, row(1, state)).RefreshSeconds; got != want {
			t.Errorf("%s: %d, want %d", state, got, want)
		}
	}
}

func TestListedOnlyJobHasReasonNeedsAndNoForm(t *testing.T) {
	t.Parallel()
	p := build(t, nil, nil, 0, []JobInfo{{
		ID: "option_open", Kind: "listed_only", Zone: "America/New_York",
		WhyCode: "no_lower_time_bound", WhyText: "a run before its slot would store pre-open prices",
		NeedsText: "a lower bound at the slot instant",
	}})
	j := p.Jobs[0]
	if j.Kind != "listed_only" || j.Force || j.WhyText == "" || j.NeedsText == "" {
		t.Errorf("job %+v", j)
	}
}

func TestForceNoJobHasNoSwitchAndShowsWhy(t *testing.T) {
	t.Parallel()
	p := build(t, nil, nil, 0, []JobInfo{
		{ID: "fred_series", Kind: "any_day", ForceWhy: "revision rows: run missing only"},
		{ID: "calendar_refresh", Kind: "today_only", ForceOK: true},
	})
	if p.Jobs[0].Force || p.Jobs[0].ForceWhy != "revision rows: run missing only" {
		t.Errorf("force-no job %+v", p.Jobs[0])
	}
	if !p.Jobs[1].Force || p.Jobs[1].ForceWhy != "" {
		t.Errorf("force-yes job %+v", p.Jobs[1])
	}
}

func TestNotASessionText(t *testing.T) {
	t.Parallel()
	got := one(t, rowReason(1, jobrequest.StateRefused, "no_key")).Requests[0].StateText
	if !strings.Contains(got, "not a session") {
		t.Errorf("text %q", got)
	}
}

func TestSlotNearText(t *testing.T) {
	t.Parallel()
	l := line("not_reached", "")
	l.ReasonCode = "slot_near"
	l.DueAt = time.Date(2026, 11, 12, 15, 30, 0, 0, time.UTC)
	p := one(t, rowReason(1, jobrequest.StateFailed, "slot_near", l))
	want := fmt.Sprintf(zhFailedSlotNearAtFmt, "15:30")
	if p.Requests[0].StateText != want {
		t.Errorf("text %q", p.Requests[0].StateText)
	}
	if k := p.Requests[0].Keys[0]; k.Outcome != "not_run" || k.Reason != "slot_near" {
		t.Errorf("line %+v", k)
	}
}

func TestTodayOnlyJobCarriesTodayInItsZone(t *testing.T) {
	t.Parallel()
	// 2026-11-12 14:06 UTC is 23:06 on the 12th in Tokyo and 09:06 in New York; at 16:00 UTC Tokyo is the 13th.
	late := time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)
	out, err := Build(context.Background(), nil, late, nil, 0, []JobInfo{
		{ID: "option_eod_jp", Kind: "today_only", Zone: "Asia/Tokyo"},
		{ID: "calendar_refresh", Kind: "today_only", Zone: "America/New_York"},
		{ID: "fred_series", Kind: "any_day"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var p page
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	got := []string{p.Jobs[0].Today, p.Jobs[1].Today, p.Jobs[2].Today}
	want := []string{"2026-11-13", "2026-11-12", "2026-11-12"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("job %d today %q, want %q", i, got[i], want[i])
		}
	}
	if _, err := Build(context.Background(), nil, late, nil, 0, []JobInfo{{ID: "x", Kind: "any_day", Zone: "Nowhere/Land"}}); err == nil {
		t.Error("an unknown zone is accepted")
	}
}

func TestNoteShownBesideForm(t *testing.T) {
	t.Parallel()
	form := &Form{PostPath: "/owner-job-console", Expires: "2026-11-12T15:06:00Z", MAC: "00"}
	p := build(t, nil, form, 0, []JobInfo{
		{ID: "quote_snapshot", Kind: "today_only", Note: "its three keys share one scheduled instant"},
		{ID: "fred_series", Kind: "any_day"},
	})
	if p.Jobs[0].Note == nil || *p.Jobs[0].Note != "its three keys share one scheduled instant" || p.Jobs[1].Note != nil {
		t.Errorf("notes %v %v", p.Jobs[0].Note, p.Jobs[1].Note)
	}
	if p.Form["post_path"] != "/owner-job-console" {
		t.Errorf("form %v", p.Form)
	}
}

func TestSelectedOlderThanTwenty(t *testing.T) {
	t.Parallel()
	var recent []jobrequest.Row
	for id := int64(100); id > 80; id-- {
		recent = append(recent, row(id, jobrequest.StateDone))
	}
	old := row(5, jobrequest.StateDone)
	r := &fakeReader{recent: recent, byID: map[int64]jobrequest.Row{5: old}}
	p := build(t, r, nil, 5, nil)
	if len(p.Requests) != 20 {
		t.Fatalf("%d requests", len(p.Requests))
	}
	if p.Requests[19].ID != 5 || p.Selected == nil || *p.Selected != 5 {
		t.Errorf("last %d selected %v", p.Requests[19].ID, p.Selected)
	}
	if p.Requests[0].ID != 100 {
		t.Errorf("newest first lost: %d", p.Requests[0].ID)
	}
	// One of the twenty newest is selected: nothing is added.
	if p := build(t, r, nil, 90, nil); len(p.Requests) != 20 || *p.Selected != 90 {
		t.Errorf("listed selected: %d requests, selected %v", len(p.Requests), p.Selected)
	}
}

func TestNoSuchRequestGap(t *testing.T) {
	t.Parallel()
	r := &fakeReader{recent: []jobrequest.Row{row(1, jobrequest.StateDone)}}
	p := build(t, r, nil, 99, nil)
	if len(p.Gaps) != 1 || p.Gaps[0]["reason"] != "no_such_request" || p.Selected != nil || len(p.Requests) != 1 {
		t.Errorf("gaps %v selected %v requests %d", p.Gaps, p.Selected, len(p.Requests))
	}
}

type fakeRuns struct {
	open   []jobrun.JobRun
	usuals []jobrun.UsualStat
}

func (f fakeRuns) Running(context.Context) ([]jobrun.JobRun, error) { return f.open, nil }
func (f fakeRuns) Usuals(context.Context) ([]jobrun.UsualStat, error) {
	return f.usuals, nil
}

type runsPage struct {
	Running []struct {
		Line           string
		Job            string
		StartedAt      string `json:"started_at"`
		KeysDone       int    `json:"keys_done"`
		KeysTotal      int    `json:"keys_total"`
		Step           string
		ElapsedMinutes int  `json:"elapsed_minutes"`
		UsualMinutes   int  `json:"usual_minutes"`
		Slow           bool `json:"slow"`
	}
	Requests []struct {
		ID         int64
		StateText  string                             `json:"state_text"`
		WaitingFor *struct{ Line, Job, Since string } `json:"waiting_for"`
	}
}

// buildRuns runs BuildWithRuns, validates against the schema and decodes.
func buildRuns(t *testing.T, rows []jobrequest.Row, runs Runs) runsPage {
	t.Helper()
	out, err := BuildWithRuns(context.Background(), &fakeReader{recent: rows}, fixedNow, nil, 0, nil, runs)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema(t).Validate(doc); err != nil {
		t.Fatalf("schema: %v\n%s", err, out)
	}
	var p runsPage
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func fixedFigures(figs ...jobrunapi.Figure) jobrunapi.FiguresFunc {
	return func(time.Time, []jobrunapi.Input, []jobrunapi.Usual) []jobrunapi.Figure { return figs }
}

var runStart = time.Date(2026, 11, 12, 13, 30, 0, 0, time.UTC)

func oneRun(job string) fakeRuns {
	return fakeRuns{open: []jobrun.JobRun{{ID: "r1", JobID: job, StartedAt: runStart, KeysDone: 12, KeysTotal: 40, Step: "figures/US:AAPL"}}}
}

func TestRunningNowLine(t *testing.T) {
	t.Parallel()
	f := jobrunapi.Figure{JobID: "compute_figures", ElapsedMinutes: 36, UsualMinutes: 30, HasUsual: true, KeysDone: 12, KeysTotal: 40, Step: "figures/US:AAPL"}
	p := buildRuns(t, nil, Runs{Reader: oneRun("compute_figures"), Figures: fixedFigures(f)})
	if len(p.Running) != 1 {
		t.Fatalf("running = %d", len(p.Running))
	}
	r := p.Running[0]
	for _, want := range []string{"compute_figures", "started 13:30 UTC", "已完成 12 / 40", "figures/US:AAPL", "跑了 36 分钟，平常 30 分钟"} {
		if !strings.Contains(r.Line, want) {
			t.Errorf("line %q lacks %q", r.Line, want)
		}
	}
	if r.Slow || r.StartedAt != "2026-11-12T13:30:00Z" || r.KeysDone != 12 || r.KeysTotal != 40 {
		t.Errorf("fields: %+v", r)
	}
	if p := buildRuns(t, nil, Runs{}); len(p.Running) != 0 {
		t.Errorf("zero Runs gave %d lines", len(p.Running))
	}
}

func TestWaitingForLine(t *testing.T) {
	t.Parallel()
	waiting := row(7, jobrequest.StateQueued)
	waiting.HoldJob = "compute_figures"
	waiting.HoldAt = runStart
	other := row(8, jobrequest.StateQueued)
	other.Request.JobID = "other_job"
	other.HoldJob = "compute_figures"
	other.HoldAt = runStart
	runs := Runs{Reader: oneRun("compute_figures"), Figures: fixedFigures(jobrunapi.Figure{JobID: "compute_figures"})}
	p := buildRuns(t, []jobrequest.Row{waiting, other}, runs)
	w := p.Requests[0].WaitingFor
	if w == nil || w.Job != "compute_figures" || w.Since != "2026-11-12T13:30:00Z" || !strings.Contains(w.Line, "已在运行，自 13:30 UTC") {
		t.Fatalf("waiting_for = %+v", w)
	}
	if p.Requests[0].StateText != w.Line {
		t.Errorf("state text %q", p.Requests[0].StateText)
	}
	if p.Requests[1].WaitingFor != nil {
		t.Errorf("a request holding for another job's slot got waiting_for")
	}
	p = buildRuns(t, []jobrequest.Row{waiting}, Runs{Reader: fakeRuns{}, Figures: fixedFigures()})
	if p.Requests[0].WaitingFor != nil || p.Requests[0].StateText != fmt.Sprintf(zhQueuedStartsAfterFmt, "13:30", "compute_figures") {
		t.Errorf("no open run: %+v %q", p.Requests[0].WaitingFor, p.Requests[0].StateText)
	}
}

func TestSlowRunLine(t *testing.T) {
	t.Parallel()
	f := jobrunapi.Figure{JobID: "compute_figures", ElapsedMinutes: 95, UsualMinutes: 20, HasUsual: true, Slow: true, KeysDone: 3, KeysTotal: 40}
	waiting := row(7, jobrequest.StateQueued)
	waiting.HoldJob, waiting.HoldAt = "compute_figures", runStart
	p := buildRuns(t, []jobrequest.Row{waiting}, Runs{Reader: oneRun("compute_figures"), Figures: fixedFigures(f)})
	r := p.Running[0]
	if !r.Slow || !strings.Contains(r.Line, "跑了 95 分钟，平常 20 分钟") || !strings.Contains(r.Line, "slow") {
		t.Errorf("slow run: %+v", r)
	}
	if len(p.Requests) != 1 || p.Requests[0].WaitingFor == nil {
		t.Errorf("slow run changed the requests: %+v", p.Requests)
	}
}

func TestNoHistoryLine(t *testing.T) {
	t.Parallel()
	f := jobrunapi.Figure{JobID: "compute_figures", ElapsedMinutes: 4, KeysDone: 1, KeysTotal: 2}
	p := buildRuns(t, nil, Runs{Reader: oneRun("compute_figures"), Figures: fixedFigures(f)})
	if l := p.Running[0].Line; !strings.Contains(l, "跑了 4 分钟，没有历史") || strings.Contains(l, "平常") {
		t.Errorf("line %q", l)
	}
}

func TestRunningLinesDoNoArithmetic(t *testing.T) {
	t.Parallel()
	f := jobrunapi.Figure{JobID: "compute_figures", ElapsedMinutes: 777, UsualMinutes: 5, HasUsual: true, KeysDone: 9, KeysTotal: 2, Step: "s"}
	p := buildRuns(t, nil, Runs{Reader: oneRun("compute_figures"), Figures: fixedFigures(f)})
	r := p.Running[0]
	if r.ElapsedMinutes != 777 || r.UsualMinutes != 5 || r.Slow || r.KeysDone != 9 || r.KeysTotal != 2 || r.Step != "s" {
		t.Errorf("figures not copied as given: %+v", r)
	}
}

func jsonString(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestBuildWithoutRunsSameBytes(t *testing.T) {
	t.Parallel()
	want := `{
  "schema": "job_console",
  "schema_version": 1,
  "generated_at": "2026-11-12T14:06:00Z",
  "title": ` + jsonString(titleText) + `,
  "sentence": ` + jsonString(sentenceText) + `,
  "form": null,
  "gaps": [
    {
      "kind": "gap",
      "reason": "job_requests_unavailable",
      "text": ` + jsonString(unavailableText) + `
    }
  ],
  "jobs": [],
  "requests": [],
  "selected": null,
  "refresh_seconds": 0
}`
	got, err := Build(context.Background(), nil, fixedNow, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("Build output changed:\n%s", got)
	}
}

func TestWaitingForOnlyForOwnJob(t *testing.T) {
	t.Parallel()
	own := row(7, jobrequest.StateQueued)
	own.HoldJob, own.HoldAt = "compute_figures", runStart
	other := row(8, jobrequest.StateQueued)
	other.Request.JobID = "other_job"
	other.HoldJob, other.HoldAt = "compute_figures", runStart // holds for a slot of another job
	idle := row(9, jobrequest.StateQueued)                    // no hold at all
	runs := Runs{Reader: oneRun("compute_figures"), Figures: fixedFigures(jobrunapi.Figure{JobID: "compute_figures"})}
	p := buildRuns(t, []jobrequest.Row{own, other, idle}, runs)
	if p.Requests[0].WaitingFor == nil || p.Requests[1].WaitingFor != nil || p.Requests[2].WaitingFor != nil {
		t.Errorf("waiting_for: %+v %+v %+v", p.Requests[0].WaitingFor, p.Requests[1].WaitingFor, p.Requests[2].WaitingFor)
	}
}

func TestRunningPairsFigureWithItsOwnRun(t *testing.T) {
	t.Parallel()
	a := time.Date(2026, 11, 12, 13, 0, 0, 0, time.UTC)
	b := time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	reader := fakeRuns{open: []jobrun.JobRun{{JobID: "a", StartedAt: a}, {JobID: "b", StartedAt: b}}}
	figs := fixedFigures(jobrunapi.Figure{JobID: "b"}, jobrunapi.Figure{JobID: "a"})
	p := buildRuns(t, nil, Runs{Reader: reader, Figures: figs})
	for _, r := range p.Running {
		if (r.Job == "a") != (r.StartedAt == "2026-11-12T13:00:00Z") {
			t.Errorf("job %s shows start %s", r.Job, r.StartedAt)
		}
	}
}

func TestConsoleTextsAreChineseWithEnglish(t *testing.T) {
	t.Parallel()
	han := regexp.MustCompile(`\p{Han}`)
	gloss := regexp.MustCompile(`\([A-Za-z]`)
	check := func(t *testing.T, what, text string) {
		t.Helper()
		if !han.MatchString(text) || !gloss.MatchString(text) {
			t.Errorf("%s %q lacks Chinese or a glossed English meaning", what, text)
		}
	}
	due := time.Date(2026, 11, 12, 15, 30, 0, 0, time.UTC)
	mk := func(group, status, errc, reason, mark string, returned, usable int) jobrequest.KeyLine {
		l := line(group, status)
		l.ErrorCode, l.ReasonCode, l.Mark, l.Returned, l.Usable = errc, reason, mark, returned, usable
		return l
	}
	notDue := mk("not_due", "", "", "", "", 0, 0)
	notDue.DueAt = due
	lines := []jobrequest.KeyLine{
		mk("already_present", "", "", "", "present", 0, 0), notDue,
		mk("lease_held", "", "", "", "", 0, 0), mk("not_reached", "", "", "slot_near", "", 0, 0),
		mk("rerun", "", "timeout", "", "present", 0, 0), mk("rerun", "", "", "no_value", "present", 0, 0),
		mk("rerun", "", "", "", "present", 5, 5), mk("rerun", "", "", "", "present", 0, 0),
		mk("ran", "abandoned", "", "", "", 0, 0), mk("ran", "partial", "", "", "", 7, 3),
		mk("ran", "done", "vendor_down", "", "", 0, 0), mk("ran", "done", "", "", "present", 12, 12),
		mk("ran", "done", "", "", "", 12, 12), mk("ran", "done", "", "empty_before_close", "", 0, 0),
		mk("ran", "done", "", "", "no_value", 0, 0),
	}
	if len(lines) != 15 {
		t.Fatalf("%d lines, the table has fifteen rows", len(lines))
	}
	for i, l := range lines {
		k := one(t, row(1, jobrequest.StateDone, l)).Requests[0].Keys[0]
		check(t, "outcome_text", k.OutcomeText)
		if k.OutcomeText == k.Outcome {
			t.Errorf("line %d shows the bare code %q", i, k.Outcome)
		}
		if !strings.HasPrefix(k.OutcomeText, outcomeName(k.Outcome)+": ") {
			t.Errorf("line %d: %q does not start with the name of %q", i, k.OutcomeText, k.Outcome)
		}
	}
	states := []struct {
		state  jobrequest.State
		reason string
	}{
		{jobrequest.StateRunning, ""}, {jobrequest.StateDone, "completed"},
		{jobrequest.StateRefused, "no_key"}, {jobrequest.StateRefused, "not_due"}, {jobrequest.StateRefused, "no_plan"},
		{jobrequest.StateRefused, "left_out"}, {jobrequest.StateRefused, "not_runnable"},
		{jobrequest.StateRefused, "not_today"}, {jobrequest.StateRefused, "job_running"},
		{jobrequest.StateFailed, "slot_near"}, {jobrequest.StateFailed, "lease_held"},
		{jobrequest.StateFailed, "plan_failed"}, {jobrequest.StateFailed, "error"},
		{jobrequest.StateFailed, "restart"}, {jobrequest.StateFailed, "broker_silent"},
		{jobrequest.StateInterrupted, "stopped"}, {jobrequest.StateWithdrawn, "withdrawn"},
		{jobrequest.StateRefused, "other"}, {jobrequest.StateFailed, ""}, {jobrequest.StateInterrupted, "other"},
		{jobrequest.StateQueued, ""},
	}
	for _, c := range states {
		r := rowReason(1, c.state, c.reason)
		r.StartedAt = fixedNow.Add(-time.Minute)
		check(t, string(c.state)+" "+c.reason, one(t, r).Requests[0].StateText)
	}
	for _, s := range []jobrequest.State{jobrequest.StateQueued, jobrequest.StateRunning, jobrequest.StateDone,
		jobrequest.StateRefused, jobrequest.StateFailed, jobrequest.StateInterrupted, jobrequest.StateWithdrawn} {
		check(t, "state name", stateName(s))
	}
}
