package core_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/notify/notifytest"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

type alertRig struct {
	sent    *notifytest.SentStore
	sender  *notifytest.Sender
	clock   *clockwork.FakeClock
	n       notify.AlertNotifier
	results []compute.AlertResult
	evals   int
	today   contracts.Day
}

func newAlertRig(t *testing.T, start time.Time, cfg notify.Config, rs ...compute.AlertResult) *alertRig {
	t.Helper()
	if cfg.Zone == nil {
		cfg.Zone = utc
	}
	r := &alertRig{sent: &notifytest.SentStore{}, sender: &notifytest.Sender{},
		clock: clockwork.NewFakeClockAt(start), results: rs}
	deps := notify.AlertDeps{
		Sent: r.sent,
		Read: func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
			return nil, nil
		},
		Evaluate: func(_ context.Context, _ compute.AlertRules, _ compute.ReadingFunc, today contracts.Day) ([]compute.AlertResult, error) {
			r.evals++
			r.today = today
			return r.results, nil
		},
	}
	n, err := core.NewAlerts(deps, r.sender, r.clock, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.n = n
	return r
}

func fired(id, level string) compute.AlertResult {
	return compute.AlertResult{RuleID: id, Title: "Title " + id, Level: level, State: "fired", Public: true,
		Detail: "VIX 31.5", Day: "2026-11-09", DataDay: "2026-11-08"}
}

func TestAlertMarkOnlyAfterSend(t *testing.T) {
	t.Parallel()
	r := newAlertRig(t, noon(), notify.Config{}, fired("a", "w"))
	var atSend int
	r.sender.OnSend = func() { atSend = len(r.sent.Marks()) }
	n, err := r.n.NotifyAlerts(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("got %d, %v", n, err)
	}
	if atSend != 0 || len(r.sent.Marks()) != 1 {
		t.Fatalf("marks at send %d, after %d", atSend, len(r.sent.Marks()))
	}
	if m := r.sent.Marks()[0]; m.RuleID != "a" || m.Day != "2026-11-09" {
		t.Fatalf("mark %+v", m)
	}
}

func TestAlertFailedSendMarksNothing(t *testing.T) {
	t.Parallel()
	r := newAlertRig(t, noon(), notify.Config{}, fired("a", "w"))
	boom := errors.New("boom")
	r.sender.SetErr(boom)
	n, err := r.n.NotifyAlerts(context.Background())
	if !errors.Is(err, boom) || n != 0 || len(r.sent.Marks()) != 0 {
		t.Fatalf("got %d, %v, marks %d", n, err, len(r.sent.Marks()))
	}
}

func TestAlertOneMessagePerBatch(t *testing.T) {
	t.Parallel()
	r := newAlertRig(t, noon(), notify.Config{}, fired("a", "w"), fired("b", "n"), fired("c", "d"))
	n, err := r.n.NotifyAlerts(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("got %d, %v", n, err)
	}
	texts := r.sender.Texts()
	if len(texts) != 1 || !strings.HasPrefix(texts[0], "optdesk: 3 market alert(s)\n") {
		t.Fatalf("texts %q", texts)
	}
}

func TestAlertSentNotRepeated(t *testing.T) {
	t.Parallel()
	r := newAlertRig(t, noon(), notify.Config{}, fired("a", "w"))
	ctx := context.Background()
	if _, err := r.n.NotifyAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := r.n.NotifyAlerts(ctx)
	if err != nil || n != 0 || len(r.sender.Texts()) != 1 {
		t.Fatalf("got %d, %v, texts %d", n, err, len(r.sender.Texts()))
	}
	r.sent.Seed(store.SentMark{RuleID: "z", Day: "2026-11-09"})
	r.results = []compute.AlertResult{fired("z", "n")}
	if n, _ := r.n.NotifyAlerts(ctx); n != 0 {
		t.Fatalf("seeded mark was said again")
	}
}

