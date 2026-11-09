package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// emptyHistory and emptyExpect are readers over no rows.
type emptyHistory struct{}

func (emptyHistory) GapsSince(context.Context, time.Time) ([]gaphistory.Gap, error) {
	return []gaphistory.Gap{}, nil
}
func (emptyHistory) Close() error { return nil }

type emptyExpect struct{}

func (emptyExpect) OpenGaps(context.Context) ([]store.Gap, error) { return nil, nil }
func (emptyExpect) Expectations(context.Context, contracts.Day, contracts.Day) ([]store.Expectation, error) {
	return nil, nil
}
func (emptyExpect) Close() error { return nil }

// memSent is a sent store in memory; it records the marks and the closes.
type memSent struct {
	mu     sync.Mutex
	marks  []store.SentMark
	closed int
}

func (m *memSent) SentSince(context.Context, contracts.Day) ([]store.SentMark, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.marks), nil
}

func (m *memSent) MarkSent(_ context.Context, rule string, day contracts.Day) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.marks = append(m.marks, store.SentMark{RuleID: rule, Day: day})
	return nil
}

func (m *memSent) Close() error { m.mu.Lock(); m.closed++; m.mu.Unlock(); return nil }

// noMorning is the morning notifier of a rig that does not test the morning pass.
type noMorning struct{}

func (noMorning) NotifyMorning(context.Context) (bool, error) { return false, nil }

// morningFakes gives a rig empty readers and a morning notifier that does nothing; a test overrides what it checks.
func morningFakes(d *deps) {
	d.openGapHistory = func(context.Context, string) (gaphistory.Reader, error) { return emptyHistory{}, nil }
	d.openGapReader = func(context.Context, string) (store.GapReader, error) { return emptyExpect{}, nil }
	d.newMorningNotifier = func(notify.MorningDeps, notify.Sender, clockwork.Clock, notify.Config) (notify.MorningNotifier, error) {
		return noMorning{}, nil
	}
}

// fakeMorning records each pass in the order list of the rig's notifier.
type fakeMorning struct {
	r      *schedRig
	err    error
	panics bool
}

func (f fakeMorning) NotifyMorning(context.Context) (bool, error) {
	f.r.notifier.mu.Lock()
	f.r.notifier.order = append(f.r.notifier.order, "morning")
	f.r.notifier.mu.Unlock()
	if f.panics {
		panic("boom")
	}
	return false, f.err
}

func (r *schedRig) useMorning(m fakeMorning) *notify.MorningDeps {
	var got notify.MorningDeps
	var mu sync.Mutex
	r.a.deps.newMorningNotifier = func(md notify.MorningDeps, _ notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.MorningNotifier, error) {
		mu.Lock()
		got = md
		mu.Unlock()
		return m, nil
	}
	return &got
}

func TestSchedulerCallsMorningAfterGaps(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.useMorning(fakeMorning{r: r})
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	_ = r.stop(t, done)
	if got := r.notifier.order; !slices.Equal(got, []string{"gaps", "alerts", "morning"}) {
		t.Fatalf("order %v, want [gaps alerts morning]", got)
	}
}

func TestMorningErrorDoesNotStopTick(t *testing.T) {
	t.Parallel()
	for name, m := range map[string]func(*schedRig) fakeMorning{
		"error": func(r *schedRig) fakeMorning {
			return fakeMorning{r: r, err: errors.Join(notify.ErrSend, errors.New("POST https://api.example/bottest-token chat 1000001"))}
		},
		"panic": func(r *schedRig) fakeMorning { return fakeMorning{r: r, panics: true} },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newSchedRig(t, fullEnv)
			r.useMorning(m(r))
			done := r.start()
			r.waitStarted(t, done)
			r.tick()
			r.tick() // the second tick runs: the failed pass did not stop the ticker
			_ = r.stop(t, done)
			if got := r.notifier.order; !slices.Equal(got, []string{"gaps", "alerts", "morning", "gaps", "alerts", "morning"}) {
				t.Fatalf("order %v", got)
			}
			log := r.errb.String()
			if !strings.Contains(log, "morning notify") || strings.Contains(log, "test-token") || strings.Contains(log, "1000001") {
				t.Fatalf("log %q", log)
			}
		})
	}
}

func TestMorningUsesExistingDSNLine(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	var mu sync.Mutex
	var histDSN, readerDSN []string
	r.a.deps.openGapHistory = func(_ context.Context, dsn string) (gaphistory.Reader, error) {
		mu.Lock()
		histDSN = append(histDSN, dsn)
		mu.Unlock()
		return emptyHistory{}, nil
	}
	r.a.deps.openGapReader = func(_ context.Context, dsn string) (store.GapReader, error) {
		mu.Lock()
		readerDSN = append(readerDSN, dsn)
		mu.Unlock()
		return emptyExpect{}, nil
	}
	got := r.useMorning(fakeMorning{r: r})
	done := r.start()
	r.waitStarted(t, done)
	_ = r.stop(t, done)
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(histDSN, []string{"pages-dsn"}) || !slices.Equal(readerDSN, []string{"pages-dsn"}) {
		t.Fatalf("history opened with %v, reader with %v, want the pages line once each", histDSN, readerDSN)
	}
	if r.sentOpen.Load() != 1 || r.sentDSN != "alerts-dsn" {
		t.Fatalf("sent store opened %d times with %q, want once with the alerts line", r.sentOpen.Load(), r.sentDSN)
	}
	if got.History == nil || got.Expect == nil || got.Sent == nil {
		t.Fatalf("morning deps %+v are incomplete", *got)
	}
}

func TestNotifyOnceDryRunPrintsMorning(t *testing.T) {
	t.Parallel()
	r := newNotifyRig(t, fullEnv+alertsEnv) // a dry run needs no token
	sent := &memSent{}
	r.a.deps.openSent = func(context.Context, string) (store.SentStore, error) { return sent, nil }
	r.a.deps.newMorningNotifier = core.NewMorningNotifier
	r.a.deps.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClockAt(time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)) }
	if err := notifyCmd(r.a, []string{"--once", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.out.String(), "morning report 2026-11-06") || !strings.Contains(r.out.String(), "all quiet") {
		t.Fatalf("stdout %q holds no morning text", r.out.String())
	}
	if len(*r.sent) != 0 || len(sent.marks) != 0 || sent.closed != 1 {
		t.Fatalf("sent %v, marks %v, closed %d: a dry run sends and marks nothing", *r.sent, sent.marks, sent.closed)
	}

	// Without --dry-run the same pass sends through the real sender and marks the day.
	r = newNotifyRig(t, notifyEnv)
	sent = &memSent{}
	r.a.deps.openSent = func(context.Context, string) (store.SentStore, error) { return sent, nil }
	r.a.deps.newMorningNotifier = core.NewMorningNotifier
	r.a.deps.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClockAt(time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)) }
	if err := notifyCmd(r.a, []string{"--once"}); err != nil {
		t.Fatal(err)
	}
	if len(*r.sent) != 2 || !strings.Contains((*r.sent)[1], "morning report 2026-11-06") || len(sent.marks) != 1 {
		t.Fatalf("sent %v, marks %v", *r.sent, sent.marks)
	}
}
