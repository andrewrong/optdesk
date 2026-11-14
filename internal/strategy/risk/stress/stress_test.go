package stress_test

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/risk/bsm"
	"github.com/andrewrong/optdesk/internal/strategy/risk/stress"
)

const (
	runDay    = contracts.Day("2026-11-14")
	oneYearOn = "2027-11-14" // 365 days after runDay
)

func sessions(old map[contracts.Day]int) func(contracts.Day) int {
	return func(d contracts.Day) int { return old[d] }
}

func book(h ...strategy.RiskHolding) strategy.RiskBook {
	return strategy.RiskBook{Alias: "mine", Currency: "USD", RunDay: runDay, Rate: "0", Holdings: h, HasCapture: true, SessionsOld: sessions(nil)}
}

func shares(sym string, class strategy.RiskClass, qty, spot string) strategy.RiskHolding {
	return strategy.RiskHolding{Underlying: sym, Currency: "USD", Class: class, Quantity: qty, Multiplier: "1", Spot: spot, SpotDay: runDay}
}

func put(class strategy.RiskClass, qty, spot, strike, mark string, cashSettled bool) strategy.RiskHolding {
	return strategy.RiskHolding{
		Underlying: "ABC", Currency: "USD", Class: class,
		Option:   &strategy.RiskLeg{Right: "P", Strike: strike, Expiry: oneYearOn, CashSettled: cashSettled},
		Quantity: qty, Multiplier: "100", Spot: spot, Mark: mark, MarkDay: runDay, SpotDay: runDay, MarkUsable: true,
	}
}

func stressIt(b strategy.RiskBook) strategy.RiskOutcome { return stress.New(bsm.New()).Stress(b) }

func total(o strategy.RiskOutcome, cur string) (string, bool) {
	for _, t := range o.Totals {
		if t.Currency == cur {
			return t.WorstLoss, true
		}
	}
	return "", false
}

func classOf(t *testing.T, o strategy.RiskOutcome, cur string, c strategy.RiskClass) strategy.RiskClassResult {
	t.Helper()
	for _, r := range o.Classes {
		if r.Currency == cur && r.Class == c {
			return r
		}
	}
	t.Fatalf("no class %s %s in %+v", cur, c, o.Classes)
	return strategy.RiskClassResult{}
}

func TestStressNewShortPutWorstPointEqualsHandValue(t *testing.T) {
	t.Parallel()
	// ATM put, S=K=100, one year, r=0, vol 0.2: price 100*(2N(0.1)-1) = 7.9656. Repriced at 85 with the
	// implied vol 0.2000008: 17.16134. Short one contract of 100: -100*(17.16134-7.9656) = -919.57.
	o := stressIt(book(put("equity", "-1", "100", "100", "7.9656", false)))
	r := classOf(t, o, "USD", "equity")
	if r.Worst.Move != "-0.15" || r.Worst.PnL != "-919.57" || r.Worst.AssignmentCash != "10000.00" {
		t.Fatalf("worst %+v", r.Worst)
	}
	if got, _ := total(o, "USD"); got != "919.57" {
		t.Fatalf("total %q", got)
	}
}

func TestStressNewWorstPointOfBroadHighIsTheMinusEightEnd(t *testing.T) {
	t.Parallel()
	r := classOf(t, stressIt(book(shares("SPX", "broad_high", "10", "100"))), "USD", "broad_high")
	if r.Worst.Move != "-0.08" || r.Worst.PnL != "-80.00" {
		t.Fatalf("worst %+v", r.Worst)
	}
}

func TestStressNewClassesAreSummedWithoutOffset(t *testing.T) {
	t.Parallel()
	// worsts: equity long 10 at -15% = -150; broad_high long 10 at -8% = -80; broad_other short 10 at +10% = -100.
	o := stressIt(book(shares("A", "equity", "10", "100"), shares("SPX", "broad_high", "10", "100"), shares("RUT", "broad_other", "-10", "100")))
	if got, _ := total(o, "USD"); got != "330.00" {
		t.Fatalf("total %q, want 330.00", got)
	}
}

func TestStressNewCashSettledIndexLegHasNoAssignmentCash(t *testing.T) {
	t.Parallel()
	o := stressIt(book(put("broad_high", "-1", "100", "100", "7.9656", true)))
	r := classOf(t, o, "USD", "broad_high")
	for _, p := range r.Ladder {
		if p.AssignmentCash != "0.00" {
			t.Fatalf("assignment cash %+v", p)
		}
	}
	if r.Worst.PnL == "0.00" {
		t.Fatalf("loss missing: %+v", r.Worst)
	}
}