func TestAlertQuietHoursHold(t *testing.T) {
	t.Parallel()
	zone := time.FixedZone("Etc/GMT-8", 8*3600)
	// 15:30 UTC is 23:30 in Etc/GMT-8: inside the default window.
	r := newAlertRig(t, time.Date(2026, 11, 9, 15, 30, 0, 0, utc), notify.Config{Zone: zone}, fired("a", "d"))
	n, err := r.n.NotifyAlerts(context.Background())
	if err != nil || n != 0 || r.evals != 0 || len(r.sender.Texts()) != 0 {
		t.Fatalf("got %d, %v, evals %d", n, err, r.evals)
	}
}

func TestAlertHeldSentAtSeven(t *testing.T) {
	t.Parallel()
	zone := time.FixedZone("Etc/GMT-8", 8*3600)
	// 22:59 UTC is 06:59 next day in Etc/GMT-8.
	r := newAlertRig(t, time.Date(2026, 11, 9, 22, 59, 0, 0, utc), notify.Config{Zone: zone}, fired("a", "d"))
	ctx := context.Background()
	if n, _ := r.n.NotifyAlerts(ctx); n != 0 || len(r.sender.Texts()) != 0 {
		t.Fatalf("sent in quiet hours")
	}
	r.clock.Advance(time.Minute)
	n, err := r.n.NotifyAlerts(ctx)
	if err != nil || n != 1 || len(r.sender.Texts()) != 1 {
		t.Fatalf("got %d, %v", n, err)
	}
	if r.today != "2026-11-10" {
		t.Fatalf("today %s, want the day in the zone", r.today)
	}
}

func TestAlertQuietWindowCrossesMidnight(t *testing.T) {
	t.Parallel()
	cfg := notify.Config{QuietFrom: "22:00", QuietTo: "06:00"}
	for hour, wantSent := range map[int]bool{21: true, 23: false, 3: false, 6: true} {
		r := newAlertRig(t, time.Date(2026, 11, 9, hour, 0, 0, 0, utc), cfg, fired("a", "n"))
		n, err := r.n.NotifyAlerts(context.Background())
		if err != nil || (n == 1) != wantSent {
			t.Fatalf("hour %d: got %d, %v", hour, n, err)
		}
	}
}

