package reconcile

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

var keyC = contracts.ExpectedKey{DatasetID: "ds2", SubjectKey: "c"}

// logLedger records the order of the plan and window writes, and can fail PutWindow.
type logLedger struct {
	*spyLedger
	mu     sync.Mutex
	log    []string
	putErr error
}

func (l *logLedger) add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.log = append(l.log, s)
}

func (l *logLedger) PutWindow(ctx context.Context, w store.Window) error {
	if l.putErr != nil {
		return l.putErr
	}
	l.add("window:" + w.DatasetID)
	return l.spyLedger.PutWindow(ctx, w)
}

func (l *logLedger) PutExpected(ctx context.Context, d contracts.Day, h string, k []contracts.ExpectedKey) error {
	l.add("expected")
	return l.spyLedger.PutExpected(ctx, d, h, k)
}

func (l *logLedger) SetCursor(ctx context.Context, c store.Cursor) error {
	l.add("cursor")
	return l.spyLedger.SetCursor(ctx, c)
}

// akSetup is a ledger with the plan {A} and the window of ds stored.
func akSetup(t *testing.T) *logLedger {
	t.Helper()
	l := &logLedger{spyLedger: bfSetup(bfDay, keyA)}
	if err := l.fakeLedger.PutWindow(context.Background(), store.Window{DatasetID: "ds", Day: bfDay, CloseAt: utc("2026-11-04T21:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	return l
}

// reconcileHash is the hash Reconcile stores for bfDay for a job planning keys.
func reconcileHash(t *testing.T, keys ...contracts.ExpectedKey) string {
	t.Helper()
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: keys}
	if err := newRec(led, afterClose(), job).Reconcile(context.Background(), bfDay); err != nil {
		t.Fatal(err)
	}
	return led.hash[bfDay]
}

func TestBackfillAddsMissingKeys(t *testing.T) {
	t.Parallel()
	l := akSetup(t)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}}
	res, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), bfDay, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"window:ds2", "expected"}; !reflect.DeepEqual(l.log, want) {
		t.Fatalf("writes %v, want %v", l.log, want)
	}
	if l.windowsPut.Load() != 1 || l.expecteds.Load() != 1 || l.cursors.Load() != 0 {
		t.Fatalf("windows %d expected %d cursors %d", l.windowsPut.Load(), l.expecteds.Load(), l.cursors.Load())
	}
	if want := []contracts.ExpectedKey{keyA, keyC}; !reflect.DeepEqual(l.expected[bfDay], want) {
		t.Fatalf("stored %v, want %v", l.expected[bfDay], want)
	}
	if want := reconcileHash(t, keyA, keyC); l.hash[bfDay] != want {
		t.Fatalf("hash %q, want %q", l.hash[bfDay], want)
	}
	if len(res.Filled) != 2 || res.Filled[1] != keyC {
		t.Fatalf("filled %v", res.Filled)
	}
}

// onlyCal answers only for the calendar id it was built with.
type onlyCal struct {
	id string
	fakeCalendar
}

func (c onlyCal) Session(ctx context.Context, id string, d contracts.Day) (clock.Session, error) {
	if id != c.id {
		return clock.Session{CalendarID: id, Day: d}, nil
	}
	return c.fakeCalendar.Session(ctx, id, d)
}

func (c onlyCal) NextSession(ctx context.Context, id string, d contracts.Day) (clock.Session, error) {
	if id != c.id {
		return clock.Session{}, noValue{}
	}
	return c.fakeCalendar.NextSession(ctx, id, d)
}

func TestBackfillAddKeysNoWindow(t *testing.T) {
	t.Parallel()
	// The job of C is on a calendar with no session on the day: its key has no window rule.
	l := akSetup(t)
	jobA := windowedJob{fakeJob: &fakeJob{id: "a", keys: []contracts.ExpectedKey{keyA}}, calendarID: "XNYS"}
	jobC := windowedJob{fakeJob: &fakeJob{id: "c", keys: []contracts.ExpectedKey{keyC}}, calendarID: "OTHER"}
	cal := onlyCal{id: "XNYS", fakeCalendar: oneDay()}
	b := NewBackfillerWithRule(l, cal, afterClose(), []contracts.Job{jobA, jobC}, runner.WindowRule{CalendarID: "XNYS"})
	if _, err := b.Backfill(context.Background(), bfDay, nil); err != nil {
		t.Fatal(err)
	}
	if l.expecteds.Load() != 0 || l.windowsPut.Load() != 0 || len(l.expected[bfDay]) != 1 {
		t.Fatalf("expected %d windows %d keys %v", l.expecteds.Load(), l.windowsPut.Load(), l.expected[bfDay])
	}

	// A day that is a session of no calendar adds nothing.
	const off contracts.Day = "2026-11-03"
	l = akSetup(t)
	l.expected[off], l.hash[off] = []contracts.ExpectedKey{keyA}, "plan-1"
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}}
	if _, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), off, nil); err != nil {
		t.Fatal(err)
	}
	if l.expecteds.Load() != 0 || l.windowsPut.Load() != 0 || l.cursors.Load() != 0 {
		t.Fatalf("expected %d windows %d", l.expecteds.Load(), l.windowsPut.Load())
	}
}

