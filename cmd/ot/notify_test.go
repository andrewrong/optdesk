package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/notify/dryrun"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// notifyLedger is a scheduler ledger with open gaps; it records the marks.
type notifyLedger struct {
	store.SchedulerLedger
	mu     sync.Mutex
	gaps   []store.Gap
	marked []store.Gap
	closed int
}

func (l *notifyLedger) Status(context.Context) (store.SchedulerStatus, error) {
	return store.SchedulerStatus{Gaps: l.gaps}, nil
}

func (l *notifyLedger) MarkGapReported(_ context.Context, g store.Gap) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.marked = append(l.marked, g)
	return nil
}

func (l *notifyLedger) Close() error { l.mu.Lock(); l.closed++; l.mu.Unlock(); return nil }

type notifyRig struct {
	a     *app
	out   *bytes.Buffer
	errb  *bytes.Buffer
	sl    *notifyLedger
	fence *fakeInstFence
	sent  *[]string
	sendE error
}

// newNotifyRig is an app whose stores and notify constructors are fakes; the notifier is the real core over a
// fake sender, and the dry-run sender is the real one.
func newNotifyRig(t *testing.T, env string) *notifyRig {
	t.Helper()
	a, errb := testApp(t, env)
	r := &notifyRig{a: a, out: &bytes.Buffer{}, errb: errb, fence: &fakeInstFence{}, sent: &[]string{}}
	r.sl = &notifyLedger{gaps: []store.Gap{{JobID: "fred_series", DatasetID: "fred.DGS10", Day: "2026-11-05", Reason: "expired"}}}
	a.stdout = r.out
	var opened, closed atomic.Int32
	var day contracts.Day
	runOnceFakes(a, &opened, &closed, &day)
	d := &a.deps
	d.openFence = func(context.Context, string) (store.InstanceFence, error) {
		t.Error("notify opened the lock")
		return r.fence, nil
	}
	d.newHandler = func([]byte) http.Handler { t.Error("notify built the site"); return nil }
	d.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClockAt(time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)) }
	d.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return r.sl, nil }
	d.newNotifier = core.New
	d.newDryRunSender = dryrun.New
	d.newTelegramSender = func(token, chat string, _ notify.TelegramOptions) (notify.Sender, error) {
		return senderFunc(func(_ context.Context, text string) error {
			*r.sent = append(*r.sent, text)
			return r.sendE
		}), nil
	}
	return r
}

type senderFunc func(context.Context, string) error

func (f senderFunc) Send(ctx context.Context, text string) error { return f(ctx, text) }

const notifyEnv = fullEnv + telegramEnv

func TestNotifyDryRunMarksNothing(t *testing.T) {
	t.Parallel()
	r := newNotifyRig(t, fullEnv) // a dry run needs no token
	if err := notifyCmd(r.a, []string{"--once", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if len(r.sl.marked) != 0 {
		t.Fatalf("marked %v", r.sl.marked)
	}
	if !strings.Contains(r.out.String(), "optdesk") || len(*r.sent) != 0 {
		t.Fatalf("stdout %q, sent %v", r.out.String(), *r.sent)
	}
}

func TestNotifyOnceExits(t *testing.T) {
	t.Parallel()
	r := newNotifyRig(t, notifyEnv)
	if err := notifyCmd(r.a, []string{"--once"}); err != nil {
		t.Fatal(err)
	}
	if len(*r.sent) != 1 || len(r.sl.marked) != 1 || r.sl.closed != 1 {
		t.Fatalf("sent %d, marked %d, ledger closed %d, want 1, 1, 1", len(*r.sent), len(r.sl.marked), r.sl.closed)
	}
	for _, args := range [][]string{nil, {"--dry-run"}, {"--once", "extra"}} {
		if err := notifyCmd(r.a, args); err == nil || !strings.Contains(err.Error(), "usage") {
			t.Fatalf("args %v: err %v, want usage", args, err)
		}
	}
}

func TestNotifyQuietFlagKeepsQuietHours(t *testing.T) {
	t.Parallel()
	r := newNotifyRig(t, notifyEnv)
	night := time.Date(2026, 11, 6, 23, 30, 0, 0, time.UTC)
	r.a.deps.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClockAt(night) }
	if err := notifyCmd(r.a, []string{"--once", "--quiet"}); err != nil {
		t.Fatal(err)
	}
	if len(*r.sent) != 0 || len(r.sl.marked) != 0 {
		t.Fatalf("a quiet pass sent %d and marked %d at 23:30", len(*r.sent), len(r.sl.marked))
	}
	if err := notifyCmd(r.a, []string{"--once"}); err != nil || len(*r.sent) != 1 {
		t.Fatalf("err %v, sent %d: the hand pass without --quiet must send at 23:30", err, len(*r.sent))
	}
}

func TestNotifyMissingTokenNamesVariableOnly(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ env, want, secret string }{
		"token":   {fullEnv + "OT_TELEGRAM_CHAT_ID=1000001\n", "OT_TELEGRAM_TOKEN", "1000001"},
		"chat id": {fullEnv + "OT_TELEGRAM_TOKEN=test-token\n", "OT_TELEGRAM_CHAT_ID", "test-token"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newNotifyRig(t, c.env)
			err := notifyCmd(r.a, []string{"--once"})
			if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), c.secret) || strings.Contains(err.Error(), "dsn") {
				t.Fatalf("err %v", err)
			}
			if len(*r.sent) != 0 {
				t.Fatal("sent")
			}
		})
	}
}

