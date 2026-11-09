package optionledger

import (
	"testing"
)

func TestAssignClosesOnlyTheOption(t *testing.T) {
	t.Parallel()
	r := replay(
		optRow("1", "-1", "100", "-0.65", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "1", "0", "0", "C", "20260220", "111", "50", "20260220", "P", "A"),
	)
	if len(r.Positions) != 1 || r.Positions[0].Status != "assigned" {
		t.Fatalf("positions %+v", r.Positions)
	}
	if len(r.Cycles) != 0 {
		t.Fatalf("an assignment row alone made shares: %+v", r.Cycles)
	}
}

func TestAssignmentStockRowNotCountedTwice(t *testing.T) {
	t.Parallel()
	r := replay(
		optRow("1", "-2", "300", "-1.30", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "2", "0", "0", "C", "20260220", "111", "50", "20260220", "P", "A"),
		stkRow("3", "200", "50", "-10000", "-1", "20260220"),
	)
	if len(r.Cycles) != 1 || r.Cycles[0].Shares != "200" {
		t.Fatalf("cycles %+v", r.Cycles)
	}
}

func TestEarlyAssignmentMarked(t *testing.T) {
	t.Parallel()
	got := checkFixture(t, "early_assignment")
	if !got.Positions[0].Early {
		t.Fatal("not early")
	}
	on := replay(
		optRow("1", "-1", "100", "-0.65", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "1", "0", "0", "C", "20260220", "111", "50", "20260220", "P", "A"),
	)
	if on.Positions[0].Early {
		t.Fatal("assignment on the expiry date marked early")
	}
}

func TestCloseWithoutOpenUnresolved(t *testing.T) {
	t.Parallel()
	checkFixture(t, "close_without_open")
}

func TestFifoSplitClose(t *testing.T) {
	t.Parallel()
	r := replay(
		optRow("10", "-1", "100", "-1", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("11", "-2", "300", "-2", "O", "20260106", "111", "50", "20260220", "P", ""),
		optRow("12", "2", "-100", "-1", "C", "20260110", "111", "50", "20260220", "P", ""),
	)
	want := []struct{ id, status, qty, premium, comm string }{
		{"a:10:1", "closed", "-1", "50.0000", "1.5000"},
		{"a:11:1", "closed", "-1", "100.0000", "1.5000"},
		{"a:11:2", "open", "-1", "150.0000", "1.0000"},
	}
	if len(r.Positions) != len(want) {
		t.Fatalf("positions %+v", r.Positions)
	}
	for i, w := range want {
		p := r.Positions[i]
		if p.PositionID != w.id || p.Status != w.status || p.Quantity != w.qty || p.PremiumIncome != w.premium || p.Commissions != w.comm {
			t.Errorf("position %d: %+v, want %+v", i, p, w)
		}
	}
	if len(r.Unresolved) != 0 {
		t.Errorf("unresolved %v", r.Unresolved)
	}
}

func TestCorrectionRowGoesToReview(t *testing.T) {
	t.Parallel()
	cancel := optRow("2", "1", "-80", "-1", "C", "20260110", "111", "50", "20260220", "P", "Ca")
	cancel.Attributes["transactionType"] = "TradeCancel"
	r := replay(
		optRow("1", "-1", "100", "-1", "O", "20260105", "111", "50", "20260220", "P", ""),
		cancel,
	)
	if len(r.Review) != 1 || r.Review[0] != "2" {
		t.Fatalf("review %v", r.Review)
	}
	if len(r.Positions) != 1 || r.Positions[0].Status != "open" || r.Positions[0].Commissions != "1.0000" {
		t.Fatalf("the correction row entered a figure: %+v", r.Positions)
	}
	if len(r.Unresolved) != 0 {
		t.Fatalf("unresolved %v", r.Unresolved)
	}
}
