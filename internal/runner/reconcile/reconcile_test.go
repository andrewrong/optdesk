package reconcile

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

var (
	_ runner.NewFunc         = New
	_ runner.NewWithRuleFunc = NewWithRule
)

var (
	keyA = contracts.ExpectedKey{DatasetID: "ds", SubjectKey: "a"}
	keyB = contracts.ExpectedKey{DatasetID: "ds", SubjectKey: "b"}
)

func oneDay() fakeCalendar {
	return fakeCalendar{sessions: []clock.Session{
		{CalendarID: "XNYS", Day: "2026-11-04", IsSession: true, OpenAt: utc("2026-11-04T14:30:00Z"), CloseAt: utc("2026-11-04T21:00:00Z")},
		{CalendarID: "XNYS", Day: "2026-11-05", IsSession: true, OpenAt: utc("2026-11-05T14:30:00Z"), CloseAt: utc("2026-11-05T21:00:00Z")},
	}}
}

func newRec(led *fakeLedger, clk clockwork.Clock, job *fakeJob) runner.Reconciler {
	return NewWithRule(led, oneDay(), clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"})
}

func TestReconcileRepairNoValueCrash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{
		keys:    []contracts.ExpectedKey{keyA, keyB},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {}}, // returns nothing
	}
	// keyB crashed earlier: a running run whose lease is before now.
	led.runs["2026-11-04"] = []store.Run{{
		ID: "2026-11-04|ds|b|1", Day: "2026-11-04", Key: keyB, Status: store.RunRunning,
		LeaseUntil: clk.Now().Add(-time.Minute),
	}}
	if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-04")
	if present[keyA] != store.StatusNoValue {
		t.Fatalf("keyA = %q, want no_value", present[keyA])
	}
	if present[keyB] != store.StatusPresent {
		t.Fatalf("keyB = %q, want present", present[keyB])
	}
	runs, _ := led.Runs(ctx, "2026-11-04")
	perKey := map[contracts.ExpectedKey]int{}
	for _, r := range runs {
		perKey[r.Key]++
		if r.Status != store.RunDone {
			t.Fatalf("run %s status %s", r.ID, r.Status)
		}
	}
	if perKey[keyA] != 1 || perKey[keyB] != 2 {
		t.Fatalf("runs per key = %v, want a:1 b:2 (one repair, one crash re-run)", perKey)
	}
	// A second pass finds nothing missing and starts no run.
	before := len(job.calls)
	if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != before {
		t.Fatalf("second pass ran %d more times", len(job.calls)-before)
	}
}

func TestFullCycleLateRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	// The run started inside the window and crashed.
	led.runs["2026-11-04"] = []store.Run{{
		ID: "2026-11-04|ds|a|1", Day: "2026-11-04", Key: keyA, Status: store.RunRunning,
		LeaseUntil: clk.Now().Add(5 * time.Minute),
	}}
	// Plan: lease still live, so the key is pending and nothing is re-run.
	if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 0 || led.cursor.Day != "2026-11-03" {
		t.Fatalf("calls=%d cursor=%s", len(job.calls), led.cursor.Day)
	}
	w, err := led.Window(ctx, "ds", "2026-11-04")
	if err != nil {
		t.Fatal(err)
	}
	// Late run: advance past CloseAt; the lease has expired, so recovery re-runs once.
	clk.Advance(w.CloseAt.Sub(clk.Now()) + time.Hour)
	if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-04")
	if present[keyA] != store.StatusPresent || len(job.calls) != 1 {
		t.Fatalf("present=%v calls=%d", present, len(job.calls))
	}
	runs, _ := led.Runs(ctx, "2026-11-04")
	if len(runs) != 2 || runs[1].ReasonCode != "late" {
		t.Fatalf("runs = %+v", runs)
	}
	if led.cursor.Day != "2026-11-04" {
		t.Fatalf("cursor = %s", led.cursor.Day)
	}
}

