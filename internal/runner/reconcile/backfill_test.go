package reconcile

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

const bfDay contracts.Day = "2026-11-04"

// spyLedger counts the writes and the lease calls of a backfill over the in-memory ledger.
type spyLedger struct {
	*fakeLedger
	acquired, released                             atomic.Int32
	records, marks, cursors, expecteds, windowsPut atomic.Int32
	acquireErr                                     error
}

func (s *spyLedger) Acquire(ctx context.Context, o string, ttl time.Duration) (store.Lease, error) {
	if s.acquireErr != nil {
		return store.Lease{}, s.acquireErr
	}
	s.acquired.Add(1)
	return s.fakeLedger.Acquire(ctx, o, ttl)
}

func (s *spyLedger) Release(ctx context.Context, l store.Lease) error {
	s.released.Add(1)
	return s.fakeLedger.Release(ctx, l)
}

func (s *spyLedger) RecordRun(ctx context.Context, r store.Run) error {
	s.records.Add(1)
	return s.fakeLedger.RecordRun(ctx, r)
}

func (s *spyLedger) MarkPresent(ctx context.Context, d contracts.Day, k contracts.ExpectedKey, st store.PresentStatus) error {
	s.marks.Add(1)
	return s.fakeLedger.MarkPresent(ctx, d, k, st)
}

func (s *spyLedger) SetCursor(ctx context.Context, c store.Cursor) error {
	s.cursors.Add(1)
	return s.fakeLedger.SetCursor(ctx, c)
}

func (s *spyLedger) PutExpected(ctx context.Context, d contracts.Day, h string, k []contracts.ExpectedKey) error {
	s.expecteds.Add(1)
	return s.fakeLedger.PutExpected(ctx, d, h, k)
}

func (s *spyLedger) PutWindow(ctx context.Context, w store.Window) error {
	s.windowsPut.Add(1)
	return s.fakeLedger.PutWindow(ctx, w)
}

func (s *spyLedger) writes() int32 {
	return s.records.Load() + s.marks.Load() + s.cursors.Load() + s.expecteds.Load() + s.windowsPut.Load()
}

// bfSetup is a ledger with the stored plan of bfDay and the cursor at cursor.
func bfSetup(cursor contracts.Day, keys ...contracts.ExpectedKey) *spyLedger {
	l := &spyLedger{fakeLedger: newLedger()}
	l.cursor = store.Cursor{Day: cursor}
	if len(keys) > 0 {
		l.expected[bfDay], l.hash[bfDay] = keys, "plan-1"
	}
	return l
}

func newBackfiller(l store.OpsLedger, clk clockwork.Clock, job *fakeJob) runner.Backfiller {
	return NewBackfillerWithRule(l, oneDay(), clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"})
}

func afterClose() clockwork.Clock { return clockwork.NewFakeClockAt(utc("2026-11-06T00:00:00Z")) }

func TestBackfillRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cursor contracts.Day
		day    contracts.Day
		keys   []contracts.ExpectedKey
		want   runner.BackfillRefusal
	}{
		{"empty cursor", "", bfDay, []contracts.ExpectedKey{keyA}, runner.RefusedAheadOfCursor},
		{"after cursor", "2026-11-03", bfDay, []contracts.ExpectedKey{keyA}, runner.RefusedAheadOfCursor},
		{"no plan at cursor", bfDay, bfDay, nil, runner.RefusedNoPlan},
		{"no plan behind cursor", "2026-11-05", bfDay, nil, runner.RefusedNoPlan},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := bfSetup(tc.cursor, tc.keys...)
			job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
			res, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), tc.day, nil)
			if err != nil || res.Refused != tc.want {
				t.Fatalf("refused %q err %v, want %q", res.Refused, err, tc.want)
			}
			if l.writes() != 0 || len(job.calls) != 0 {
				t.Fatalf("writes %d calls %d", l.writes(), len(job.calls))
			}
		})
	}
}

func TestBackfillNoValueBecomesPresent(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"empty_before_close", ""} {
		t.Run("reason "+reason, func(t *testing.T) {
			ctx := context.Background()
			l := bfSetup(bfDay, keyA)
			l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusNoValue}
			first := store.Run{ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunDone, ReasonCode: reason}
			l.runs[bfDay] = []store.Run{first}
			job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
			res, err := newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil)
			if err != nil || len(res.Filled) != 1 || res.Filled[0] != keyA {
				t.Fatalf("res %+v err %v", res, err)
			}
			if l.present[bfDay][keyA] != store.StatusPresent {
				t.Fatalf("status %q", l.present[bfDay][keyA])
			}
			runs := l.runs[bfDay]
			if len(runs) != 2 || runs[0] != first || runs[1].ID != "2026-11-04|ds|a|2" || runs[1].ReasonCode != "backfill" {
				t.Fatalf("runs %+v", runs)
			}
			if l.cursor.Day != bfDay || l.cursors.Load() != 0 {
				t.Fatalf("cursor %s", l.cursor.Day)
			}
		})
	}
}

func TestBackfillPresentIsNotRun(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), bfDay, nil)
	if err != nil || len(res.AlreadyPresent) != 1 || len(job.calls) != 0 || len(l.runs[bfDay]) != 0 || l.writes() != 0 {
		t.Fatalf("res %+v err %v calls %d", res, err, len(job.calls))
	}
}

func TestBackfillIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup(bfDay, keyA, keyB)
	if err := l.PutWindow(ctx, store.Window{DatasetID: "ds", Day: bfDay, CloseAt: utc("2026-11-04T21:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	job := &fakeJob{
		keys:    []contracts.ExpectedKey{keyA, keyB},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyB: {}},
	}
	b := newBackfiller(l, afterClose(), job)
	res, err := b.Backfill(ctx, bfDay, nil)
	if err != nil || len(res.Filled) != 1 || len(res.Absent) != 1 || res.Absent[0] != keyB {
		t.Fatalf("first %+v err %v", res, err)
	}
	res, err = b.Backfill(ctx, bfDay, nil)
	if err != nil || len(res.AlreadyPresent) != 1 || res.AlreadyPresent[0] != keyA || len(res.Absent) != 1 {
		t.Fatalf("second %+v err %v", res, err)
	}
	if len(job.calls) != 3 || job.calls[2] != keyB {
		t.Fatalf("calls %v", job.calls)
	}
	if l.present[bfDay][keyB] != store.StatusNoValue || len(l.runs[bfDay]) != 3 {
		t.Fatalf("status %q runs %d", l.present[bfDay][keyB], len(l.runs[bfDay]))
	}
}

func TestBackfillJobError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	boom := kindErr{kind: contracts.Retryable}
	l := bfSetup(bfDay, keyA, keyB)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyB: store.StatusNoValue}
	job := &fakeJob{
		keys:    []contracts.ExpectedKey{keyA, keyB},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: boom}, keyB: {Err: boom}},
	}
	res, err := newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil)
	if err != nil || len(res.Missing) != 2 || len(res.Filled)+len(res.Absent) != 0 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if _, marked := l.present[bfDay][keyA]; marked || l.present[bfDay][keyB] != store.StatusNoValue {
		t.Fatalf("present %v", l.present[bfDay])
	}
	for _, r := range l.runs[bfDay] {
		if r.ErrorCode != string(contracts.Retryable) || r.ReasonCode != "backfill" {
			t.Fatalf("run %+v", r)
		}
	}

	l = bfSetup(bfDay, keyA)
	job = &fakeJob{
		keys:    []contracts.ExpectedKey{keyA},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: kindErr{kind: contracts.NoValue}}},
	}
	res, err = newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil)
	if err != nil || len(res.Absent) != 1 || len(res.Missing) != 0 {
		t.Fatalf("no value %+v err %v", res, err)
	}
}

func TestBackfillDatasetFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	other := contracts.ExpectedKey{DatasetID: "other", SubjectKey: "x"}
	gone := contracts.ExpectedKey{DatasetID: "gone", SubjectKey: "y"}

	l := bfSetup(bfDay, keyA, other)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, other}}
	res, err := newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, []string{"other"})
	if err != nil || len(res.Filled) != 1 || res.Filled[0] != other || len(job.calls) != 1 {
		t.Fatalf("res %+v err %v calls %v", res, err, job.calls)
	}

	l = bfSetup(bfDay, keyA)
	job = &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	_, err = newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, []string{"ds", "nope"})
	if err == nil || !strings.Contains(err.Error(), "nope") || len(job.calls) != 0 || l.writes() != 0 {
		t.Fatalf("err %v calls %v", err, job.calls)
	}

	l = bfSetup(bfDay, keyA, gone)
	job = &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	_, err = newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil)
	if err == nil || !strings.Contains(err.Error(), "gone") || len(job.calls) != 0 || l.writes() != 0 {
		t.Fatalf("err %v calls %v", err, job.calls)
	}
}

func TestBackfillLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	check := func(name string, l *spyLedger, job *fakeJob, day contracts.Day) {
		if _, err := newBackfiller(l, afterClose(), job).Backfill(ctx, day, nil); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if l.acquired.Load() != 1 || l.released.Load() != 1 {
			t.Fatalf("%s: acquired %d released %d", name, l.acquired.Load(), l.released.Load())
		}
	}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	check("success", bfSetup(bfDay, keyA), job, bfDay)
	check("refusal", bfSetup("", keyA), job, bfDay)
	check("job error", bfSetup(bfDay, keyA), &fakeJob{
		keys:    []contracts.ExpectedKey{keyA},
		results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: kindErr{kind: contracts.Retryable}}},
	}, bfDay)

	l := bfSetup(bfDay, keyA)
	l.acquireErr = errors.New("held")
	job = &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	if _, err := newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil); !errors.Is(err, l.acquireErr) || len(job.calls) != 0 || l.writes() != 0 {
		t.Fatalf("err %v calls %v", err, job.calls)
	}
}

func TestBackfillLiveRunning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := afterClose()
	now := clk.Now()
	running := func(until time.Time) store.Run {
		return store.Run{ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunRunning, LeaseUntil: until}
	}

	l := bfSetup(bfDay, keyA)
	l.runs[bfDay] = []store.Run{running(now.Add(time.Minute))}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err := newBackfiller(l, clk, job).Backfill(ctx, bfDay, nil)
	if err != nil || len(res.Missing) != 1 || len(l.runs[bfDay]) != 1 || len(job.calls) != 0 || l.writes() != 0 {
		t.Fatalf("live: res %+v err %v runs %d", res, err, len(l.runs[bfDay]))
	}

	l = bfSetup(bfDay, keyA)
	l.runs[bfDay] = []store.Run{running(now.Add(-time.Minute))}
	job = &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err = newBackfiller(l, clk, job).Backfill(ctx, bfDay, nil)
	if err != nil || len(res.Filled) != 1 || len(l.runs[bfDay]) != 2 {
		t.Fatalf("expired: res %+v err %v runs %d", res, err, len(l.runs[bfDay]))
	}
	if r := l.runs[bfDay][0]; r.Status != store.RunDone || r.ReasonCode != "crashed" {
		t.Fatalf("closed run %+v", r)
	}
}