func TestAlertTextHoldsNoSentinel(t *testing.T) {
	t.Parallel()
	r := newAlertRig(t, noon(), notify.Config{}, fired("a", "w"))
	if _, err := r.n.NotifyAlerts(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := "optdesk: 1 market alert(s)\nW Title a: VIX 31.5 (data of 2026-11-08) 2026-11-09"
	if got := r.sender.Texts()[0]; got != want {
		t.Fatalf("got %q", got)
	}
}

func TestAlertTextCapsLines(t *testing.T) {
	t.Parallel()
	var rs []compute.AlertResult
	for i := 0; i < 25; i++ {
		rs = append(rs, fired(fmt.Sprintf("r%02d", i), "n"))
	}
	r := newAlertRig(t, noon(), notify.Config{}, rs...)
	text := ""
	n, err := r.n.NotifyAlerts(context.Background())
	if err != nil || n != 20 {
		t.Fatalf("got %d, %v", n, err)
	}
	text = r.sender.Texts()[0]
	lines := strings.Split(text, "\n")
	if len(lines) != 22 || lines[21] != "and 5 more" || !strings.HasPrefix(lines[0], "optdesk: 25 market") || len(text) > 3500 {
		t.Fatalf("lines %d, last %q", len(lines), lines[len(lines)-1])
	}
}

func TestAlertTextOrder(t *testing.T) {
	t.Parallel()
	late := fired("a", "d")
	late.Day = "2026-11-10"
	r := newAlertRig(t, noon(), notify.Config{}, fired("c", "n"), fired("b", "w"), late, fired("a2", "d"), fired("a1", "d"))
	if _, err := r.n.NotifyAlerts(context.Background()); err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, l := range strings.Split(r.sender.Texts()[0], "\n")[1:] {
		titles = append(titles, strings.SplitN(l, ":", 2)[0])
	}
	want := "D Title a1,D Title a2,D Title a,W Title b,N Title c"
	if got := strings.Join(titles, ","); got != want {
		t.Fatalf("got %s", got)
	}
}

func TestAlertTextShowsDataDay(t *testing.T) {
	t.Parallel()
	x := fired("a", "n")
	x.DataDay = "2026-11-03"
	r := newAlertRig(t, noon(), notify.Config{}, x)
	if _, err := r.n.NotifyAlerts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := r.sender.Texts()[0]; !strings.Contains(got, "(data of 2026-11-03)") {
		t.Fatalf("got %q", got)
	}
}

func TestAlertBackoffDoubles(t *testing.T) {
	t.Parallel()
	r := newAlertRig(t, noon(), notify.Config{}, fired("a", "n"))
	ctx := context.Background()
	r.sender.SetErr(errors.New("down"))
	for _, step := range []time.Duration{15 * time.Minute, 30 * time.Minute, 60 * time.Minute, 60 * time.Minute} {
		before := len(r.sender.Texts())
		if _, err := r.n.NotifyAlerts(ctx); err == nil {
			t.Fatal("want error")
		}
		r.clock.Advance(step - time.Second)
		if _, err := r.n.NotifyAlerts(ctx); err != nil || len(r.sender.Texts()) != before+1 {
			t.Fatalf("step %v: retried too early (%v)", step, err)
		}
		r.clock.Advance(time.Second)
	}
	r.sender.SetErr(nil)
	if n, err := r.n.NotifyAlerts(ctx); err != nil || n != 1 {
		t.Fatalf("got %d, %v", n, err)
	}
}

func TestAlertMarkFailureRetried(t *testing.T) {
	t.Parallel()
	r := newAlertRig(t, noon(), notify.Config{}, fired("a", "n"))
	ctx := context.Background()
	r.sent.SetMarkErr(errors.New("db"))
	if n, err := r.n.NotifyAlerts(ctx); err == nil || n != 0 {
		t.Fatalf("got %d, %v", n, err)
	}
	r.sent.SetMarkErr(nil)
	n, err := r.n.NotifyAlerts(ctx)
	if err != nil || n != 1 || len(r.sender.Texts()) != 2 || len(r.sent.Marks()) != 1 {
		t.Fatalf("got %d, %v, texts %d", n, err, len(r.sender.Texts()))
	}
}

func TestAlertCapMarksOnlyPrinted(t *testing.T) {
	t.Parallel()
	var rs []compute.AlertResult
	for i := 0; i < 22; i++ {
		rs = append(rs, fired(fmt.Sprintf("r%02d", i), "n"))
	}
	r := newAlertRig(t, noon(), notify.Config{}, rs...)
	if _, err := r.n.NotifyAlerts(context.Background()); err != nil {
		t.Fatal(err)
	}
	marks := r.sent.Marks()
	if len(marks) != 20 {
		t.Fatalf("marks %d", len(marks))
	}
	for _, m := range marks {
		if m.RuleID == "r20" || m.RuleID == "r21" {
			t.Fatalf("marked unprinted %s", m.RuleID)
		}
	}
}

func TestAlertNonPublicSubjectNeverSent(t *testing.T) {
	t.Parallel()
	x := fired("a", "d")
	x.Public = false
	r := newAlertRig(t, noon(), notify.Config{}, x)
	n, err := r.n.NotifyAlerts(context.Background())
	if err != nil || n != 0 || len(r.sender.Texts()) != 0 || len(r.sent.Marks()) != 0 {
		t.Fatalf("got %d, %v", n, err)
	}
}

func TestAlertNoDataNeverSent(t *testing.T) {
	t.Parallel()
	x, y := fired("a", "d"), fired("b", "d")
	x.State, y.State = "no_data", "clear"
	r := newAlertRig(t, noon(), notify.Config{}, x, y)
	n, err := r.n.NotifyAlerts(context.Background())
	if err != nil || n != 0 || len(r.sender.Texts()) != 0 {
		t.Fatalf("got %d, %v", n, err)
	}
}
