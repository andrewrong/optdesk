package core_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/notify/notifytest"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

var kl = time.FixedZone("KL", 8*3600)

type fakeHistory struct {
	mu    sync.Mutex
	rows  []gaphistory.Gap
	since []time.Time
}

func (f *fakeHistory) GapsSince(_ context.Context, since time.Time) ([]gaphistory.Gap, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.since = append(f.since, since)
	return append([]gaphistory.Gap{}, f.rows...), nil
}
func (f *fakeHistory) Close() error { return nil }

type fakeExpect struct {
	mu       sync.Mutex
	open     []store.Gap
	exps     []store.Expectation
	reads    int
	from, to contracts.Day
}

func (f *fakeExpect) OpenGaps(context.Context) ([]store.Gap, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.open, nil
}
func (f *fakeExpect) Expectations(_ context.Context, from, to contracts.Day) ([]store.Expectation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	f.from, f.to = from, to
	return f.exps, nil
}
func (f *fakeExpect) Close() error { return nil }

type mrig struct {
	hist   *fakeHistory
	expect *fakeExpect
	sent   *notifytest.SentStore
	sender *notifytest.Sender
	clock  *clockwork.FakeClock
	n      notify.MorningNotifier
}

// klAt is a KL wall time.
func klAt(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, kl) }

func newMRig(t *testing.T, start time.Time, cfg notify.Config) *mrig {
	t.Helper()
	if cfg.Zone == nil {
		cfg.Zone = kl
	}
	r := &mrig{hist: &fakeHistory{}, expect: &fakeExpect{}, sent: &notifytest.SentStore{},
		sender: &notifytest.Sender{}, clock: clockwork.NewFakeClockAt(start)}
	n, err := core.NewMorningNotifier(notify.MorningDeps{History: r.hist, Expect: r.expect, Sent: r.sent}, r.sender, r.clock, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.n = n
	return r
}

func (r *mrig) tick(t *testing.T) bool {
	t.Helper()
	ok, err := r.n.NotifyMorning(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func (r *mrig) readNothing(t *testing.T) {
	t.Helper()
	if len(r.hist.since) != 0 || r.expect.reads != 0 || len(r.sent.Marks()) != 0 || len(r.sender.Texts()) != 0 {
		t.Fatalf("something was read or sent")
	}
}

func TestBeforeFromSendsNothing(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 7, 59), notify.Config{})
	if r.tick(t) {
		t.Fatal("sent before from")
	}
	r.readNothing(t)
}

func TestAfterUntilSendsNothing(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 12, 0), notify.Config{})
	if r.tick(t) {
		t.Fatal("sent at until")
	}
	r.readNothing(t)
}

func TestSendsOncePerKLDay(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 0), notify.Config{})
	if !r.tick(t) {
		t.Fatal("not sent at from")
	}
	if got := r.sent.Marks(); len(got) != 1 || got[0] != (store.SentMark{RuleID: "morning_report", Day: "2026-11-10"}) {
		t.Fatalf("marks = %v", got)
	}
	if got := r.hist.since[0]; !got.Equal(klAt(9, 8, 0)) {
		t.Fatalf("since = %v, want previous 08:00 KL", got)
	}
	if r.expect.from != "2026-11-03" || r.expect.to != "2026-11-09" {
		t.Fatalf("expectation range %s..%s", r.expect.from, r.expect.to)
	}
}

func TestSecondTickSameDayIsNoop(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 0), notify.Config{})
	r.tick(t)
	r.clock.Advance(time.Hour)
	if r.tick(t) {
		t.Fatal("sent twice")
	}
	if len(r.sender.Texts()) != 1 {
		t.Fatalf("texts = %d", len(r.sender.Texts()))
	}
}

func TestNextDaySendsAgain(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 9, 0), notify.Config{})
	r.tick(t)
	r.clock.Advance(24 * time.Hour)
	if !r.tick(t) {
		t.Fatal("next day not sent")
	}
	if got := r.sent.Marks(); len(got) != 2 || got[1].Day != "2026-11-11" {
		t.Fatalf("marks = %v", got)
	}
}

