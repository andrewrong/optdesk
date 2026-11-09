package optionledger

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestPremiumIncomeReportedApart(t *testing.T) {
	t.Parallel()
	// 2 contracts opened at 1.50 and bought back at 0.40, commissions 2.60.
	r := replay(
		optRow("1", "-2", "300", "-1.30", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "2", "-80", "-1.30", "C", "20260110", "111", "50", "20260220", "P", ""),
	)
	p := r.Positions[0]
	if p.PremiumIncome != "220.0000" || p.Commissions != "2.6000" || p.RealizedPnL != "217.4000" || p.Status != "closed" {
		t.Fatalf("%+v", p)
	}
}

func TestExpiredPutRealizedPnl(t *testing.T) {
	t.Parallel()
	checkFixture(t, "expired_put")
}

func markInput(markDay string, sessions []string) compute.LedgerInput {
	in := compute.LedgerInput{
		Alias: "a", AsOf: "2026-01-20",
		Trades: []compute.LedgerTrade{optRow("1", "-1", "100", "-0.65", "O", "20260105", "111", "50", "20260220", "P", "")},
		Marks:  []compute.LedgerMark{{Contract: "111", Day: toDay(markDay), Bid: "0.40", Ask: "0.50"}},
	}
	for _, s := range sessions {
		in.SessionDays = append(in.SessionDays, toDay(s))
	}
	return in
}

func TestMarkMidOfBidAsk(t *testing.T) {
	t.Parallel()
	r := New().Replay(markInput("2026-01-20", []string{"2026-01-20"}), compute.LedgerParams{MarkMaxAgeSessions: 5})
	p := r.Positions[0]
	// 100 - 0.65 - 0.45 x 1 x 100 = 54.35
	if p.MarkStatus != "ok" || p.Mark != "0.4500" || p.MarkDay != "2026-01-20" || p.UnrealizedPnL != "54.3500" || p.RealizedPnL != "" {
		t.Fatalf("%+v", p)
	}
}

func TestMarkOlderThanFiveSessionsIsNoMark(t *testing.T) {
	t.Parallel()
	days := []string{"2026-01-12", "2026-01-13", "2026-01-14", "2026-01-15", "2026-01-16", "2026-01-20"}
	// mark on 01-12: five session days after it (13 14 15 16 20) is still used
	ok := New().Replay(markInput("2026-01-12", days), compute.LedgerParams{MarkMaxAgeSessions: 5}).Positions[0]
	if ok.MarkStatus != "ok" {
		t.Fatalf("age 5: %+v", ok)
	}
	old := New().Replay(markInput("2026-01-12", days), compute.LedgerParams{MarkMaxAgeSessions: 4}).Positions[0] // five sessions after the mark, limit four
	if old.MarkStatus != "no_mark" || old.Mark != "" || old.UnrealizedPnL != "" {
		t.Fatalf("too old: %+v", old)
	}
	none := New().Replay(compute.LedgerInput{Alias: "a", AsOf: "2026-01-20", Trades: markInput("", nil).Trades},
		compute.LedgerParams{MarkMaxAgeSessions: 5}).Positions[0]
	if none.MarkStatus != "no_mark" {
		t.Fatalf("no row: %+v", none)
	}
}

func toDay(s string) contracts.Day { return contracts.Day(s) }
