package reconcile

import (
	"context"
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
	// 2026-11-04 failed its one repair run: unresolved gap, the cursor holds and 11-05 is not touched.
	if led.cursor.Day != "2026-11-03" {
		t.Fatalf("cursor = %s, want 2026-11-03", led.cursor.Day)
	}
	if len(led.runs["2026-11-05"]) != 0 {
		t.Fatal("later day was reconciled past a gap")
	}
	if len(led.runs["2026-11-04"]) != 1 {
		t.Fatalf("runs = %d, want exactly one repair run", len(led.runs["2026-11-04"]))
	}
	// Another pass starts no further run: the gap stays.
	if err := rec.Reconcile(ctx, "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	if len(led.runs["2026-11-04"]) != 1 || led.cursor.Day != "2026-11-03" {
		t.Fatal("gap was retried or cursor moved")
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
