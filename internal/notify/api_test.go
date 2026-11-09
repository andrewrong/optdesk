package notify_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

type fakeSender struct{ texts []string }

func (f *fakeSender) Send(_ context.Context, text string) error {
	f.texts = append(f.texts, text)
	return nil
}

type fakeSource struct {
	gaps   []store.Gap
	marked []store.Gap
	at     time.Time
}

func (f *fakeSource) OpenGaps(context.Context) ([]store.Gap, error) { return f.gaps, nil }

func (f *fakeSource) MarkGapReported(_ context.Context, g store.Gap, at time.Time) error {
	f.marked = append(f.marked, g)
	f.at = at
	return nil
}

type fakeNotifier struct {
	src    notify.GapSource
	sender notify.Sender
}

func (n *fakeNotifier) NotifyGaps(ctx context.Context) (int, error) {
	gaps, err := n.src.OpenGaps(ctx)
	if err != nil || len(gaps) == 0 {
		return 0, err
	}
	if err := n.sender.Send(ctx, "text"); err != nil {
		return 0, err
	}
	for _, g := range gaps {
		if err := n.src.MarkGapReported(ctx, g, time.Unix(1, 0)); err != nil {
			return 0, err
		}
	}
	return len(gaps), nil
}

var _ notify.NewNotifierFunc = func(src notify.GapSource, s notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.Notifier, error) {
	return &fakeNotifier{src: src, sender: s}, nil
}

var _ notify.NewTelegramSenderFunc = func(string, string, notify.TelegramOptions) (notify.Sender, error) {
	return &fakeSender{}, nil
}

var _ notify.NewDryRunSenderFunc = func(io.Writer) notify.Sender { return &fakeSender{} }

