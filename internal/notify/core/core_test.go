package core_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/notify/notifytest"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

var utc = time.UTC

// noon is outside the default quiet hours.
func noon() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, utc) }

type rig struct {
	src    *notifytest.Source
	sender *notifytest.Sender
	clock  *clockwork.FakeClock
	n      notify.Notifier
}

func newRig(t *testing.T, start time.Time, cfg notify.Config) *rig {
	t.Helper()
	if cfg.Zone == nil {
		cfg.Zone = utc
	}
	r := &rig{src: &notifytest.Source{}, sender: &notifytest.Sender{}, clock: clockwork.NewFakeClockAt(start)}
	n, err := core.New(r.src, r.sender, r.clock, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.n = n
	return r
}

func dsGap(id string, i int) store.Gap {
	return store.Gap{DatasetID: id, Day: contracts.Day("2026-11-06"), Reason: "expired",
		FirstSeenAt: time.Date(2026, 11, 7, 0, i, 0, 0, utc)}
}

func TestMarkOnlyAfterSend(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetGaps(dsGap("a", 0))
	var markedAtSend int
	r.sender.OnSend = func() { markedAtSend = len(r.src.Marked()) }
	n, err := r.n.NotifyGaps(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("got %d, %v", n, err)
	}
	if markedAtSend != 0 || len(r.src.Marked()) != 1 {
		t.Fatalf("marked at send %d, after %d", markedAtSend, len(r.src.Marked()))
	}
	if at := r.src.MarkTimes()[0]; !at.Equal(noon()) {
		t.Fatalf("mark time %v", at)
	}
}

func TestFailedSendMarksNothing(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetGaps(dsGap("a", 0))
	r.sender.SetErr(notify.ErrSend)
	n, err := r.n.NotifyGaps(context.Background())
	if !errors.Is(err, notify.ErrSend) || n != 0 {
		t.Fatalf("got %d, %v", n, err)
	}
	if len(r.src.Marked()) != 0 {
		t.Fatal("marked after failed send")
	}
}

func TestOneMessagePerBatch(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetGaps(dsGap("a", 0), dsGap("b", 1), dsGap("c", 2))
	n, err := r.n.NotifyGaps(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("got %d, %v", n, err)
	}
	if got := len(r.sender.Texts()); got != 1 {
		t.Fatalf("%d texts", got)
	}
	if first := strings.SplitN(r.sender.Texts()[0], "\n", 2)[0]; first != "optdesk: 3 job(s) missed work" {
		t.Fatalf("first line %q", first)
	}
}

func TestReportedGapNotRepeated(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetGaps(dsGap("a", 0))
	ctx := context.Background()
	if _, err := r.n.NotifyGaps(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := r.n.NotifyGaps(ctx)
	if err != nil || n != 0 || len(r.sender.Texts()) != 1 {
		t.Fatalf("second pass: %d, %v, %d texts", n, err, len(r.sender.Texts()))
	}
}

func TestClearedGapSkipped(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	g := dsGap("a", 0)
	g.ClearedAt = noon()
	r.src.SetGaps(g)
	n, err := r.n.NotifyGaps(context.Background())
	if err != nil || n != 0 || len(r.sender.Texts()) != 0 {
		t.Fatalf("got %d, %v, %d texts", n, err, len(r.sender.Texts()))
	}
}

func TestQuietHoursHoldGaps(t *testing.T) {
	t.Parallel()
	r := newRig(t, time.Date(2026, 11, 9, 5, 0, 0, 0, utc), notify.Config{})
	r.src.SetGaps(dsGap("a", 0))
	ctx := context.Background()
	n, err := r.n.NotifyGaps(ctx)
	if err != nil || n != 0 || len(r.sender.Texts()) != 0 || len(r.src.Marked()) != 0 {
		t.Fatalf("quiet pass: %d, %v", n, err)
	}
	r.clock.Advance(2 * time.Hour) // 07:00
	n, err = r.n.NotifyGaps(ctx)
	if err != nil || n != 1 || len(r.sender.Texts()) != 1 {
		t.Fatalf("after window: %d, %v", n, err)
	}
}

func TestQuietWindowCrossesMidnight(t *testing.T) {
	t.Parallel()
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata")
	}
	cases := []struct {
		at    time.Time
		quiet bool
	}{
		{time.Date(2026, 11, 9, 22, 59, 0, 0, ny), false},
		{time.Date(2026, 11, 9, 23, 0, 0, 0, ny), true},
		{time.Date(2026, 11, 10, 0, 30, 0, 0, ny), true},
		{time.Date(2026, 11, 10, 6, 59, 0, 0, ny), true},
		{time.Date(2026, 11, 10, 7, 0, 0, 0, ny), false},
		{time.Date(2026, 11, 10, 15, 0, 0, 0, ny).UTC(), false},
		{time.Date(2026, 11, 10, 3, 0, 0, 0, ny).UTC(), true}, // zone applies to a UTC clock
	}
	for _, c := range cases {
		r := newRig(t, c.at, notify.Config{Zone: ny})
		r.src.SetGaps(dsGap("a", 0))
		n, err := r.n.NotifyGaps(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if held := n == 0; held != c.quiet {
			t.Errorf("%v: held=%v want %v", c.at, held, c.quiet)
		}
	}
	// a custom same-day window
	r := newRig(t, noon(), notify.Config{QuietFrom: "09:00", QuietTo: "13:00"})
	r.src.SetGaps(dsGap("a", 0))
	if n, _ := r.n.NotifyGaps(context.Background()); n != 0 {
		t.Fatal("custom window not held")
	}
}

func TestTextHoldsNoSentinel(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	const s = "SENTINEL-SECRET"
	r.src.SetGaps(
		store.Gap{DatasetID: "ds1", Day: "2026-11-06", Reason: s},
		store.Gap{DatasetID: "ds2", Day: "2026-11-06", Reason: "expired", JobID: ""},
	)
	if _, err := r.n.NotifyGaps(context.Background()); err != nil {
		t.Fatal(err)
	}
	text := r.sender.Texts()[0]
	if strings.Contains(text, s) || !strings.Contains(text, "dataset ds1 day 2026-11-06 unknown") {
		t.Fatalf("text %q", text)
	}
	// the error text of a failed send is not copied into the text either
	r2 := newRig(t, noon(), notify.Config{})
	r2.src.SetGaps(store.Gap{JobID: "job-" + s, Day: "2026-11-06", Reason: "preflight_failed"})
	if _, err := r2.n.NotifyGaps(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := r2.sender.Texts()[0]; !strings.Contains(got, "job job-"+s+" day 2026-11-06 preflight_failed") {
		t.Fatalf("job line %q", got)
	}
}

func TestTextCapsLines(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	var gaps []store.Gap
	for i := 0; i < 25; i++ {
		gaps = append(gaps, dsGap(fmt.Sprintf("d%02d", i), i))
	}
	r.src.SetGaps(gaps...)
	text := func() string { return r.sender.Texts()[len(r.sender.Texts())-1] }
	if _, err := r.n.NotifyGaps(context.Background()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(text(), "\n")
	if len(lines) != 22 || lines[0] != "optdesk: 25 job(s) missed work" || lines[21] != "and 5 more" {
		t.Fatalf("%d lines: %q", len(lines), lines[len(lines)-1])
	}
	// long ids: the byte cap prints fewer lines
	r2 := newRig(t, noon(), notify.Config{})
	long := strings.Repeat("x", 400)
	var big []store.Gap
	for i := 0; i < 20; i++ {
		big = append(big, dsGap(fmt.Sprintf("%s%02d", long, i), i))
	}
	r2.src.SetGaps(big...)
	n, err := r2.n.NotifyGaps(context.Background())
	if err != nil || n >= 20 || n == 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := r2.sender.Texts()[0]; len(got) > 3500 || !strings.Contains(got, fmt.Sprintf("and %d more", 20-n)) {
		t.Fatalf("%d bytes", len(got))
	}
}

func TestCapMarksOnlyPrinted(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	var gaps []store.Gap
	for i := 0; i < 23; i++ {
		gaps = append(gaps, dsGap(fmt.Sprintf("d%02d", i), 22-i)) // the last one is the oldest
	}
	r.src.SetGaps(gaps...)
	ctx := context.Background()
	n, err := r.n.NotifyGaps(ctx)
	if err != nil || n != 20 || len(r.src.Marked()) != 20 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	for _, g := range r.src.Marked() {
		if g.DatasetID == "d00" || g.DatasetID == "d01" || g.DatasetID == "d02" {
			t.Fatalf("marked unprinted %s", g.DatasetID)
		}
	}
	n, err = r.n.NotifyGaps(ctx)
	if err != nil || n != 3 || len(r.sender.Texts()) != 2 {
		t.Fatalf("second pass n=%d err=%v", n, err)
	}
}

func TestBackoffDoubles(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetGaps(dsGap("a", 0))
	r.sender.SetErr(notify.ErrSend)
	ctx := context.Background()
	attempts := func() int { return len(r.sender.Texts()) }
	pass := func() { _, _ = r.n.NotifyGaps(ctx) }
	pass() // fails, wait 15
	r.clock.Advance(14 * time.Minute)
	pass()
	if attempts() != 1 {
		t.Fatalf("attempt inside 15 min: %d", attempts())
	}
	r.clock.Advance(time.Minute)
	pass() // fails, wait 30
	r.clock.Advance(29 * time.Minute)
	pass()
	if attempts() != 2 {
		t.Fatalf("attempt inside 30 min: %d", attempts())
	}
	r.clock.Advance(time.Minute)
	pass() // fails, wait 60
	r.clock.Advance(59 * time.Minute)
	pass()
	if attempts() != 3 {
		t.Fatalf("attempt inside 60 min: %d", attempts())
	}
	r.clock.Advance(time.Minute)
	pass() // fails, wait stays 60
	r.clock.Advance(59 * time.Minute)
	pass()
	if attempts() != 4 {
		t.Fatalf("wait grew past 60: %d", attempts())
	}
	r.clock.Advance(time.Minute)
	r.sender.SetErr(nil)
	n, err := r.n.NotifyGaps(ctx)
	if err != nil || n != 1 {
		t.Fatalf("recovery: %d, %v", n, err)
	}
	// success resets the backoff
	r.src.SetGaps(dsGap("b", 1))
	r.sender.SetErr(notify.ErrSend)
	pass()
	r.clock.Advance(15 * time.Minute)
	pass()
	if attempts() != 7 {
		t.Fatalf("backoff not reset: %d", attempts())
	}
}

func TestMarkFailureRetried(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetGaps(dsGap("a", 0))
	r.src.SetMarkErr(errors.New("db down"))
	ctx := context.Background()
	if _, err := r.n.NotifyGaps(ctx); err == nil {
		t.Fatal("mark failure not returned")
	}
	r.src.SetMarkErr(nil)
	n, err := r.n.NotifyGaps(ctx) // no backoff after a confirmed send
	if err != nil || n != 1 || len(r.sender.Texts()) != 2 || len(r.src.Marked()) != 1 {
		t.Fatalf("retry: n=%d err=%v texts=%d", n, err, len(r.sender.Texts()))
	}
}

func TestBadConfigRefused(t *testing.T) {
	t.Parallel()
	src, snd, clk := &notifytest.Source{}, &notifytest.Sender{}, clockwork.NewFakeClock()
	for _, cfg := range []notify.Config{
		{Zone: utc, QuietFrom: "25:00"},
		{Zone: utc, QuietTo: "7am"},
		{QuietFrom: "23:00", QuietTo: "07:00"},
	} {
		if _, err := core.New(src, snd, clk, cfg); err == nil {
			t.Errorf("config %+v accepted", cfg)
		}
	}
	if _, err := core.New(src, snd, clk, notify.Config{Zone: utc}); err != nil {
		t.Fatalf("defaults refused: %v", err)
	}
}

func TestKeyGapLineCarriesDataset(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetGaps(
		store.Gap{DatasetID: "option_eod", Day: "2026-11-06", Reason: "crashed_repeatedly"},
		store.Gap{JobID: "daily_close", Day: "2026-11-06", Reason: "preflight_failed"},
	)
	if _, err := r.n.NotifyGaps(context.Background()); err != nil {
		t.Fatal(err)
	}
	text := r.sender.Texts()[0]
	for _, want := range []string{"dataset option_eod day 2026-11-06 crashed_repeatedly", "job daily_close day 2026-11-06 preflight_failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
}

func TestOpenGapsErrorReturned(t *testing.T) {
	t.Parallel()
	r := newRig(t, noon(), notify.Config{})
	r.src.SetOpenErr(errors.New("db down"))
	if n, err := r.n.NotifyGaps(context.Background()); err == nil || n != 0 || len(r.sender.Texts()) != 0 {
		t.Fatalf("got %d, %v", n, err)
	}
}