func TestQuietDayStillSends(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 30), notify.Config{})
	r.tick(t)
	texts := r.sender.Texts()
	if len(texts) != 1 || !strings.Contains(texts[0], "all quiet") || !strings.Contains(texts[0], "2026-11-10") {
		t.Fatalf("texts = %q", texts)
	}
}

func TestUnclearedGapsCounted(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 30), notify.Config{})
	r.expect.open = []store.Gap{{DatasetID: "secret1", Reason: "expired"}, {DatasetID: "secret2", Reason: "expired", ReportedAt: klAt(9, 9, 0)}}
	r.hist.rows = []gaphistory.Gap{{DatasetID: "pushed", Reason: "expired", ReportedAt: klAt(9, 10, 0), ClearedAt: klAt(10, 1, 0)}}
	r.tick(t)
	text := r.sender.Texts()[0]
	for _, s := range []string{"open gaps: 2", "cleared after push: 1"} {
		if !strings.Contains(text, s) {
			t.Fatalf("missing %q in\n%s", s, text)
		}
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "pushed") || strings.Contains(text, "all quiet") {
		t.Fatalf("pushed gap printed or quiet claimed:\n%s", text)
	}
}

func TestLateExpectationListed(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 30), notify.Config{})
	r.expect.exps = []store.Expectation{
		{DatasetID: "a", ExpectedDay: "2026-11-06", Gap: "late"},
		{DatasetID: "b", ExpectedDay: "2026-11-09", Gap: "late"},
		{DatasetID: "c", ExpectedDay: "2026-11-09", Gap: "late"},
		{DatasetID: "d", ExpectedDay: "2026-11-09"},
	}
	r.tick(t)
	if text := r.sender.Texts()[0]; !strings.Contains(text, "late datasets of 2026-11-09: 2") {
		t.Fatalf("text:\n%s", text)
	}
}

func TestSendFailureMarksNothing(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 30), notify.Config{})
	boom := errors.New("boom")
	r.sender.SetErr(boom)
	ok, err := r.n.NotifyMorning(context.Background())
	if ok || !errors.Is(err, boom) {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(r.sent.Marks()) != 0 {
		t.Fatal("marked after failed send")
	}
	r.sender.SetErr(nil)
	r.clock.Advance(time.Minute)
	if !r.tick(t) {
		t.Fatal("next tick did not retry")
	}
}

func TestMarkFailureReturnsError(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 30), notify.Config{})
	boom := errors.New("boom")
	r.sent.SetMarkErr(boom)
	_, err := r.n.NotifyMorning(context.Background())
	if !errors.Is(err, boom) || len(r.sender.Texts()) != 1 {
		t.Fatalf("err=%v texts=%d", err, len(r.sender.Texts()))
	}
}

// The notifier holds no way to mark a gap: its inputs have no such method, and a self-healed
// gap keeps ReportedAt unset in the history after the pass.
func TestNeverMarksGapReported(t *testing.T) {
	t.Parallel()
	r := newMRig(t, klAt(10, 8, 30), notify.Config{})
	r.hist.rows = []gaphistory.Gap{{DatasetID: "healed", Reason: "expired", ClearedAt: klAt(10, 2, 0)}}
	r.tick(t)
	if !strings.Contains(r.sender.Texts()[0], "dataset healed") {
		t.Fatalf("not listed:\n%s", r.sender.Texts()[0])
	}
	if !r.hist.rows[0].ReportedAt.IsZero() {
		t.Fatal("gap marked reported")
	}
	if got := r.sent.Marks(); len(got) != 1 || got[0].RuleID != "morning_report" {
		t.Fatalf("marks = %v", got)
	}
}

func TestZoneIsKL(t *testing.T) {
	t.Parallel()
	// 2026-11-09 23:30 UTC is 2026-11-10 07:30 KL: before from. 00:10 UTC next is 08:10 KL.
	r := newMRig(t, time.Date(2026, 11, 9, 23, 30, 0, 0, time.UTC), notify.Config{})
	if r.tick(t) {
		t.Fatal("sent at 07:30 KL")
	}
	r.clock.Advance(40 * time.Minute)
	if !r.tick(t) {
		t.Fatal("not sent at 08:10 KL")
	}
	if got := r.sent.Marks(); got[0].Day != "2026-11-10" {
		t.Fatalf("day = %s, want the KL date", got[0].Day)
	}
}