func TestBackfillNoPlanStillRefused(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), bfDay, nil)
	if err != nil || res.Refused != runner.RefusedNoPlan {
		t.Fatalf("refused %q err %v", res.Refused, err)
	}
	if l.expecteds.Load() != 0 || l.windowsPut.Load() != 0 {
		t.Fatalf("expected %d windows %d", l.expecteds.Load(), l.windowsPut.Load())
	}
}

func TestBackfillAddKeysIdempotent(t *testing.T) {
	t.Parallel()
	l := akSetup(t)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}}
	b := newBackfiller(l, afterClose(), job)
	if _, err := b.Backfill(context.Background(), bfDay, nil); err != nil {
		t.Fatal(err)
	}
	before := len(l.log)
	runs, marks := l.records.Load(), l.marks.Load()
	if _, err := b.Backfill(context.Background(), bfDay, nil); err != nil {
		t.Fatal(err)
	}
	if len(l.log) != before || l.records.Load() != runs || l.marks.Load() != marks {
		t.Fatalf("second call wrote: log %v", l.log)
	}
}

func TestBackfillPutsMissingWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	l := &logLedger{spyLedger: bfSetup(bfDay, keyA)}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	b := newBackfiller(l, clk, job)
	res, err := b.Backfill(ctx, bfDay, nil)
	if err != nil || len(res.InWindow) != 0 || len(res.Filled)+len(res.Absent)+len(res.Missing) != 1 || len(job.calls) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if l.windowsPut.Load() != 1 || l.expecteds.Load() != 0 {
		t.Fatalf("windows %d expected %d", l.windowsPut.Load(), l.expecteds.Load())
	}
	if l.present[bfDay][keyA] != store.StatusPresent {
		t.Fatalf("marked %q", l.present[bfDay][keyA])
	}
	writes := l.writes()
	if _, err := b.Backfill(ctx, bfDay, nil); err != nil {
		t.Fatal(err)
	}
	if l.windowsPut.Load() != 1 || l.writes() != writes {
		t.Fatalf("second call wrote: windows %d writes %d", l.windowsPut.Load(), l.writes())
	}
}

func TestBackfillAddedDataset(t *testing.T) {
	t.Parallel()
	l := akSetup(t)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}}
	res, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), bfDay, []string{"ds2"})
	if err != nil || len(res.Filled) != 1 || res.Filled[0] != keyC {
		t.Fatalf("res %+v err %v", res, err)
	}
	if !reflect.DeepEqual(job.calls, []contracts.ExpectedKey{keyC}) || l.expecteds.Load() != 1 {
		t.Fatalf("calls %v expected %d", job.calls, l.expecteds.Load())
	}
}

func TestBackfillErrorWritesNone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	boom := errors.New("boom")
	l := akSetup(t)
	l.putErr = boom
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}}
	if _, err := newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil); !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
	if l.expecteds.Load() != 0 {
		t.Fatalf("expected %d after a failed PutWindow", l.expecteds.Load())
	}

	l = akSetup(t)
	job = &fakeJob{keys: []contracts.ExpectedKey{keyA, keyC}}
	_, err := newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, []string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "nope") || l.windowsPut.Load() != 0 || l.expecteds.Load() != 0 {
		t.Fatalf("filter: err %v windows %d expected %d", err, l.windowsPut.Load(), l.expecteds.Load())
	}

	gone := contracts.ExpectedKey{DatasetID: "gone", SubjectKey: "y"}
	l = akSetup(t)
	l.expected[bfDay] = []contracts.ExpectedKey{keyA, gone}
	_, err = newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil)
	if err == nil || !strings.Contains(err.Error(), "gone") || l.windowsPut.Load() != 0 || l.expecteds.Load() != 0 {
		t.Fatalf("no job: err %v windows %d expected %d", err, l.windowsPut.Load(), l.expecteds.Load())
	}
}
