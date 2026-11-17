package option_pnl_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// The four rows of Decision 5: d1 passes the ledger rule; d2 is later with an underlying price 0; d3 is later
// with ask 0; d4 is after the expiry. The mark is d1.
func decisionFiveRows() []optionday.Day {
	return []optionday.Day{
		dayRow(101, "2026-12-10", "1.20", "1.30", "95"),
		dayRow(101, "2026-12-11", "1.20", "1.30", "0"),
		dayRow(101, "2026-12-14", "1.20", "0", "95"),
		dayRow(101, "2026-12-21", "1.20", "1.30", "95"),
	}
}

func TestMarkIsLatestRulePassingRow(t *testing.T) {
	t.Parallel()
	p := lotPos("p1", "AAA", "101", "90.00", "open")
	m, ok := markOf(p, decisionFiveRows())
	if !ok || m.Day != "2026-12-10" || m.Contract != "101" || m.Bid != "1.20" || m.Ask != "1.30" || m.UnderlyingClose != "95" {
		t.Fatalf("mark = %+v, %v", m, ok)
	}
	if _, ok := markOf(p, decisionFiveRows()[1:3]); ok {
		t.Fatal("no row passes, want no mark")
	}
	bid0 := []optionday.Day{dayRow(101, "2026-12-10", "0", "0.05", "95")}
	if _, ok := markOf(p, bid0); !ok {
		t.Fatal("bid 0 passes the ledger rule")
	}
}

func TestOpenLotGetsMarkFromMid(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-16", "1.10", "1.20", "94"), dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.results["90.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "100"}
	r.build(t)
	ms := r.replay.ins[1].Marks
	if len(ms) != 1 || ms[0] != (compute.LedgerMark{Contract: "101", Day: "2026-11-17", Bid: "1.20", Ask: "1.30", UnderlyingClose: "95"}) {
		t.Fatalf("marks = %+v", ms)
	}
	in := r.attr.ins[0]
	if in.LedgerMarkDay != "2026-11-17" || in.LedgerPnL != "100" || in.Lot.End != "open" {
		t.Fatalf("input = %+v", in)
	}
}

func TestCompositeContractGetsNoMark(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "AAA 20261218P90", "90.00", "open")}
	card := r.build(t).Accounts[0].Lots[0]
	if len(r.replay.ins[1].Marks) != 0 || card.Gap == nil {
		t.Fatalf("marks = %v, gap = %v", r.replay.ins[1].Marks, card.Gap)
	}
}
