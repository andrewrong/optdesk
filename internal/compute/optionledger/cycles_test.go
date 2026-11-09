package optionledger

import "testing"

func TestWheelOneCycle(t *testing.T) {
	t.Parallel()
	checkFixture(t, "wheel_one_cycle")
}

func TestBasisIsStrikeNotNetOfPremium(t *testing.T) {
	t.Parallel()
	r := replay(
		optRow("1", "-2", "300", "-1.30", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "2", "0", "0", "C", "20260220", "111", "50", "20260220", "P", "A"),
		stkRow("3", "200", "50", "-10000", "-1", "20260220"),
		optRow("4", "-2", "160", "-1.30", "O", "20260225", "222", "55", "20260320", "C", ""),
	)
	c := r.Cycles[0]
	if c.CostBasisPerShare != "50.0000" || c.PremiumIncome != "460.0000" || c.Shares != "200" {
		t.Fatalf("%+v", c)
	}
	if c.StockRealizedPnL != "0.0000" || c.StockCommissions != "1.0000" {
		t.Fatalf("%+v", c)
	}
}

func TestTwoCurrenciesNeverSummed(t *testing.T) {
	t.Parallel()
	r := checkFixture(t, "two_currencies")
	if r.Positions[0].Currency == r.Positions[1].Currency {
		t.Fatal("currencies not kept apart")
	}
}