func TestNotifyErrorsHoldNoSecret(t *testing.T) {
	t.Parallel()
	r := newNotifyRig(t, notifyEnv)
	r.sendE = errors.Join(notify.ErrSend, errors.New("POST https://api.telegram.org/bottest-token/sendMessage chat 1000001 run-dsn"))
	err := notifyCmd(r.a, []string{"--once"})
	if err == nil {
		t.Fatal("no error")
	}
	for _, s := range []string{"test-token", "1000001", "run-dsn", "telegram.org"} {
		if strings.Contains(err.Error(), s) || strings.Contains(r.errb.String(), s) || strings.Contains(r.out.String(), s) {
			t.Fatalf("%q leaked: %v / %q", s, err, r.errb.String())
		}
	}
	if len(r.sl.marked) != 0 {
		t.Fatal("a failed send marked a gap")
	}
}

func TestNotifyDoesNotStartTheSite(t *testing.T) {
	t.Parallel()
	r := newNotifyRig(t, notifyEnv) // newHandler of the rig fails the test when called
	if err := notifyCmd(r.a, []string{"--once", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if err := notifyCmd(r.a, []string{"--once"}); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyNoSchedulerLock(t *testing.T) {
	t.Parallel()
	r := newNotifyRig(t, notifyEnv) // openFence of the rig fails the test when called
	r.fence.held = true             // a running scheduler must not stop the hand pass
	if err := notifyCmd(r.a, []string{"--once"}); err != nil {
		t.Fatal(err)
	}
	if r.fence.count() != 0 {
		t.Fatalf("TryLock called %d times", r.fence.count())
	}
}

func TestLedgerGapsAdapter(t *testing.T) {
	t.Parallel()
	l := &notifyLedger{gaps: []store.Gap{{JobID: "j"}}}
	var src notify.GapSource = ledgerGaps{l}
	gs, err := src.OpenGaps(t.Context())
	if err != nil || len(gs) != 1 || gs[0].JobID != "j" {
		t.Fatalf("gaps %v, err %v", gs, err)
	}
	if err := src.MarkGapReported(t.Context(), gs[0], time.Time{}); err != nil || len(l.marked) != 1 {
		t.Fatalf("marked %v, err %v", l.marked, err)
	}
	ro := readOnlyGaps{src}
	if err := ro.MarkGapReported(t.Context(), gs[0], time.Time{}); err != nil || len(l.marked) != 1 {
		t.Fatalf("the read-only view marked: %v", l.marked)
	}
}

var _ runner.GapSyncer = notifyingGaps{}

// The leak table of env_test.go is owned by another file of the package; the row of ot notify joins it here.
func init() { leakRows["notify"] = []string{"--once"} }

func TestAlertAndGapNotifiersShareOneConfig(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	done := r.start()
	r.waitStarted(t, done)
	_ = r.stop(t, done)
	if r.gapCfg.Zone == nil || r.gapCfg != r.alertCfg {
		t.Fatalf("gap config %+v, alert config %+v", r.gapCfg, r.alertCfg)
	}
	if r.alertCfg.QuietFrom != "" || r.alertCfg.QuietTo != "" {
		t.Fatalf("the scheduler turned the quiet hours off: %+v", r.alertCfg)
	}
}