func TestCursorHoldsOnGap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-06T00:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{
		keys:    []contracts.ExpectedKey{keyA},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: kindErr{kind: contracts.Retryable}}},
	}
	rec := newRec(led, clk, job)
	if err := rec.Reconcile(ctx, "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	// 2026-11-04 failed its one repair run: unresolved gap, the cursor holds; the walk goes on to 11-05.
	if led.cursor.Day != "2026-11-03" {
		t.Fatalf("cursor = %s, want 2026-11-03", led.cursor.Day)
	}
	if len(led.runs["2026-11-05"]) == 0 {
		t.Fatal("the walk stopped at the gap: 2026-11-05 has no run")
	}
	if len(led.runs["2026-11-04"]) != 1 {
		t.Fatalf("runs = %d, want exactly one repair run", len(led.runs["2026-11-04"]))
	}
	// Another pass starts no further run: both windows have closed, so each absence is
	// expired (no_value) and the cursor passes both days.
	if err := rec.Reconcile(ctx, "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []contracts.Day{"2026-11-04", "2026-11-05"} {
		if len(led.runs[d]) != 1 || led.runs[d][0].ReasonCode != "expired" {
			t.Fatalf("runs on %s = %+v, want the one run expired", d, led.runs[d])
		}
	}
	if led.cursor.Day != "2026-11-05" {
		t.Fatalf("cursor = %s, want 2026-11-05", led.cursor.Day)
	}
	// A clean ledger advances over both days.
	led2 := newLedger()
	led2.cursor = store.Cursor{Day: "2026-11-03"}
	if err := newRec(led2, clk, &fakeJob{keys: []contracts.ExpectedKey{keyA}}).Reconcile(ctx, "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	if led2.cursor.Day != "2026-11-05" {
		t.Fatalf("cursor = %s, want 2026-11-05", led2.cursor.Day)
	}
}

func TestWalkPastHeldDay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sess := func(d string) clock.Session {
		return clock.Session{CalendarID: "XNYS", Day: contracts.Day(d), IsSession: true,
			OpenAt: utc(d + "T14:30:00Z"), CloseAt: utc(d + "T21:00:00Z")}
	}
	cal := fakeCalendar{sessions: []clock.Session{sess("2026-11-03"), sess("2026-11-04"), sess("2026-11-05")}}
	// D = 11-05, D-3 = 11-02 is the cursor, so 11-03 is the held day (D-2) the fred key fails on.
	clk := clockwork.NewFakeClockAt(utc("2026-11-05T22:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-02"}
	fredKey := contracts.ExpectedKey{DatasetID: "fred", SubjectKey: "DGS3MO"}
	optKey := contracts.ExpectedKey{DatasetID: "option_eod", SubjectKey: "SPY"}
	fred := windowedJob{
		fakeJob: &fakeJob{id: "fred_series", keys: []contracts.ExpectedKey{fredKey}, results: map[contracts.ExpectedKey]contracts.JobResult{
			fredKey: {Err: kindErr{kind: contracts.Retryable}},
		}},
		calendarID: "XNYS", close: 96 * time.Hour,
	}
	opt := windowedJob{fakeJob: &fakeJob{id: "option_eod", keys: []contracts.ExpectedKey{optKey}}, calendarID: "XNYS", close: 5 * time.Hour}
	rec := NewWithRule(led, cal, clk, []contracts.Job{fred, opt}, runner.WindowRule{CalendarID: "XNYS"})
	if err := rec.Reconcile(ctx, "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-05")
	if present[optKey] != store.StatusPresent || len(led.runs["2026-11-05"]) == 0 {
		t.Fatalf("option_eod key of D not run: present=%v", present)
	}
	if led.cursor.Day != "2026-11-02" {
		t.Fatalf("cursor = %s, want it held at 2026-11-02, before the held day", led.cursor.Day)
	}
}

type kindErr struct{ kind contracts.ErrorKind }

func (e kindErr) Error() string             { return string(e.kind) }
func (e kindErr) Kind() contracts.ErrorKind { return e.kind }

// idCalendar records the calendar ids it is asked for.
type idCalendar struct {
	fakeCalendar
	ids *[]string
}

func (c idCalendar) Session(ctx context.Context, id string, day contracts.Day) (clock.Session, error) {
	*c.ids = append(*c.ids, id)
	return c.fakeCalendar.Session(ctx, id, day)
}

