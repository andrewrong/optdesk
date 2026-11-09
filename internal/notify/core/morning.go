package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/jonboulle/clockwork"
)

// expectLookbackDays is the span of calendar days before today whose expectations are read.
const expectLookbackDays = 7

type morningNotifier struct {
	deps       notify.MorningDeps
	sender     notify.Sender
	clock      clockwork.Clock
	zone       *time.Location
	from, till int // minutes after midnight
	mu         sync.Mutex
}

// NewMorningNotifier is notify.NewMorningNotifierFunc. It fails only on a bad Config or an incomplete MorningDeps.
func NewMorningNotifier(deps notify.MorningDeps, sender notify.Sender, clock clockwork.Clock, cfg notify.Config) (notify.MorningNotifier, error) {
	if sender == nil || clock == nil {
		return nil, errors.New("notify: sender and clock are required")
	}
	if deps.History == nil || deps.Expect == nil || deps.Sent == nil {
		return nil, errors.New("notify: MorningDeps.History, Expect and Sent are required")
	}
	if cfg.Zone == nil {
		return nil, errors.New("notify: Config.Zone is required")
	}
	from, till := cfg.MorningFrom, cfg.MorningUntil
	if from == "" {
		from = notify.DefaultMorningFrom
	}
	if till == "" {
		till = notify.DefaultMorningUntil
	}
	w, err := parseWindow(from, till)
	if err != nil {
		return nil, err
	}
	return &morningNotifier{deps: deps, sender: sender, clock: clock, zone: cfg.Zone, from: w.from, till: w.to}, nil
}

var _ notify.NewMorningNotifierFunc = NewMorningNotifier

// NotifyMorning makes one pass; see notify.MorningNotifier.
func (n *morningNotifier) NotifyMorning(ctx context.Context) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.clock.Now().In(n.zone)
	if m := now.Hour()*60 + now.Minute(); m < n.from || m >= n.till {
		return false, nil
	}
	day := contracts.Day(now.Format(time.DateOnly))
	marks, err := n.deps.Sent.SentSince(ctx, day)
	if err != nil {
		return false, fmt.Errorf("notify: read sent marks: %w", err)
	}
	for _, m := range marks {
		if m.RuleID == notify.MorningRuleID && m.Day == day {
			return false, nil
		}
	}
	in, err := n.gather(ctx, now)
	if err != nil {
		return false, err
	}
	if err := n.sender.Send(ctx, morningText(in)); err != nil {
		return false, err
	}
	if err := n.deps.Sent.MarkSent(ctx, notify.MorningRuleID, day); err != nil {
		return true, fmt.Errorf("notify: mark morning report sent: %w", err)
	}
	return true, nil
}

// gather reads the three sources and builds the input of the text.
func (n *morningNotifier) gather(ctx context.Context, now time.Time) (morningInput, error) {
	since := time.Date(now.Year(), now.Month(), now.Day()-1, n.from/60, n.from%60, 0, 0, n.zone)
	hist, err := n.deps.History.GapsSince(ctx, since)
	if err != nil {
		return morningInput{}, fmt.Errorf("notify: read gap history: %w", err)
	}
	open, err := n.deps.Expect.OpenGaps(ctx)
	if err != nil {
		return morningInput{}, fmt.Errorf("notify: read open gaps: %w", err)
	}
	// The previous session day is the newest ExpectedDay among the rows; today is not read.
	yesterday := contracts.Day(now.AddDate(0, 0, -1).Format(time.DateOnly))
	last := contracts.Day(now.AddDate(0, 0, -expectLookbackDays).Format(time.DateOnly))
	exps, err := n.deps.Expect.Expectations(ctx, last, yesterday)
	if err != nil {
		return morningInput{}, fmt.Errorf("notify: read expectations: %w", err)
	}
	day := contracts.Day(now.Format(time.DateOnly))
	in := morningInput{day: string(day), zone: n.zone, open: len(open)}
	for _, e := range exps {
		if e.ExpectedDay > in.sessionDay {
			in.sessionDay = e.ExpectedDay
		}
	}
	for _, e := range exps {
		if e.ExpectedDay == in.sessionDay && e.Gap != "" {
			in.late++
		}
	}
	for _, g := range hist {
		if g.ClearedAt.IsZero() || g.ClearedAt.Before(since) {
			continue
		}
		if g.ReportedAt.IsZero() {
			in.selfHealed = append(in.selfHealed, g)
		} else {
			in.clearedSent++
		}
	}
	sort.SliceStable(in.selfHealed, func(i, j int) bool { return in.selfHealed[i].ClearedAt.Before(in.selfHealed[j].ClearedAt) })
	return in, nil
}
