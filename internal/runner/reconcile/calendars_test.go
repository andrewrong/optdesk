package reconcile

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// multiCal answers per calendar id. A calendar missing from rows is unknown. A calendar in runsOut
// answers its sessions first and UnknownCalendar for every day after its last row.
type multiCal struct {
	rows    map[string][]clock.Session
	runsOut map[string]bool
	ids     *[]string
}

func (m multiCal) last(id string) contracts.Day {
	rows := m.rows[id]
	return rows[len(rows)-1].Day
}

func (m multiCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	if m.ids != nil {
		*m.ids = append(*m.ids, id)
	}
	rows, ok := m.rows[id]
	if !ok || (m.runsOut[id] && day > m.last(id)) {
		return clock.Session{}, kindErr{kind: contracts.UnknownCalendar}
	}
	for _, s := range rows {
		if s.Day == day {
			return s, nil
		}
	}
	return clock.Session{CalendarID: id, Day: day}, nil
}

func (m multiCal) NextSession(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	rows, ok := m.rows[id]
	if !ok || (m.runsOut[id] && day >= m.last(id)) {
		return clock.Session{}, kindErr{kind: contracts.UnknownCalendar}
	}
	for _, s := range rows {
		if s.Day > day {
			return s, nil
		}
	}
	return clock.Session{}, noValue{}
}

func sessOf(cal, day, open, closeAt string) clock.Session {
	return clock.Session{CalendarID: cal, Day: contracts.Day(day), IsSession: true, OpenAt: utc(open), CloseAt: utc(closeAt)}
}

// nyse and jpx give XNYS and XJPX sessions: 2026-11-26 is an XNYS holiday and an XJPX session,
// 2026-11-27 is an XNYS session and an XJPX holiday, 2026-11-25 is a session of both.
func twoCalendars() multiCal {
	return multiCal{rows: map[string][]clock.Session{
		"XNYS": {
			sessOf("XNYS", "2026-11-24", "2026-11-24T14:30:00Z", "2026-11-24T21:00:00Z"),
			sessOf("XNYS", "2026-11-25", "2026-11-25T14:30:00Z", "2026-11-25T21:00:00Z"),
			sessOf("XNYS", "2026-11-27", "2026-11-27T14:30:00Z", "2026-11-27T18:00:00Z"),
		},
		"XJPX": {
			sessOf("XJPX", "2026-11-25", "2026-11-25T00:00:00Z", "2026-11-25T06:30:00Z"),
			sessOf("XJPX", "2026-11-26", "2026-11-26T00:00:00Z", "2026-11-26T06:30:00Z"),
		},
	}}
}

func twoJobs() (*fakeJob, windowedJob) {
	a := &fakeJob{id: "us", keys: []contracts.ExpectedKey{keyOf("ds_a")}}
	b := windowedJob{fakeJob: &fakeJob{id: "jp", keys: []contracts.ExpectedKey{keyOf("ds_b")}}, calendarID: "XJPX", close: 2 * time.Hour}
	return a, b
}

func dsOf(keys []contracts.ExpectedKey) string {
	var ids []string
	for _, k := range keys {
		ids = append(ids, k.DatasetID)
	}
	return strings.Join(ids, ",")
}

func TestUnionWalksBothCalendars(t *testing.T) {
	t.Parallel()
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-25"}
	a, b := twoJobs()
	clk := clockwork.NewFakeClockAt(utc("2026-11-26T12:00:00Z"))
	r := New(led, twoCalendars(), clk, []contracts.Job{a, b})
	if err := r.Reconcile(context.Background(), "2026-11-26"); err != nil {
		t.Fatal(err)
	}
	if got := dsOf(led.expected["2026-11-26"]); got != "ds_b" {
		t.Fatalf("keys on the XNYS holiday = %q, want ds_b only", got)
	}
	if len(led.puts) != 1 || led.puts[0].DatasetID != "ds_b" ||
		led.puts[0].OpenAt != utc("2026-11-26T00:00:00Z") || led.puts[0].CloseAt != utc("2026-11-26T08:30:00Z") {
		t.Fatalf("puts = %+v, want the ds_b window from the XJPX open and close", led.puts)
	}
	if led.cursor.Day != "2026-11-26" {
		t.Fatalf("cursor = %s, want 2026-11-26", led.cursor.Day)
	}
}

