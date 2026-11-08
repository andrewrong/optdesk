package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// alertLookbackDays is the span of days Evaluate reads, so the span of marks to load.
const alertLookbackDays = 5

type alertNotifier struct {
	deps   notify.AlertDeps
	sender notify.Sender
	clock  clockwork.Clock
	zone   *time.Location
	win    window

	mu       sync.Mutex
	failures int
	holdTill time.Time
}

// NewAlerts is notify.NewAlertNotifierFunc. It fails only on a bad Config or an incomplete AlertDeps.
func NewAlerts(deps notify.AlertDeps, sender notify.Sender, clock clockwork.Clock, cfg notify.Config) (notify.AlertNotifier, error) {
	if sender == nil || clock == nil {
		return nil, errors.New("notify: sender and clock are required")
	}
	if deps.Evaluate == nil || deps.Read == nil || deps.Sent == nil {
		return nil, errors.New("notify: AlertDeps.Evaluate, Read and Sent are required")
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
	return &alertNotifier{deps: deps, sender: sender, clock: clock, zone: cfg.Zone, win: w}, nil
}

var _ notify.NewAlertNotifierFunc = NewAlerts

// NotifyAlerts makes one pass; see notify.AlertNotifier.
func (n *alertNotifier) NotifyAlerts(ctx context.Context) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.clock.Now()
	if now.Before(n.holdTill) || quiet(now, n.zone, n.win) {
		return 0, nil
	}
	today := now.In(n.zone)
	results, err := n.deps.Evaluate(ctx, n.deps.Rules, n.deps.Read, contracts.Day(today.Format(time.DateOnly)))
	if err != nil {
		return 0, fmt.Errorf("notify: evaluate alerts: %w", err)
	}
	from := contracts.Day(today.AddDate(0, 0, -alertLookbackDays).Format(time.DateOnly))
	marks, err := n.deps.Sent.SentSince(ctx, from)
	if err != nil {
		return 0, fmt.Errorf("notify: read sent marks: %w", err)
	}
	said := make(map[store.SentMark]bool, len(marks))
	for _, m := range marks {
		said[m] = true
	}
	var fresh []compute.AlertResult
	for _, r := range results {
		if r.State != "fired" || !r.Public || said[store.SentMark{RuleID: r.RuleID, Day: r.Day}] {
			continue
		}
		fresh = append(fresh, r)
	}
	if len(fresh) == 0 {
		return 0, nil
	}
	text, printed := buildAlertText(fresh)
	if err := n.sender.Send(ctx, text); err != nil {
		step := backoffSteps[min(n.failures, len(backoffSteps)-1)]
		n.failures++
		n.holdTill = now.Add(step)
		return 0, err
	}
	n.failures = 0
	n.holdTill = time.Time{}
	for i, r := range printed {
		if err := n.deps.Sent.MarkSent(ctx, r.RuleID, r.Day); err != nil {
			return i, fmt.Errorf("notify: mark alert sent: %w", err)
		}
	}
	return len(printed), nil
}