func TestNotifierContract(t *testing.T) {
	t.Parallel()
	src := &fakeSource{gaps: []store.Gap{{DatasetID: "d", Day: "2026-11-04", Reason: "expired"}}}
	snd := &fakeSender{}
	var newFn notify.NewNotifierFunc = func(g notify.GapSource, s notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.Notifier, error) {
		return &fakeNotifier{src: g, sender: s}, nil
	}
	n, err := newFn(src, snd, clockwork.NewFakeClock(), notify.Config{QuietFrom: notify.DefaultQuietFrom, QuietTo: notify.DefaultQuietTo, Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	got, err := n.NotifyGaps(context.Background())
	if err != nil || got != 1 {
		t.Fatalf("NotifyGaps = %d, %v", got, err)
	}
	if len(snd.texts) != 1 || len(src.marked) != 1 || src.marked[0].DatasetID != "d" {
		t.Fatalf("texts = %v, marked = %v", snd.texts, src.marked)
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()
	for name, c := range map[string][2]string{
		"token": {notify.TokenEnv, "OT_TELEGRAM_TOKEN"},
		"chat":  {notify.ChatIDEnv, "OT_TELEGRAM_CHAT_ID"},
		"from":  {notify.DefaultQuietFrom, "23:00"},
		"to":    {notify.DefaultQuietTo, "07:00"},
	} {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}

func TestSenderAndOptionsContract(t *testing.T) {
	t.Parallel()
	var got notify.TelegramOptions
	var newFn notify.NewTelegramSenderFunc = func(token, chatID string, o notify.TelegramOptions) (notify.Sender, error) {
		got = o
		if token == "" || chatID == "" {
			return nil, notify.ErrSend
		}
		return &fakeSender{}, nil
	}
	want := notify.TelegramOptions{ServerURL: "http://127.0.0.1:1", HTTPClient: &http.Client{}}
	s, err := newFn("test-token", "1000001", want)
	if err != nil || got != want {
		t.Fatalf("sender %v, err %v, options %+v", s, err, got)
	}
	if err := s.Send(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := newFn("", "", want); err != notify.ErrSend {
		t.Fatalf("err = %v, want ErrSend", err)
	}
}

type fakeSentStore struct {
	marks  []store.SentMark
	marked []store.SentMark
}

func (f *fakeSentStore) SentSince(context.Context, contracts.Day) ([]store.SentMark, error) {
	return f.marks, nil
}

func (f *fakeSentStore) MarkSent(_ context.Context, ruleID string, day contracts.Day) error {
	f.marked = append(f.marked, store.SentMark{RuleID: ruleID, Day: day})
	return nil
}

func (f *fakeSentStore) Close() error { return nil }

type fakeAlertNotifier struct {
	deps   notify.AlertDeps
	sender notify.Sender
}

func (n *fakeAlertNotifier) NotifyAlerts(ctx context.Context) (int, error) {
	today := contracts.Day("2026-11-06")
	res, err := n.deps.Evaluate(ctx, n.deps.Rules, n.deps.Read, today)
	if err != nil {
		return 0, err
	}
	var fired []compute.AlertResult
	for _, r := range res {
		if r.State == "fired" && r.Public {
			fired = append(fired, r)
		}
	}
	if len(fired) == 0 {
		return 0, nil
	}
	if err := n.sender.Send(ctx, "optdesk: 1 market alert"); err != nil {
		return 0, err
	}
	for _, r := range fired {
		if err := n.deps.Sent.MarkSent(ctx, r.RuleID, r.Day); err != nil {
			return 0, err
		}
	}
	return len(fired), nil
}

var _ notify.NewAlertNotifierFunc = func(d notify.AlertDeps, s notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.AlertNotifier, error) {
	return &fakeAlertNotifier{deps: d, sender: s}, nil
}

func TestAlertNotifierContract(t *testing.T) {
	t.Parallel()
	sent := &fakeSentStore{}
	snd := &fakeSender{}
	deps := notify.AlertDeps{
		Evaluate: func(context.Context, compute.AlertRules, compute.ReadingFunc, contracts.Day) ([]compute.AlertResult, error) {
			return []compute.AlertResult{
				{RuleID: "r1", State: "fired", Public: true, Day: "2026-11-06"},
				{RuleID: "r2", State: "fired", Public: false, Day: "2026-11-06"},
				{RuleID: "r3", State: "clear", Public: true, Day: "2026-11-06"},
			}, nil
		},
		Sent: sent,
	}
	var newFn notify.NewAlertNotifierFunc = func(d notify.AlertDeps, s notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.AlertNotifier, error) {
		return &fakeAlertNotifier{deps: d, sender: s}, nil
	}
	n, err := newFn(deps, snd, clockwork.NewFakeClock(), notify.Config{Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	got, err := n.NotifyAlerts(context.Background())
	if err != nil || got != 1 {
		t.Fatalf("NotifyAlerts = %d, %v", got, err)
	}
	if len(snd.texts) != 1 || len(sent.marked) != 1 || sent.marked[0].RuleID != "r1" {
		t.Fatalf("texts = %v, marked = %v", snd.texts, sent.marked)
	}
}

type fakeHistory struct{ gaps []gaphistory.Gap }

func (f *fakeHistory) GapsSince(context.Context, time.Time) ([]gaphistory.Gap, error) {
	return f.gaps, nil
}

func (f *fakeHistory) Close() error { return nil }

type fakeExpect struct{}

func (fakeExpect) OpenGaps(context.Context) ([]store.Gap, error) { return nil, nil }
func (fakeExpect) Expectations(context.Context, contracts.Day, contracts.Day) ([]store.Expectation, error) {
	return nil, nil
}
func (fakeExpect) Close() error { return nil }

type fakeMorning struct {
	deps   notify.MorningDeps
	sender notify.Sender
}

func (n *fakeMorning) NotifyMorning(ctx context.Context) (bool, error) {
	day := contracts.Day("2026-11-09")
	marks, err := n.deps.Sent.SentSince(ctx, day)
	if err != nil {
		return false, err
	}
	for _, m := range marks {
		if m.RuleID == notify.MorningRuleID && m.Day == day {
			return false, nil
		}
	}
	if _, err := n.deps.History.GapsSince(ctx, time.Unix(0, 0)); err != nil {
		return false, err
	}
	if err := n.sender.Send(ctx, "morning"); err != nil {
		return false, err
	}
	return true, n.deps.Sent.MarkSent(ctx, notify.MorningRuleID, day)
}

func TestMorningContract(t *testing.T) {
	t.Parallel()
	if notify.DefaultMorningFrom != "08:00" || notify.DefaultMorningUntil != "12:00" || notify.MorningRuleID != "morning_report" {
		t.Fatal("morning constants changed")
	}
	cfg := notify.Config{MorningFrom: "08:30", MorningUntil: "11:00", Zone: time.UTC}
	var newFn notify.NewMorningNotifierFunc = func(d notify.MorningDeps, s notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.MorningNotifier, error) {
		return &fakeMorning{deps: d, sender: s}, nil
	}
	sent := &fakeSentStore{}
	snd := &fakeSender{}
	deps := notify.MorningDeps{History: &fakeHistory{}, Expect: fakeExpect{}, Sent: sent}
	n, err := newFn(deps, snd, clockwork.NewFakeClock(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := n.NotifyMorning(context.Background())
	if err != nil || !ok {
		t.Fatalf("NotifyMorning = %v, %v", ok, err)
	}
	if len(snd.texts) != 1 || len(sent.marked) != 1 || sent.marked[0].RuleID != notify.MorningRuleID {
		t.Fatalf("texts = %v, marked = %v", snd.texts, sent.marked)
	}
	sent.marks = sent.marked
	ok, err = n.NotifyMorning(context.Background())
	if err != nil || ok || len(snd.texts) != 1 {
		t.Fatalf("second NotifyMorning = %v, %v, texts %d", ok, err, len(snd.texts))
	}
}