func TestJobNotPlannedOnOtherCalendarHoliday(t *testing.T) {
	t.Parallel()
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-26"}
	a, b := twoJobs()
	clk := clockwork.NewFakeClockAt(utc("2026-11-27T12:00:00Z"))
	r := New(led, twoCalendars(), clk, []contracts.Job{a, b})
	if err := r.Reconcile(context.Background(), "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	if got := dsOf(led.expected["2026-11-27"]); got != "ds_a" {
		t.Fatalf("keys on the XJPX holiday = %q, want ds_a only", got)
	}
	if len(a.calls) != 1 || len(b.calls) != 0 {
		t.Fatalf("runs: us %d, jp %d", len(a.calls), len(b.calls))
	}
}

func TestWindowFromJobCalendar(t *testing.T) {
	t.Parallel()
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-24"}
	a, b := twoJobs()
	clk := clockwork.NewFakeClockAt(utc("2026-11-25T12:00:00Z"))
	r := New(led, twoCalendars(), clk, []contracts.Job{a, b})
	if err := r.Reconcile(context.Background(), "2026-11-25"); err != nil {
		t.Fatal(err)
	}
	closes := map[string]time.Time{}
	for _, w := range led.puts {
		closes[w.DatasetID] = w.CloseAt
	}
	if len(led.puts) != 2 || closes["ds_a"] != utc("2026-11-25T21:00:00Z") || closes["ds_b"] != utc("2026-11-25T08:30:00Z") {
		t.Fatalf("puts = %+v", led.puts)
	}
}

func TestSingleCalendarUnchanged(t *testing.T) {
	t.Parallel()
	led := &countingLedger{fakeLedger: newLedger()}
	led.cursor = store.Cursor{Day: "2026-11-23"}
	var ids []string
	cal := twoCalendars()
	cal.ids = &ids
	j1 := &fakeJob{id: "j1", keys: []contracts.ExpectedKey{keyOf("ds_a")}}
	j2 := &fakeJob{id: "j2", keys: []contracts.ExpectedKey{keyOf("ds_b")}}
	clk := clockwork.NewFakeClockAt(utc("2026-11-25T12:00:00Z"))
	r := New(led, cal, clk, []contracts.Job{j1, j2})
	if err := r.Reconcile(context.Background(), "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []contracts.Day{"2026-11-24", "2026-11-25", "2026-11-27"} {
		if got := dsOf(led.expected[d]); got != "ds_a,ds_b" {
			t.Fatalf("keys on %s = %q", d, got)
		}
	}
	if len(led.expected) != 3 || len(led.puts) != 6 || led.cursor.Day != "2026-11-27" {
		t.Fatalf("expected=%d puts=%d cursor=%s", len(led.expected), len(led.puts), led.cursor.Day)
	}
	if led.acquires != 1 || led.expecteds != 3 {
		t.Fatalf("acquires=%d expecteds=%d, want 1 and 3", led.acquires, led.expecteds)
	}
	if len(ids) != 0 {
		t.Fatalf("a one-calendar reconciler asked Session for %v", ids)
	}
}

