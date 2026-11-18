package household_page_test

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestUnmatchedPutIsNotedOnItsObligationRow(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.stored = []store.Position{shortPut("7974", "7250")}
	c := figuresConfig(moneyFigures{lines: []blockapi.MoneyLine{ok("7974 short put", "7250"), ok("7974 short put", "7000")}})
	c.Account = acc
	c.Ledgers = func() compute.OptionLedger { return twoPuts{} }
	m, b := build(t, c, "main")
	rows := m["obligations"].(map[string]any)["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	if n := rowNote(rows[0]); strings.Contains(n, "snapshot") {
		t.Fatalf("matched row note = %q", n)
	}
	if n := rowNote(rows[1]); !strings.Contains(n, "not in the position snapshot") {
		t.Fatalf("unmatched row note = %q", n)
	}
	validate(t, b)
}

type twoPuts struct{}

func (twoPuts) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	put := func(u, strike, exp string) compute.LedgerPosition {
		return compute.LedgerPosition{Underlying: u, Status: "open", Right: "P", Quantity: "-1", Strike: strike,
			Expiry: exp, Multiplier: "1", Currency: "JPY"}
	}
	return compute.LedgerResult{Positions: []compute.LedgerPosition{put("7974", "7250", "20261112"), put("7974", "7000", "20261112")}}
}

func TestStressStepsAreInTheNoteNotTheGap(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	m, b := build(t, config(account(), &got), "main")
	st := m["stress"].(map[string]any)
	rows := st["rows"].([]any)
	if len(rows) != 1 || st["gap"] != nil {
		t.Fatalf("stress = %v", st)
	}
	if n := rowNote(rows[0]); n != "steps at -10%, -20% and -30% need underlying prices: no data yet" {
		t.Fatalf("note = %q", n)
	}
	if !hasGapLine(m, "Stress test") || m["run_status"] != "partial" {
		t.Fatalf("gaps = %v", m["gaps"])
	}
	validate(t, b)
}
