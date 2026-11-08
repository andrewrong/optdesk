package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

var backoffSteps = []time.Duration{15 * time.Minute, 30 * time.Minute, 60 * time.Minute}

type notifier struct {
	src    notify.GapSource
	sender notify.Sender
	clock  clockwork.Clock
	zone   *time.Location
	win    window

	mu       sync.Mutex
	failures int       // failed passes since the last success
	holdTill time.Time // no send attempt before this time
}

// New is notify.NewNotifierFunc. It fails only on a bad Config (a unparsable time or no zone).
func New(src notify.GapSource, sender notify.Sender, clock clockwork.Clock, cfg notify.Config) (notify.Notifier, error) {
	if src == nil || sender == nil || clock == nil {
		return nil, errors.New("notify: source, sender and clock are required")
	}
	if cfg.Zone == nil {
		return nil, errors.New("notify: Config.Zone is required")
	}
	from, to := cfg.QuietFrom, cfg.QuietTo
	if from == "" {
		from = notify.DefaultQuietFrom
	}
	if to == "" {
		to = notify.DefaultQuietTo
	}
	w, err := parseWindow(from, to)
	if err != nil {
		return nil, err
	}
	return &notifier{src: src, sender: sender, clock: clock, zone: cfg.Zone, win: w}, nil
}

var _ notify.NewNotifierFunc = New

// NotifyGaps makes one pass; see notify.Notifier.
func (n *notifier) NotifyGaps(ctx context.Context) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.clock.Now()
	if now.Before(n.holdTill) {
		return 0, nil
	}
	all, err := n.src.OpenGaps(ctx)
	if err != nil {
		return 0, fmt.Errorf("notify: read gaps: %w", err)
	}
	var fresh []store.Gap
	for _, g := range all {
		if !g.ClearedAt.IsZero() || !g.ReportedAt.IsZero() {
			continue
		}
		fresh = append(fresh, g)
	}
	if len(fresh) == 0 || quiet(now, n.zone, n.win) {
		return 0, nil
	}
	text, printed := buildText(fresh)
	if err := n.sender.Send(ctx, text); err != nil {
		step := backoffSteps[min(n.failures, len(backoffSteps)-1)]
		n.failures++
		n.holdTill = now.Add(step)
		return 0, err
	}
	n.failures = 0
	n.holdTill = time.Time{}
	for i, g := range printed {
		if err := n.src.MarkGapReported(ctx, g, now); err != nil {
			return i, fmt.Errorf("notify: mark gap reported: %w", err)
		}
	}
	return len(printed), nil
}
