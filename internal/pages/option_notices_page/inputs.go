package option_notices_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const kindHeldCallGuard = "held_call_guard"

func noticePositions(rows []store.Position) []compute.NoticePosition {
	out := make([]compute.NoticePosition, 0, len(rows))
	for _, p := range rows {
		out = append(out, compute.NoticePosition{Symbol: p.Symbol, SecType: p.SecType, Expiry: p.Expiry,
			Strike: p.Strike, Right: p.Right, Multiplier: p.Multiplier, Quantity: p.Quantity, Currency: p.Currency})
	}
	return out
}

func noticeValues(rows []store.AccountValue) []compute.NoticeValue {
	out := make([]compute.NoticeValue, 0, len(rows))
	for _, v := range rows {
		out = append(out, compute.NoticeValue{Tag: v.Tag, Value: v.Value, Currency: v.Currency})
	}
	return out
}

type gateResult struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// blockedReasons are the codes of the blocked gates of the stored JSON array text; a text that
// does not parse gives none.
func blockedReasons(text string) []string {
	var gates []gateResult
	if err := json.Unmarshal([]byte(text), &gates); err != nil {
		return []string{}
	}
	out := []string{}
	for _, g := range gates {
		if g.Status == "block" && g.Reason != "" {
			out = append(out, g.Reason)
		}
	}
	return out
}

// guardInputs reads the held_call_guard rows of day for alias (the only kind read). The second
// result is false when there are none.
func guardInputs(ctx context.Context, r store.CandidateReader, alias string, day contracts.Day) ([]compute.NoticeGuard, bool, error) {
	rows, err := r.Candidates(ctx, day, kindHeldCallGuard)
	if err != nil {
		return nil, false, fmt.Errorf("option_notices_page: read guard rows: %w", err)
	}
	out := []compute.NoticeGuard{}
	for _, c := range rows {
		if c.Alias != alias {
			continue
		}
		out = append(out, compute.NoticeGuard{Symbol: c.Issuer, Expiry: string(c.Expiry), Strike: c.Strike,
			Status: c.Status, Reasons: blockedReasons(c.Gates)})
	}
	return out, len(out) > 0, nil
}

// earningsInputs reads, for each distinct symbol of rows, the newest listed report date seen in the
// lookback window ending on day. A symbol with none is left out.
func earningsInputs(ctx context.Context, r store.EarningsDateReader, rows []store.Position, day contracts.Day) ([]compute.NoticeEarnings, error) {
	from, err := daysBack(day, lookbackDays)
	if err != nil {
		return nil, fmt.Errorf("option_notices_page: bad day %q: %w", day, err)
	}
	out := []compute.NoticeEarnings{}
	seen := map[string]bool{}
	for _, p := range rows {
		if p.Symbol == "" || !strings.EqualFold(p.SecType, "OPT") || seen[p.Symbol] {
			continue
		}
		seen[p.Symbol] = true
		dates, err := r.EarningsDates(ctx, p.Symbol, from, day)
		if err != nil {
			return nil, fmt.Errorf("option_notices_page: read earnings of %s: %w", p.Symbol, err)
		}
		var next *contracts.Day
		for _, e := range dates {
			if e.Status == "listed" && e.ReportDate != nil {
				d := contracts.Day(e.ReportDate.Format(dayLayout))
				next = &d
			}
		}
		if next != nil {
			out = append(out, compute.NoticeEarnings{Symbol: p.Symbol, Next: next})
		}
	}
	return out, nil
}
