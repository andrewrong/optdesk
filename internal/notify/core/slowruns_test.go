package core

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/notifytest"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
	"github.com/jonboulle/clockwork"
)

// slowLedger is a fake ledger: Running returns the rows with no mark yet, as the real view does.
type slowLedger struct {
	mu   sync.Mutex
	rows []jobrun.JobRun
}

func (l *slowLedger) Running(context.Context) ([]jobrun.JobRun, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]jobrun.JobRun(nil), l.rows...), nil
}
func (l *slowLedger) Usuals(context.Context) ([]jobrun.UsualStat, error) { return nil, nil }
func (l *slowLedger) MarkSlowNotified(_ context.Context, id string, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.rows {
		if l.rows[i].ID == id {
			l.rows[i].SlowNotifiedAt = at
		}
	}
	return nil
}

type slowRig struct {
	ledger *slowLedger
	sender *notifytest.Sender
	clock  *clockwork.FakeClock
	logs   *bytes.Buffer
	n      notify.SlowNotifier
}

func newSlowRig(t *testing.T, start time.Time) *slowRig {
	t.Helper()
	r := &slowRig{
		ledger: &slowLedger{rows: []jobrun.JobRun{{
			ID: "r1", JobID: "job-a", Source: "scheduler", StartedAt: start.Add(-2 * time.Hour),
			KeysDone: 3, KeysTotal: 9, Step: "SECRET-KEY",
		}}},
		sender: &notifytest.Sender{},
		clock:  clockwork.NewFakeClockAt(start),
		logs:   &bytes.Buffer{},
	}
	figs := func(_ time.Time, runs []jobrunapi.Input, _ []jobrunapi.Usual) []jobrunapi.Figure {
		out := make([]jobrunapi.Figure, len(runs))
		for i, in := range runs {
			out[i] = jobrunapi.Figure{JobID: in.JobID, ElapsedMinutes: 120, Slow: true}
		}
		return out
	}
	n, err := NewSlow(notify.SlowDeps{Runs: r.ledger, Mark: r.ledger, Figures: figs}, r.sender, r.clock, notify.Config{Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	n.(*slowNotifier).log = slog.New(slog.NewTextHandler(r.logs, nil))
	r.n = n
	return r
}

func TestOneNoticePerRun(t *testing.T) {
	t.Parallel()
	r := newSlowRig(t, time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC))
	total := 0
	for range 5 {
		n, err := r.n.NotifySlowRuns(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		total += n
	}
	if total != 1 || len(r.sender.Texts()) != 1 {
		t.Fatalf("reported %d, sent %d, want 1 and 1", total, len(r.sender.Texts()))
	}
	if c := strings.Count(r.logs.String(), "job_run_slow"); c != 1 {
		t.Fatalf("log lines = %d, want 1: %s", c, r.logs.String())
	}
	for _, want := range []string{"job=job-a", "elapsed_s=7200", "usual_s=0", "keys_done=3", "keys_total=9", "source=scheduler"} {
		if !strings.Contains(r.logs.String(), want) {
			t.Errorf("log lacks %s: %s", want, r.logs.String())
		}
	}
	if strings.Contains(r.logs.String(), "SECRET-KEY") || strings.Contains(r.sender.Texts()[0], "SECRET-KEY") {
		t.Fatal("key text leaked")
	}
}

func TestNoMarkWhenSendFails(t *testing.T) {
	t.Parallel()
	r := newSlowRig(t, time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC))
	r.sender.SetErr(errors.New("down"))
	if n, err := r.n.NotifySlowRuns(context.Background()); err == nil || n != 0 {
		t.Fatalf("got %d, %v; want 0 and an error", n, err)
	}
	if !r.ledger.rows[0].SlowNotifiedAt.IsZero() {
		t.Fatal("run marked after a failed send")
	}
	if r.logs.Len() != 0 {
		t.Fatalf("log line for an unsent notice: %s", r.logs.String())
	}
	r.sender.SetErr(nil)
	if n, err := r.n.NotifySlowRuns(context.Background()); err != nil || n != 1 {
		t.Fatalf("retry got %d, %v; want 1", n, err)
	}
}

func TestQuietWindowSendsNothing(t *testing.T) {
	t.Parallel()
	r := newSlowRig(t, time.Date(2026, 11, 9, 2, 0, 0, 0, time.UTC))
	if n, err := r.n.NotifySlowRuns(context.Background()); err != nil || n != 0 {
		t.Fatalf("got %d, %v; want 0", n, err)
	}
	if len(r.sender.Texts()) != 0 || !r.ledger.rows[0].SlowNotifiedAt.IsZero() {
		t.Fatal("quiet window sent or marked")
	}
	r.clock.Advance(7 * time.Hour)
	if n, _ := r.n.NotifySlowRuns(context.Background()); n != 1 {
		t.Fatalf("after the window got %d, want 1", n)
	}
}