func TestNewWithRuleEmptyCalendar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, set, want string }{
		{"empty is default", "", DefaultCalendarID},
		{"set is used", "X", "X"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ids []string
			cal := idCalendar{fakeCalendar: oneDay(), ids: &ids}
			clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
			job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
			rec := NewWithRule(newLedger(), cal, clk, []contracts.Job{job}, runner.WindowRule{CalendarID: tc.set})
			if err := rec.Reconcile(context.Background(), "2026-11-04"); err != nil {
				t.Fatal(err)
			}
			if len(ids) == 0 {
				t.Fatal("calendar never asked")
			}
			for _, id := range ids {
				if id != tc.want {
					t.Fatalf("calendar id = %q, want %q", id, tc.want)
				}
			}
		})
	}
}

func empty() map[contracts.ExpectedKey]contracts.JobResult {
	return map[contracts.ExpectedKey]contracts.JobResult{keyA: {}}
}

func TestEmptyBeforeCloseStaysMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: empty()}
	rec := newRec(led, clk, job)
	if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-04")
	if _, ok := present[keyA]; ok || led.cursor.Day != "2026-11-03" {
		t.Fatalf("present=%v cursor=%s", present, led.cursor.Day)
	}
	if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(job.calls))
	}
	job.mu.Lock()
	job.results = nil
	job.mu.Unlock()
	if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ = led.Present(ctx, "2026-11-04")
	if present[keyA] != store.StatusPresent || led.cursor.Day != "2026-11-04" {
		t.Fatalf("present=%v cursor=%s", present, led.cursor.Day)
	}
	ids := map[string]bool{}
	for _, r := range led.runs["2026-11-04"] {
		if ids[r.ID] {
			t.Fatalf("duplicate run id %s", r.ID)
		}
		ids[r.ID] = true
	}
}

func TestEmptyBeforeCloseDoesNotExhaustCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: empty()}
	rec := newRec(led, clk, job)
	for i := 1; i <= 3; i++ {
		if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
			t.Fatal(err)
		}
		if len(job.calls) != i {
			t.Fatalf("after call %d: %d runs", i, len(job.calls))
		}
	}
	clk.Advance(time.Hour) // at the close instant
	if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-04")
	if present[keyA] != store.StatusNoValue || len(job.calls) != 4 {
		t.Fatalf("present=%v calls=%d", present, len(job.calls))
	}
}

func TestNoValueErrorFinalBeforeClose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: kindErr{kind: contracts.NoValue}}}}
	if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-04")
	if present[keyA] != store.StatusNoValue || led.cursor.Day != "2026-11-04" {
		t.Fatalf("present=%v cursor=%s", present, led.cursor.Day)
	}
}

func TestEmptyAtOrAfterCloseNoValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, now string }{
		{"at close", "2026-11-04T21:00:00Z"},
		{"after close", "2026-11-04T22:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			clk := clockwork.NewFakeClockAt(utc(tc.now))
			led := newLedger()
			led.cursor = store.Cursor{Day: "2026-11-03"}
			job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: empty()}
			if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
				t.Fatal(err)
			}
			present, _ := led.Present(ctx, "2026-11-04")
			if present[keyA] != store.StatusNoValue || led.cursor.Day != "2026-11-04" {
				t.Fatalf("present=%v cursor=%s", present, led.cursor.Day)
			}
		})
	}
}

func TestNonEmptyFetchUnchanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-04")
	if present[keyA] != store.StatusPresent || led.cursor.Day != "2026-11-04" {
		t.Fatalf("present=%v cursor=%s", present, led.cursor.Day)
	}
	led2 := newLedger()
	led2.cursor = store.Cursor{Day: "2026-11-03"}
	job2 := &fakeJob{keys: []contracts.ExpectedKey{keyA},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: kindErr{kind: contracts.Retryable}}}}
	if err := newRec(led2, clk, job2).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	present, _ = led2.Present(ctx, "2026-11-04")
	if _, ok := present[keyA]; ok || led2.cursor.Day != "2026-11-03" {
		t.Fatalf("present=%v cursor=%s", present, led2.cursor.Day)
	}
}

