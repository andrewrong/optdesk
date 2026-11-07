package reconcile

import (
	"context"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeCalendar answers from a list of sessions in day order; days not listed are not sessions.
type fakeCalendar struct{ sessions []clock.Session }

func (f fakeCalendar) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	for _, s := range f.sessions {
		if s.Day == day {
			return s, nil
		}
	}
	return clock.Session{CalendarID: id, Day: day}, nil
}

func (f fakeCalendar) NextSession(_ context.Context, _ string, day contracts.Day) (clock.Session, error) {
	for _, s := range f.sessions {
		if s.Day > day {
			return s, nil
		}
	}
	return clock.Session{}, noValue{}
}

type noValue struct{}

func (noValue) Error() string             { return "no value" }
func (noValue) Kind() contracts.ErrorKind { return contracts.NoValue }

// fakeLedger is an in-memory store.OpsLedger.
type fakeLedger struct {
	mu       sync.Mutex
	expected map[contracts.Day][]contracts.ExpectedKey
	hash     map[contracts.Day]string
	present  map[contracts.Day]map[contracts.ExpectedKey]store.PresentStatus
	windows  map[string]store.Window
	runs     map[contracts.Day][]store.Run
	cursor   store.Cursor
	puts     []store.Window
	record   int
}

func newLedger() *fakeLedger {
	return &fakeLedger{
		expected: map[contracts.Day][]contracts.ExpectedKey{},
		hash:     map[contracts.Day]string{},
		present:  map[contracts.Day]map[contracts.ExpectedKey]store.PresentStatus{},
		windows:  map[string]store.Window{},
		runs:     map[contracts.Day][]store.Run{},
	}
}

func (l *fakeLedger) PutExpected(_ context.Context, d contracts.Day, h string, k []contracts.ExpectedKey) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.expected[d], l.hash[d] = k, h
	return nil
}

func (l *fakeLedger) Expected(_ context.Context, d contracts.Day) ([]contracts.ExpectedKey, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.expected[d], l.hash[d], nil
}

func (l *fakeLedger) MarkPresent(_ context.Context, d contracts.Day, k contracts.ExpectedKey, s store.PresentStatus) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.present[d] == nil {
		l.present[d] = map[contracts.ExpectedKey]store.PresentStatus{}
	}
	l.present[d][k] = s
	return nil
}

func (l *fakeLedger) Present(_ context.Context, d contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[contracts.ExpectedKey]store.PresentStatus{}
	for k, v := range l.present[d] {
		out[k] = v
	}
	return out, nil
}

func (l *fakeLedger) PutWindow(_ context.Context, w store.Window) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.windows[w.DatasetID+"|"+string(w.Day)] = w
	l.puts = append(l.puts, w)
	return nil
}

func (l *fakeLedger) Window(_ context.Context, id string, d contracts.Day) (store.Window, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w, ok := l.windows[id+"|"+string(d)]; ok {
		return w, nil
	}
	return store.Window{}, noValue{}
}

func (l *fakeLedger) RecordRun(_ context.Context, r store.Run) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.record++
	for i, o := range l.runs[r.Day] {
		if o.ID == r.ID {
			l.runs[r.Day][i] = r
			return nil
		}
	}
	l.runs[r.Day] = append(l.runs[r.Day], r)
	return nil
}

func (l *fakeLedger) Runs(_ context.Context, d contracts.Day) ([]store.Run, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]store.Run(nil), l.runs[d]...), nil
}

func (l *fakeLedger) SetCursor(_ context.Context, c store.Cursor) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cursor = c
	return nil
}

func (l *fakeLedger) Cursor(context.Context) (store.Cursor, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cursor, nil
}

func (l *fakeLedger) Acquire(_ context.Context, owner string, ttl time.Duration) (store.Lease, error) {
	return store.Lease{Owner: owner, ExpiresAt: time.Time{}.Add(ttl)}, nil
}
func (l *fakeLedger) Release(context.Context, store.Lease) error { return nil }
func (l *fakeLedger) Close() error                               { return nil }

// fakeJob plans fixed keys and answers each run from results (default: one usable record).
type fakeJob struct {
	id      string
	keys    []contracts.ExpectedKey
	results map[contracts.ExpectedKey]contracts.JobResult
	mu      sync.Mutex
	calls   []contracts.ExpectedKey
}

func (j *fakeJob) ID() string {
	if j.id != "" {
		return j.id
	}
	return "fake"
}
func (j *fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.keys, "plan-1", nil
}

func (j *fakeJob) Run(_ context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls = append(j.calls, keys...)
	if r, ok := j.results[keys[0]]; ok {
		return r
	}
	return contracts.JobResult{Returned: 1, Usable: 1}
}

// windowedJob is a fakeJob that declares its window with plain values, as runner.Windowed asks.
type windowedJob struct {
	*fakeJob
	calendarID  string
	open, close time.Duration
}

func (j windowedJob) Window() (string, time.Duration, time.Duration) {
	return j.calendarID, j.open, j.close
}

// countingLedger counts the calls that a refused Reconcile must not make.
type countingLedger struct {
	*fakeLedger
	acquires, expecteds int
}

func (l *countingLedger) Acquire(ctx context.Context, owner string, ttl time.Duration) (store.Lease, error) {
	l.acquires++
	return l.fakeLedger.Acquire(ctx, owner, ttl)
}

func (l *countingLedger) PutExpected(ctx context.Context, d contracts.Day, h string, k []contracts.ExpectedKey) error {
	l.expecteds++
	return l.fakeLedger.PutExpected(ctx, d, h, k)
}