func TestStressNewEmptyExerciseFieldsCountAsAssignable(t *testing.T) {
	t.Parallel()
	// CashSettled false is how the loader reports unknown or empty terms of an equity option.
	r := classOf(t, stressIt(book(put("equity", "-2", "100", "100", "7.9656", false))), "USD", "equity")
	if r.Ladder[0].AssignmentCash != "20000.00" {
		t.Fatalf("assignment cash %+v", r.Ladder[0])
	}
	if r.Ladder[10].AssignmentCash != "0.00" { // spot 115 above the strike
		t.Fatalf("assignment cash %+v", r.Ladder[10])
	}
}

func TestStressNewStaleMarkIsExcludedNotPriced(t *testing.T) {
	t.Parallel()
	b := book(put("equity", "-1", "100", "100", "7.9656", false))
	b.SessionsOld = sessions(map[contracts.Day]int{runDay: 6})
	o := stressIt(b)
	if len(o.Excluded) != 1 || o.Excluded[0].Reason != "stale_mark" || len(o.Classes) != 0 || len(o.Totals) != 0 {
		t.Fatalf("outcome %+v", o)
	}
	b = book(put("equity", "-1", "100", "100", "7.9656", false))
	b.Holdings[0].MarkUsable = false
	if o := stressIt(b); len(o.Excluded) != 1 || o.Excluded[0].Reason != "stale_mark" {
		t.Fatalf("unusable mark: %+v", o)
	}
}

func TestStressNewMarkDayOtherThanSpotDayIsStale(t *testing.T) {
	t.Parallel()
	b := book(put("equity", "-1", "100", "100", "7.9656", false))
	b.Holdings[0].MarkDay = "2026-11-13"
	if o := stressIt(b); len(o.Excluded) != 1 || o.Excluded[0].Reason != "stale_mark" {
		t.Fatalf("outcome %+v", o)
	}
}

func TestStressNewIncompleteCurrencyHasClassWorstsAndNoTotal(t *testing.T) {
	t.Parallel()
	bad := put("equity", "-1", "100", "100", "7.9656", false)
	bad.MarkUsable = false
	o := stressIt(book(shares("A", "equity", "10", "100"), bad))
	if len(o.Classes) != 1 || o.Classes[0].Worst.PnL != "-150.00" {
		t.Fatalf("classes %+v", o.Classes)
	}
	if len(o.Totals) != 0 {
		t.Fatalf("totals %+v", o.Totals)
	}
}

func TestStressNewDeepItmPutAtIntrinsicIsValuedNotExcluded(t *testing.T) {
	t.Parallel()
	// spot 80, strike 100, mark 20 equals intrinsic: no vol solves it. At 68 the intrinsic is 32:
	// short one contract: -100*(32-20) = -1200.
	o := stressIt(book(put("equity", "-1", "80", "100", "20", false)))
	if len(o.Excluded) != 0 {
		t.Fatalf("excluded %+v", o.Excluded)
	}
	r := classOf(t, o, "USD", "equity")
	if r.Worst.PnL != "-1200.00" || r.Ladder[10].PnL != "1200.00" {
		t.Fatalf("ladder %+v", r.Ladder)
	}
	if got, ok := total(o, "USD"); !ok || got != "1200.00" {
		t.Fatalf("total %q", got)
	}
}

func jpyShares(mark string) strategy.RiskHolding {
	return strategy.RiskHolding{Underlying: "7203", Currency: "JPY", Class: "", Quantity: "100", Multiplier: "1",
		Spot: "1000", Mark: mark, MarkDay: runDay, SpotDay: runDay, MarkUsable: true}
}

func TestStressNewOtherCurrencyHasItsOwnTotal(t *testing.T) {
	t.Parallel()
	// a JPY line needs a class row and a mark to be stressed, at the equity range.
	h := jpyShares("1000")
	h.Class = "equity"
	o := stressIt(book(h))
	if got, ok := total(o, "JPY"); !ok || got != "15000.00" {
		t.Fatalf("totals %+v", o.Totals)
	}
	if _, ok := total(o, "USD"); ok {
		t.Fatal("USD total of an empty currency")
	}
}

func TestStressNewUsdTotalIsCompleteWhileJpyIsNot(t *testing.T) {
	t.Parallel()
	h := jpyShares("")
	h.Class = "equity"
	o := stressIt(book(shares("A", "equity", "10", "100"), h))
	if got, ok := total(o, "USD"); !ok || got != "150.00" {
		t.Fatalf("totals %+v", o.Totals)
	}
	if _, ok := total(o, "JPY"); ok {
		t.Fatal("JPY total despite a holding not stressed")
	}
	if o.NotStressed["JPY"] != 1 {
		t.Fatalf("not stressed %v", o.NotStressed)
	}
}