func TestCursorAdvancesOverCompletePrefixAcrossCalendars(t *testing.T) {
	t.Parallel()
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-24"}
	a, b := twoJobs()
	// The XJPX job fails retryably on its windows, which are still open: its days stay incomplete.
	b.results = map[contracts.ExpectedKey]contracts.JobResult{keyOf("ds_b"): {Err: kindErr{kind: contracts.Retryable}}}
	clk := clockwork.NewFakeClockAt(utc("2026-11-25T01:00:00Z"))
	r := New(led, twoCalendars(), clk, []contracts.Job{a, b})
	if err := r.Reconcile(context.Background(), "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	// 11-25 holds ds_b open, so the cursor stays at 11-24 although 11-26 (XJPX only) and 11-27 are walked.
	if led.cursor.Day != "2026-11-24" {
		t.Fatalf("cursor = %s, want 2026-11-24", led.cursor.Day)
	}
	if len(led.runs["2026-11-27"]) == 0 || len(led.runs["2026-11-26"]) == 0 {
		t.Fatalf("the walk did not reach 11-26 and 11-27: %v", led.runs)
	}
	// A day that is a session only of the other calendar completes and moves the cursor.
	led2 := newLedger()
	led2.cursor = store.Cursor{Day: "2026-11-25"}
	a2, b2 := twoJobs()
	if err := New(led2, twoCalendars(), clk, []contracts.Job{a2, b2}).Reconcile(context.Background(), "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	if led2.cursor.Day != "2026-11-27" {
		t.Fatalf("cursor = %s, want 2026-11-27 over 11-26 and 11-27", led2.cursor.Day)
	}
}

func runOutCal(known bool) multiCal {
	xnys := []clock.Session{}
	for _, d := range []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-16", "2026-11-17"} {
		xnys = append(xnys, sessOf("XNYS", d, d+"T14:30:00Z", d+"T21:00:00Z"))
	}
	m := multiCal{rows: map[string][]clock.Session{"XNYS": xnys}, runsOut: map[string]bool{"XJPX": true}}
	if known {
		m.rows["XJPX"] = []clock.Session{
			sessOf("XJPX", "2026-11-10", "2026-11-10T00:00:00Z", "2026-11-10T06:30:00Z"),
			sessOf("XJPX", "2026-11-11", "2026-11-11T00:00:00Z", "2026-11-11T06:30:00Z"),
			sessOf("XJPX", "2026-11-12", "2026-11-12T00:00:00Z", "2026-11-12T06:30:00Z"),
		}
	}
	return m
}

func TestCalendarRunsOutDropsOnlyItsJobs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-10T01:00:00Z"))
	t.Run("known calendar runs out", func(t *testing.T) {
		t.Parallel()
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-09"}
		a, b := twoJobs()
		r := New(led, runOutCal(true), clk, []contracts.Job{a, b})
		var buf bytes.Buffer
		r.(*reconciler).log = slog.New(slog.NewTextHandler(&buf, nil))
		if err := r.Reconcile(ctx, "2026-11-16"); err != nil {
			t.Fatal(err)
		}
		for d, want := range map[contracts.Day]string{"2026-11-12": "ds_a,ds_b", "2026-11-13": "ds_a", "2026-11-16": "ds_a"} {
			if got := dsOf(led.expected[d]); got != want {
				t.Fatalf("keys on %s = %q, want %q", d, got, want)
			}
		}
		if n := strings.Count(buf.String(), "jobs dropped"); n != 1 || !strings.Contains(buf.String(), "XJPX") {
			t.Fatalf("log = %q, want one line naming XJPX", buf.String())
		}
		// The cursor is already past the last XJPX row: the next walk drops it again and does not fail.
		if err := r.Reconcile(ctx, "2026-11-17"); err != nil {
			t.Fatal(err)
		}
		if got := dsOf(led.expected["2026-11-17"]); got != "ds_a" {
			t.Fatalf("keys on 2026-11-17 = %q, want ds_a", got)
		}
	})
	t.Run("calendar unknown at construction", func(t *testing.T) {
		t.Parallel()
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-09"}
		a, b := twoJobs()
		err := New(led, runOutCal(false), clk, []contracts.Job{a, b}).Reconcile(ctx, "2026-11-16")
		if err == nil || !strings.Contains(err.Error(), "jp") {
			t.Fatalf("err = %v, want one naming the job", err)
		}
	})
	t.Run("rule calendar error", func(t *testing.T) {
		t.Parallel()
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-09"}
		cal := runOutCal(true)
		cal.runsOut = map[string]bool{"XNYS": true}
		a, b := twoJobs()
		err := New(led, cal, clk, []contracts.Job{a, b}).Reconcile(ctx, "2026-11-18")
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.UnknownCalendar {
			t.Fatalf("err = %v, want the UnknownCalendar of XNYS", err)
		}
	})
}

func TestKnownSetBuiltAtConstruction(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-12T23:00:00Z")) // XJPX still answers: its last row is 11-12
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-12"}
	a, b := twoJobs()
	r := New(led, runOutCal(true), clk, []contracts.Job{a, b}) // built at start: XJPX known
	clk.Advance(2 * time.Hour)                                 // the first walk is on 2026-11-13
	if err := r.Reconcile(context.Background(), "2026-11-13"); err != nil {
		t.Fatalf("Reconcile = %v, want nil: XJPX answered at construction and only ran out", err)
	}
}
