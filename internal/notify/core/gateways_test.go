package core_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/notify/notifytest"
	"github.com/jonboulle/clockwork"
)

type gwRig struct {
	sent   *notifytest.SentStore
	sender *notifytest.Sender
	n      notify.GatewayNotifier
}

func newGwRig(t *testing.T) *gwRig {
	t.Helper()
	r := &gwRig{sent: &notifytest.SentStore{}, sender: &notifytest.Sender{}}
	n, err := core.NewGateways(r.sender, r.sent, clockwork.NewFakeClockAt(time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	r.n = n
	return r
}

func (r *gwRig) pass(t *testing.T, facts ...notify.GatewayFact) int {
	t.Helper()
	got, err := r.n.NotifyGateways(context.Background(), facts)
	if err != nil {
		t.Fatalf("NotifyGateways: %v", err)
	}
	return got
}

var gwSince = time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)

func down(since time.Time) notify.GatewayFact {
	return notify.GatewayFact{Gateway: "main", Farm: "usfarm", Since: since, Down: true}
}

var up = notify.GatewayFact{Gateway: "main"}

func TestGatewayOncePerIncident(t *testing.T) {
	t.Parallel()
	r := newGwRig(t)
	r.pass(t, down(gwSince))
	r.pass(t, down(gwSince))
	if got := len(r.sender.Texts()); got != 1 {
		t.Fatalf("two passes on one incident sent %d texts, want 1", got)
	}
	r.pass(t, up)
	r.pass(t, down(gwSince.Add(time.Hour)))
	if got := len(r.sender.Texts()); got != 3 { // incident, recovery, second incident
		t.Fatalf("texts = %d, want 3", got)
	}
}

func TestGatewayRecoveryOnce(t *testing.T) {
	t.Parallel()
	r := newGwRig(t)
	r.pass(t, up)
	if len(r.sender.Texts()) != 0 {
		t.Fatal("recovery without incident sent a text")
	}
	r.pass(t, down(gwSince))
	r.pass(t, up)
	texts := r.sender.Texts()
	if len(texts) != 2 || !strings.Contains(texts[1], "已恢复") {
		t.Fatalf("texts = %v, want incident then recovery", texts)
	}
	r.pass(t, up)
	if got := len(r.sender.Texts()); got != 2 {
		t.Fatalf("second Down:false pass sent; texts = %d", got)
	}
}

func TestGatewayMarkAfterSend(t *testing.T) {
	t.Parallel()
	r := newGwRig(t)
	r.sender.SetErr(errors.New("boom"))
	if _, err := r.n.NotifyGateways(context.Background(), []notify.GatewayFact{down(gwSince)}); err == nil {
		t.Fatal("failed send returned no error")
	}
	if len(r.sent.Marks()) != 0 {
		t.Fatalf("failed send left marks: %v", r.sent.Marks())
	}
	r.sender.SetErr(nil)
	if got := r.pass(t, down(gwSince)); got != 1 {
		t.Fatalf("next pass sent %d, want 1", got)
	}
	if len(r.sent.Marks()) != 1 {
		t.Fatalf("marks = %v, want 1", r.sent.Marks())
	}
}

func TestGatewayText(t *testing.T) {
	t.Parallel()
	r := newGwRig(t)
	r.pass(t, down(gwSince))
	text := r.sender.Texts()[0]
	for _, want := range []string{"main", "重新登录"} {
		if !strings.Contains(text, want) {
			t.Errorf("text %q lacks %q", text, want)
		}
	}
	if strings.ContainsAny(text, "0123456789") {
		t.Errorf("text %q holds digits (account id or port)", text)
	}
}