func TestStressNewOldestSpotDayPerClass(t *testing.T) {
	t.Parallel()
	a, b, c := shares("A", "equity", "1", "100"), shares("B", "equity", "1", "100"), shares("SPX", "broad_high", "1", "100")
	a.SpotDay, b.SpotDay, c.SpotDay = "2026-11-12", "2026-11-10", "2026-11-13"
	o := stressIt(book(a, b, c))
	if got := classOf(t, o, "USD", "equity").OldestSpotDay; got != "2026-11-10" {
		t.Fatalf("equity oldest %s", got)
	}
	if got := classOf(t, o, "USD", "broad_high").OldestSpotDay; got != "2026-11-13" {
		t.Fatalf("broad_high oldest %s", got)
	}
}

func TestStressNewElevenPointsOnTenEqualIntervalsForAsymmetricRange(t *testing.T) {
	t.Parallel()
	r := classOf(t, stressIt(book(shares("SPX", "broad_high", "1", "100"))), "USD", "broad_high")
	var got []string
	for _, p := range r.Ladder {
		got = append(got, p.Move)
	}
	want := []string{"-0.08", "-0.066", "-0.052", "-0.038", "-0.024", "-0.01", "0.004", "0.018", "0.032", "0.046", "0.06"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("moves %v", got)
	}
}

func TestStressNewNoCaptureAliasHasNoTotal(t *testing.T) {
	t.Parallel()
	b := book(put("equity", "-1", "100", "100", "7.9656", false))
	b.HasCapture = false
	o := stressIt(b)
	if len(o.Totals) != 0 {
		t.Fatalf("totals %+v", o.Totals)
	}
	found := false
	for _, e := range o.Excluded {
		found = found || e.Reason == "no_capture"
	}
	if !found {
		t.Fatalf("excluded %+v", o.Excluded)
	}
	if len(o.Classes) != 1 {
		t.Fatalf("class worsts missing: %+v", o.Classes)
	}
}

func TestStressNewSharesOnlyBookIsLinear(t *testing.T) {
	t.Parallel()
	b := book(shares("A", "equity", "10", "100"))
	b.HasCapture = false // the ledger carries the shares, so this is no exclusion
	o := stressIt(b)
	r := classOf(t, o, "USD", "equity")
	if r.Ladder[0].PnL != "-150.00" || r.Ladder[5].PnL != "0.00" || r.Ladder[10].PnL != "150.00" || r.Ladder[3].PnL != "-60.00" {
		t.Fatalf("ladder %+v", r.Ladder)
	}
	if got, _ := total(o, "USD"); got != "150.00" || len(o.Excluded) != 0 {
		t.Fatalf("total %q excluded %+v", got, o.Excluded)
	}
}

// the real pricer satisfies the interface of this package.
var _ stress.Pricer = bsm.New()

func TestStressNewItmCallBelowDiscountedBoundIsValuedNotExcluded(t *testing.T) {
	t.Parallel()
	b := book(strategy.RiskHolding{Underlying: "ABC", Currency: "USD", Class: "equity",
		Option:   &strategy.RiskLeg{Right: "C", Strike: "100", Expiry: oneYearOn},
		Quantity: "-1", Multiplier: "100", Spot: "120", Mark: "22", MarkDay: runDay, SpotDay: runDay, MarkUsable: true})
	b.Rate = "0.05"
	o := stressIt(b)
	if len(o.Excluded) != 0 {
		t.Fatalf("priced ITM call excluded: %+v", o.Excluded)
	}
	// valued at intrinsic at the moved spot: at +15% the spot is 138, intrinsic 38, short one contract of 100:
	// -100*(38-22) = -1600.
	if got, ok := total(o, "USD"); !ok || got != "1600.00" {
		t.Fatalf("totals %+v", o.Totals)
	}
}

func TestStressNewUnparsableRateIsNoVolNotIntrinsic(t *testing.T) {
	t.Parallel()
	b := book(put("equity", "-1", "100", "90", "3", false))
	b.Rate = "n/a"
	o := stressIt(b)
	if len(o.Excluded) != 1 || o.Excluded[0].Reason != "no_vol" {
		t.Fatalf("excluded %+v totals %+v", o.Excluded, o.Totals)
	}
	if _, ok := total(o, "USD"); ok {
		t.Fatalf("total with an unparsable rate: %+v", o.Totals)
	}
}