func TestCrashedRunAfterEmptyRunIsTheOneClosed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	led.runs["2026-11-04"] = []store.Run{
		{ID: "2026-11-04|ds|a|1", Day: "2026-11-04", Key: keyA, Status: store.RunDone, ReasonCode: reasonEmptyBeforeClose},
		{ID: "2026-11-04|ds|a|2", Day: "2026-11-04", Key: keyA, Status: store.RunRunning, LeaseUntil: clk.Now().Add(-time.Minute)},
	}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	if err := newRec(led, clk, job).Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	runs := led.runs["2026-11-04"]
	if len(runs) != 3 {
		t.Fatalf("runs = %+v, want 3", runs)
	}
	if runs[0].ReasonCode != reasonEmptyBeforeClose || runs[0].ErrorCode != "" {
		t.Fatalf("first run was rewritten: %+v", runs[0])
	}
	if runs[1].Status != store.RunDone || runs[1].ReasonCode != reasonCrashed {
		t.Fatalf("crashed run not closed: %+v", runs[1])
	}
	if runs[2].ID != "2026-11-04|ds|a|3" {
		t.Fatalf("new run id = %s", runs[2].ID)
	}
	present, _ := led.Present(ctx, "2026-11-04")
	if present[keyA] != store.StatusPresent {
		t.Fatalf("present = %v", present)
	}
}

func TestEmptyWithoutWindowIsNoValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T10:00:00Z"))
	led := newLedger()
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: empty()}
	r := newRec(led, clk, job).(*reconciler)
	// A non-session has no window for any dataset: the key keeps the old rule.
	done, err := r.reconcileDay(ctx, clock.Session{CalendarID: "XNYS", Day: "2026-11-01"})
	if err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, "2026-11-01")
	if !done || present[keyA] != store.StatusNoValue {
		t.Fatalf("done=%v present=%v", done, present)
	}
}

// dueDay is 2026-11-05 of oneDay: XNYS closes at 21:00Z.
const dueDay contracts.Day = "2026-11-05"

func eodJob(keys ...contracts.ExpectedKey) scheduledJob {
	return scheduledJob{fakeJob: &fakeJob{id: "eod", keys: keys}, calendarID: "XNYS", anchor: "session_close", offset: 0}
}

func dueRec(led store.OpsLedger, at string, jobs ...contracts.Job) runner.Reconciler {
	return NewWithRule(led, oneDay(), clockwork.NewFakeClockAt(utc(at)), jobs, runner.WindowRule{CalendarID: "XNYS"})
}

func dueLedger() *fakeLedger {
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-04"}
	return led
}

