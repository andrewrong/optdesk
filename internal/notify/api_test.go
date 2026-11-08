package notify_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

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
