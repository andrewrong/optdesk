package household_page_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
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
	if n := rowNote(rows[1]); !strings.Contains(n, household_page.NoteNotInSnapshot) {
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
	if n := rowNote(rows[0]); n != household_page.NoteStressSteps {
		t.Fatalf("note = %q", n)
	}
	if !hasGapLine(m, "Stress test") || m["run_status"] != "partial" {
		t.Fatalf("gaps = %v", m["gaps"])
	}
	validate(t, b)
}

// lots replays open short puts of one underlying, strike and expiry, one position per quantity.
type lots []string

func (l lots) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	var ps []compute.LedgerPosition
	for _, q := range l {
		ps = append(ps, compute.LedgerPosition{Underlying: "7974", Status: "open", Right: "P", Quantity: q,
			Strike: "7250", Expiry: "20261112", Multiplier: "1", Currency: "JPY"})
	}
	return compute.LedgerResult{Positions: ps}
}

func storedPut(qty string) store.Position {
	p := shortPut("7974", "7250")
	p.Quantity = qty
	return p
}

func buildLots(t *testing.T, ledger lots, stored ...store.Position) (map[string]any, []any) {
	t.Helper()
	acc := account()
	acc.stored = stored
	var ls []blockapi.MoneyLine
	for range ledger {
		ls = append(ls, ok("7974 short put", "7250"))
	}
	c := figuresConfig(moneyFigures{lines: ls})
	c.Account = acc
	c.Ledgers = func() compute.OptionLedger { return ledger }
	m, _ := build(t, c, "main")
	return m, m["obligations"].(map[string]any)["rows"].([]any)
}

func TestTwoLotsAgainstOneStoredRowAreMatched(t *testing.T) {
	t.Parallel()
	m, rows := buildLots(t, lots{"-1", "-1"}, storedPut("-2"))
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	for _, r := range rows {
		if n := rowNote(r); strings.Contains(n, "snapshot") {
			t.Fatalf("row note = %q", n)
		}
	}
	if hasGapLine(m, "7974 P7250") {
		t.Fatalf("gaps = %v", m["gaps"])
	}
}

func TestOneLotAgainstTwoStoredRowsIsMatched(t *testing.T) {
	t.Parallel()
	m, rows := buildLots(t, lots{"-2"}, storedPut("-1"), storedPut("-1"))
	if len(rows) != 1 || strings.Contains(rowNote(rows[0]), "snapshot") {
		t.Fatalf("rows = %v", rows)
	}
	if hasGapLine(m, "7974 P7250") {
		t.Fatalf("gaps = %v", m["gaps"])
	}
	b, _ := json.Marshal(m)
	if strings.Contains(string(b), "不在期权账本中") {
		t.Fatalf("page = %s", b)
	}
}

func TestUnreadableQuantityCountsAsOneContract(t *testing.T) {
	t.Parallel()
	// two unreadable ledger lots count 1 each, so one stored row of -2 covers both
	_, rows := buildLots(t, lots{"-x", "-x"}, storedPut("-2"))
	for _, r := range rows {
		if n := rowNote(r); strings.Contains(n, "snapshot") {
			t.Fatalf("row note = %q", n)
		}
	}
	// an unreadable stored row counts 1, so it covers only one of two lots of 1
	_, rows = buildLots(t, lots{"-1", "-1"}, storedPut("-x"))
	if len(rows) != 2 || strings.Contains(rowNote(rows[0]), "snapshot") || !strings.Contains(rowNote(rows[1]), household_page.NoteNotInSnapshot) {
		t.Fatalf("rows = %v", rows)
	}
}