func TestScheduledJobHeldBeforeSlotInstant(t *testing.T) {
	t.Parallel()
	led, job := dueLedger(), eodJob(keyA)
	if err := dueRec(led, "2026-11-05T00:09:00Z", job).Reconcile(context.Background(), dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 0 || led.record != 0 || len(job.calls) != 0 {
		t.Fatalf("runs = %+v, calls = %v, want none", led.runs[dueDay], job.calls)
	}
	if led.marks != 0 || len(led.present[dueDay]) != 0 {
		t.Fatalf("marks = %d, present = %v, want none", led.marks, led.present[dueDay])
	}
	if led.cursor.Day != "2026-11-04" {
		t.Fatalf("cursor = %s, want it before the day", led.cursor.Day)
	}
	if len(led.expected[dueDay]) != 1 {
		t.Fatalf("expected = %v, want the held key planned", led.expected[dueDay])
	}
}

func TestScheduledJobStartsAtSlotInstantAndCursorAdvances(t *testing.T) {
	t.Parallel()
	led, job := dueLedger(), eodJob(keyA)
	ctx := context.Background()
	if err := dueRec(led, "2026-11-05T20:59:00Z", job).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 0 || led.cursor.Day != "2026-11-04" {
		t.Fatalf("one minute early: runs = %+v, cursor = %s", led.runs[dueDay], led.cursor.Day)
	}
	if err := dueRec(led, "2026-11-05T21:00:00Z", job).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 1 || led.cursor.Day != dueDay {
		t.Fatalf("at the instant: runs = %+v, cursor = %s", led.runs[dueDay], led.cursor.Day)
	}
}

func TestSessionOpenNegativeOffsetHeldUntilInstant(t *testing.T) {
	t.Parallel()
	led := dueLedger()
	job := scheduledJob{fakeJob: &fakeJob{id: "open", keys: []contracts.ExpectedKey{keyA}}, calendarID: "XNYS", anchor: "session_open", offset: -45 * time.Minute}
	ctx := context.Background()
	if err := dueRec(led, "2026-11-05T13:44:00Z", job).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 0 {
		t.Fatalf("runs = %+v one minute before open-45m, want none", led.runs[dueDay])
	}
	if err := dueRec(led, "2026-11-05T13:45:00Z", job).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 1 {
		t.Fatalf("runs = %+v at open-45m, want one", led.runs[dueDay])
	}
}

func TestScheduledXTKSJobPastInstantStartsAtMidnightUTC(t *testing.T) {
	t.Parallel()
	led := dueLedger()
	cal := fakeCalendar{sessions: []clock.Session{
		{CalendarID: "XTKS", Day: dueDay, IsSession: true, OpenAt: utc("2026-11-05T00:00:00Z"), CloseAt: utc("2026-11-05T06:00:00Z")},
	}}
	job := scheduledJob{fakeJob: &fakeJob{id: "jp", keys: []contracts.ExpectedKey{keyA}}, calendarID: "XTKS", anchor: "session_open", offset: -45 * time.Minute}
	clk := clockwork.NewFakeClockAt(utc("2026-11-05T00:09:00Z"))
	r := NewWithRule(led, cal, clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XTKS"})
	if err := r.Reconcile(context.Background(), dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 1 {
		t.Fatalf("runs = %+v, want one", led.runs[dueDay])
	}
}

func TestScheduledJobPastDayStartsAtOnce(t *testing.T) {
	t.Parallel()
	led, job := dueLedger(), eodJob(keyA)
	if err := dueRec(led, "2026-11-06T21:00:00Z", job).Reconcile(context.Background(), dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 1 || led.cursor.Day != dueDay {
		t.Fatalf("runs = %+v, cursor = %s", led.runs[dueDay], led.cursor.Day)
	}
}

func TestUnscheduledJobStartsAtMidnightUTC(t *testing.T) {
	t.Parallel()
	led, job := dueLedger(), &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	if err := dueRec(led, "2026-11-05T00:09:00Z", job).Reconcile(context.Background(), dueDay); err != nil {
		t.Fatal(err)
	}
	if len(led.runs[dueDay]) != 1 {
		t.Fatalf("runs = %+v, want one", led.runs[dueDay])
	}
}

func TestHeldJobHashStableSoPutExpectedOnce(t *testing.T) {
	t.Parallel()
	led := &countingLedger{fakeLedger: dueLedger()}
	job := eodJob(keyA)
	ctx := context.Background()
	if err := dueRec(led, "2026-11-05T00:09:00Z", job).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	if err := dueRec(led, "2026-11-05T21:00:00Z", job).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	// 2026-11-04 is behind the cursor, so only the day is planned: one call across both passes.
	if led.expecteds != 1 {
		t.Fatalf("PutExpected calls = %d, want 1", led.expecteds)
	}
}

func TestBackfillOfTodayWithHeldJobReturnsEveryKey(t *testing.T) {
	t.Parallel()
	led := newLedger()
	led.cursor = store.Cursor{Day: dueDay}
	held, plain := eodJob(keyA), &fakeJob{id: "plain", keys: []contracts.ExpectedKey{keyB}}
	clk := clockwork.NewFakeClockAt(utc("2026-11-05T00:09:00Z"))
	led.expected[dueDay], led.hash[dueDay] = []contracts.ExpectedKey{keyA, keyB}, "plan-1"
	b := NewBackfillerWithRule(led, oneDay(), clk, []contracts.Job{held, plain}, runner.WindowRule{CalendarID: "XNYS"})
	res, err := b.Backfill(context.Background(), dueDay, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Refused != "" || len(res.InWindow) != 2 || res.InWindow[0] != keyA || res.InWindow[1] != keyB {
		t.Fatalf("res = %+v, want both keys, the held one included, in the window", res)
	}
}

func TestScheduledJobWithUnknownAnchorIsAnError(t *testing.T) {
	t.Parallel()
	led := dueLedger()
	job := scheduledJob{fakeJob: &fakeJob{id: "bad", keys: []contracts.ExpectedKey{keyA}}, calendarID: "XNYS", anchor: "bogus"}
	err := dueRec(led, "2026-11-05T21:00:00Z", job).Reconcile(context.Background(), dueDay)
	if err == nil || !strings.Contains(err.Error(), "job bad: unknown anchor") {
		t.Fatalf("err = %v, want unknown anchor naming the job", err)
	}
}
