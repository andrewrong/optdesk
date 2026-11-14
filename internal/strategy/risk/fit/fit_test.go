package fit_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/risk/fit"
)

// fakeStresser returns a fixed outcome and records the books it was given.
type fakeStresser struct {
	out   strategy.RiskOutcome
	books []strategy.RiskBook
}

func (f *fakeStresser) Stress(b strategy.RiskBook) strategy.RiskOutcome {
	f.books = append(f.books, b)
	return f.out
}

func leg(und, right, strike, qty, mark, spot string) strategy.RiskHolding {
	return strategy.RiskHolding{
		Underlying: und, Currency: "USD", Quantity: qty, Multiplier: "100", Spot: spot, Mark: mark,
		Option: &strategy.RiskLeg{Right: right, Strike: strike, Expiry: "2026-12-18"},
	}
}

func funds(avail string) *strategy.RiskFunds {
	return &strategy.RiskFunds{Currency: "USD", Available: avail}
}

func run(t *testing.T, book strategy.RiskBook, out strategy.RiskOutcome, c strategy.RiskCandidate) (strategy.RiskFit, *fakeStresser) {
	t.Helper()
	st := &fakeStresser{out: out}
	return fit.New(st).Fit(book, out, c), st
}

func total(v string) strategy.RiskOutcome {
	return strategy.RiskOutcome{Totals: []strategy.RiskTotal{{Currency: "USD", WorstLoss: v}}}
}

func TestFitNewCashSecuredPutCapitalIsStrikeTimesMultiplier(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "P", "100", "-2", "3", "105")}}
	got, _ := run(t, strategy.RiskBook{Funds: funds("20000")}, strategy.RiskOutcome{}, c)
	// 100 x 100 x 2
	if got.Facts["capital"] != "20000.00" || got.Check.Status != "pass" || got.Check.Name != "fit_capital" {
		t.Fatalf("got %+v", got)
	}
}

func TestFitNewIndexShortOptionUsesFifteenPercentLessOtm(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "broad_high", Legs: []strategy.RiskHolding{leg("SPX", "P", "4900", "-1", "20", "5000")}}
	got, _ := run(t, strategy.RiskBook{Funds: funds("100000")}, strategy.RiskOutcome{}, c)
	// (15% x 5000 - 100 otm) = 650 per unit, above the 10% minimum of 490; x 100
	if got.Facts["capital"] != "65000.00" {
		t.Fatalf("got %+v", got.Facts)
	}
}

func TestFitNewEquityNakedUsesTwentyPercentWithTenPercentFloor(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "C", "60", "-1", "0.5", "50")}}
	got, _ := run(t, strategy.RiskBook{Funds: funds("100000")}, strategy.RiskOutcome{}, c)
	// 20% x 50 - 10 otm = 0, floor 10% x 50 = 5, plus option value 0.5 = 5.5; x 100
	if got.Facts["capital"] != "550.00" {
		t.Fatalf("floor: %+v", got.Facts)
	}
	c = strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "C", "52", "-1", "1", "50")}}
	got, _ = run(t, strategy.RiskBook{Funds: funds("100000")}, strategy.RiskOutcome{}, c)
	// 10 - 2 otm = 8 above the floor 5, plus 1 = 9; x 100
	if got.Facts["capital"] != "900.00" {
		t.Fatalf("twenty: %+v", got.Facts)
	}
}

func TestFitNewSpreadCapitalIsWidthLessCredit(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{
		leg("AAA", "P", "95", "1", "1.00", "103"), leg("AAA", "P", "100", "-1", "2.50", "103")}}
	got, _ := run(t, strategy.RiskBook{Funds: funds("1000")}, strategy.RiskOutcome{}, c)
	// (5 - 1.5) x 100
	if got.Facts["capital"] != "350.00" {
		t.Fatalf("got %+v", got.Facts)
	}
}

func TestFitNewCoveredCallNeedsNoCapital(t *testing.T) {
	t.Parallel()
	shares := strategy.RiskHolding{Underlying: "AAA", Currency: "USD", Quantity: "100", Multiplier: "1", Spot: "50"}
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "C", "55", "-1", "1", "50")}}
	got, _ := run(t, strategy.RiskBook{Funds: funds("0"), Holdings: []strategy.RiskHolding{shares}}, strategy.RiskOutcome{}, c)
	if got.Facts["capital"] != "0.00" || got.Check.Status != "pass" {
		t.Fatalf("got %+v", got)
	}
}

func TestFitNewNoFundsIsUnknownNotBlock(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "P", "100", "-1", "3", "105")}}
	got, _ := run(t, strategy.RiskBook{}, strategy.RiskOutcome{}, c)
	if got.Check.Status != "unknown" || got.Check.Reason != "no_funds" || got.Facts["capital"] != "10000.00" {
		t.Fatalf("got %+v", got)
	}
}

