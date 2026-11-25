package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// gatewayMarksFrom is the first day of the mark read: an incident is marked on the day of its Since,
// which may be old or zero, and its mark must be found for the dedupe and for the recovery text.
// alerts.sent holds a few rows a day, so the whole read is small.
const gatewayMarksFrom contracts.Day = "0001-01-01"

const gatewayUpSuffix = "_up"

type gatewayNotifier struct {
	sent   store.SentStore
	sender notify.Sender
	clock  clockwork.Clock
	mu     sync.Mutex
}

// NewGateways is notify.NewGatewayNotifierFunc.
func NewGateways(sender notify.Sender, sent store.SentStore, clock clockwork.Clock) (notify.GatewayNotifier, error) {
	if sender == nil || sent == nil || clock == nil {
		return nil, errors.New("notify: sender, sent store and clock are required")
	}
	return &gatewayNotifier{sent: sent, sender: sender, clock: clock}, nil
}

var _ notify.NewGatewayNotifierFunc = NewGateways

func gatewayKey(f notify.GatewayFact) string {
	return fmt.Sprintf("gateway_%s_%s_%d", f.Gateway, f.Farm, f.Since.Unix())
}

func gatewayDay(since time.Time) contracts.Day {
	return contracts.Day(since.UTC().Format(time.DateOnly))
}

// NotifyGateways makes one pass; see notify.GatewayNotifier.
func (n *gatewayNotifier) NotifyGateways(ctx context.Context, facts []notify.GatewayFact) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	marks, err := n.sent.SentSince(ctx, gatewayMarksFrom)
	if err != nil {
		return 0, fmt.Errorf("notify: read sent marks: %w", err)
	}
	said := make(map[string]bool, len(marks))
	for _, m := range marks {
		said[m.RuleID] = true
	}
	sentCount := 0
	var firstErr error
	for _, f := range facts {
		var err error
		var sent bool
		if f.Down {
			sent, err = n.incident(ctx, f, said)
		} else {
			sent, err = n.recovery(ctx, f.Gateway, marks, said)
		}
		if sent {
			sentCount++
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return sentCount, firstErr
}

func (n *gatewayNotifier) incident(ctx context.Context, f notify.GatewayFact, said map[string]bool) (bool, error) {
	key := gatewayKey(f)
	if said[key] {
		return false, nil
	}
	if err := n.sender.Send(ctx, fmt.Sprintf(gatewayDownText, f.Gateway, f.Farm)); err != nil {
		return false, err
	}
	said[key] = true
	if err := n.sent.MarkSent(ctx, key, gatewayDay(f.Since)); err != nil {
		return true, fmt.Errorf("notify: mark gateway incident sent: %w", err)
	}
	return true, nil
}

// recovery sends one text when the gateway has marked incidents without a recovery mark, and marks them all.
func (n *gatewayNotifier) recovery(ctx context.Context, gateway string, marks []store.SentMark, said map[string]bool) (bool, error) {
	prefix := "gateway_" + gateway + "_"
	var open []store.SentMark
	for _, m := range marks {
		if !strings.HasPrefix(m.RuleID, prefix) || strings.HasSuffix(m.RuleID, gatewayUpSuffix) || said[m.RuleID+gatewayUpSuffix] {
			continue
		}
		rest := m.RuleID[len(prefix):]
		_, num, found := strings.CutLast(rest, "_")
		if !found {
			continue
		}
		if _, err := strconv.ParseInt(num, 10, 64); err != nil {
			continue
		}
		open = append(open, m)
	}
	if len(open) == 0 {
		return false, nil
	}
	if err := n.sender.Send(ctx, fmt.Sprintf(gatewayUpText, gateway)); err != nil {
		return false, err
	}
	for _, m := range open {
		said[m.RuleID+gatewayUpSuffix] = true
		if err := n.sent.MarkSent(ctx, m.RuleID+gatewayUpSuffix, m.Day); err != nil {
			return true, fmt.Errorf("notify: mark gateway recovery sent: %w", err)
		}
	}
	return true, nil
}
