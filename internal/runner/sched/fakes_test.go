package sched

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// events is the ordered record of what the fakes saw; a test reads it with until.
type events chan string

func (e events) emit(s string) { e <- s }

// fakeFence is an in-memory store.Fence. It also has a TryLock that counts calls, which the ticker
// must never make.
type fakeFence struct {
	lost    chan struct{}
	closed  atomic.Bool
	tryLock atomic.Int32
}

func newFakeFence() *fakeFence { return &fakeFence{lost: make(chan struct{})} }

func (f *fakeFence) Lost() <-chan struct{} { return f.lost }
func (f *fakeFence) Close() error          { f.closed.Store(true); return nil }
func (f *fakeFence) TryLock(context.Context, string) (store.Fence, error) {
	f.tryLock.Add(1)
	return f, nil
}

// fakeLedger is an in-memory store.SchedulerLedger; only PutJobStates is implemented. block, when
// set, is waited on (or the context) after "put-blocked" is emitted.
type fakeLedger struct {
	store.SchedulerLedger
	ev    events
	block chan struct{}

	mu     sync.Mutex
	states []store.JobState
}

func (l *fakeLedger) PutJobStates(ctx context.Context, states []store.JobState) error {
	if l.block != nil {
		l.ev.emit("put-blocked")
		select {
		case <-l.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	l.mu.Lock()
	l.states = append([]store.JobState(nil), states...)
	l.mu.Unlock()
	l.ev.emit("put")
	return nil
}

func (l *fakeLedger) last() []store.JobState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]store.JobState(nil), l.states...)
}

// fakeOps is a store.OpsLedger that answers Cursor only.
type fakeOps struct {
	store.OpsLedger
	cursor  contracts.Day
	cursors atomic.Int32
}

func (o *fakeOps) Cursor(context.Context) (store.Cursor, error) {
	o.cursors.Add(1)
	return store.Cursor{Day: o.cursor}, nil
}