func TestFitNewOtherCurrencyIsUnknownNoFx(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "P", "100", "-1", "3", "105")}}
	book := strategy.RiskBook{Funds: &strategy.RiskFunds{Currency: "JPY", Available: "99999999"}}
	got, _ := run(t, book, strategy.RiskOutcome{}, c)
	if got.Check.Status != "unknown" || got.Check.Reason != "no_fx" {
		t.Fatalf("got %+v", got)
	}
}

func TestFitNewCapitalOverAvailableBlocks(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "P", "100", "-1", "3", "105")}}
	got, _ := run(t, strategy.RiskBook{Funds: funds("9999.99")}, strategy.RiskOutcome{}, c)
	if got.Check.Status != "block" || got.Check.Reason != "capital" {
		t.Fatalf("over: %+v", got)
	}
	got, _ = run(t, strategy.RiskBook{Funds: funds("10000")}, strategy.RiskOutcome{}, c)
	if got.Check.Status != "pass" {
		t.Fatalf("equal: %+v", got)
	}
}

func TestFitNewNoRuleIsUnknown(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "P", "100", "1", "3", "105")}}
	got, _ := run(t, strategy.RiskBook{Funds: funds("1")}, strategy.RiskOutcome{}, c)
	if got.Check.Status != "unknown" || got.Check.Reason != "no_rule" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := got.Facts["capital"]; ok {
		t.Fatalf("capital present: %+v", got.Facts)
	}
}

func TestFitNewOpenPutStrikesSumsTheOneOpenShortPut(t *testing.T) {
	t.Parallel()
	book := strategy.RiskBook{Funds: funds("1"), Holdings: []strategy.RiskHolding{
		leg("BBB", "P", "40", "-3", "1", "45"), // counts: 40 x 100 x 3 = 12000
		leg("BBB", "P", "30", "2", "1", "45"),  // long: no
		leg("BBB", "C", "40", "-1", "1", "45"), // call: no
		{Underlying: "JJJ", Currency: "JPY", Quantity: "-1", Multiplier: "100", Option: &strategy.RiskLeg{Right: "P", Strike: "900"}}, // other currency: no
	}}
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "P", "100", "-1", "3", "105")}}
	got, _ := run(t, book, strategy.RiskOutcome{}, c)
	if got.Facts["open_put_strikes"] != "12000.00" {
		t.Fatalf("got %+v", got.Facts)
	}
}

func TestFitNewIncompleteBookHasNoWorstLossWith(t *testing.T) {
	t.Parallel()
	c := strategy.RiskCandidate{Class: "equity", Legs: []strategy.RiskHolding{leg("AAA", "P", "100", "-1", "3", "105")}}
	out := strategy.RiskOutcome{Excluded: []strategy.RiskExcluded{{Underlying: "ZZZ", Currency: "USD", Reason: "no_vol"}}}
	got, st := run(t, strategy.RiskBook{Funds: funds("20000")}, out, c)
	if _, ok := got.Facts["worst_loss_with"]; ok {
		t.Fatalf("present: %+v", got.Facts)
	}
	if len(st.books) != 0 {
		t.Fatalf("stress was called")
	}
	// a complete total in another currency is not a total in the candidate's currency
	out = strategy.RiskOutcome{Totals: []strategy.RiskTotal{{Currency: "JPY", WorstLoss: "5"}}}
	got, _ = run(t, strategy.RiskBook{Funds: funds("20000")}, out, c)
	if _, ok := got.Facts["worst_loss_with"]; ok {
		t.Fatalf("present: %+v", got.Facts)
	}
}

func TestFitNewWorstLossWithAddsTheCandidateToItsClass(t *testing.T) {
	t.Parallel()
	held := leg("BBB", "P", "40", "-1", "1", "45")
	held.Class = "equity"
	book := strategy.RiskBook{Alias: "a", Funds: funds("20000"), Holdings: []strategy.RiskHolding{held}}
	c := strategy.RiskCandidate{Class: "broad_high", Legs: []strategy.RiskHolding{leg("SPX", "P", "4900", "-1", "20", "5000")}}
	got, st := run(t, book, total("1234.50"), c)
	if got.Facts["worst_loss_with"] != "1234.50" {
		t.Fatalf("got %+v", got.Facts)
	}
	if len(st.books) != 1 || len(st.books[0].Holdings) != 2 {
		t.Fatalf("books %+v", st.books)
	}
	added := st.books[0].Holdings[1]
	if added.Underlying != "SPX" || added.Class != "broad_high" || st.books[0].Alias != "a" {
		t.Fatalf("added %+v", added)
	}
	if len(book.Holdings) != 1 {
		t.Fatalf("caller's book changed")
	}
}
