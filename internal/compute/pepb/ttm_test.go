package pepb_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func ytd(doc, filed, start, end, val string) contracts.FundamentalFact {
	return income("10-Q", doc, filed, start, end, val)
}

func TestTTMAnnualPlusYTDMinusPriorYTD(t *testing.T) {
	t.Parallel()
	facts := append(baseFacts(),
		ytd("q3", "2023-11-01", "2023-01-01", "2023-09-30", "900"),
		ytd("q3", "2023-11-01", "2022-01-01", "2022-09-30", "700"))
	// TTM = 1000 + 900 - 700 = 1200; close 12 x 100 shares = 1200, so PE 1.0.
	r := runLast(facts, series("2023-10-25", 12, flat("12"))).PE // to 2023-11-05
	mustEq(t, "state", r.State, pepbapi.StateOK)
	mustEq(t, "ratio", r.Ratio, "1.0")
	mustEq(t, "basis end", string(r.BasisEnd), "2023-09-30")
	mustEq(t, "basis filed", string(r.BasisFiled), "2023-11-01")
	// Before the 10-Q was filed the basis is A alone: 1200 / 1000 = 1.2.
	mustEq(t, "10-Q not yet filed", pointText(t, r.History, "2023-10-31"), "1.2")
	// Right after the 10-K, TTM is A alone.
	r = runLast(facts, series("2023-02-05", 6, flat("12"))).PE
	mustEq(t, "after 10-K", r.Ratio, "1.2")
	mustEq(t, "after 10-K basis end", string(r.BasisEnd), "2022-12-31")
	mustEq(t, "after 10-K basis filed", string(r.BasisFiled), "2023-02-01")
}

func TestMissingPriorYTDGivesNoFigure(t *testing.T) {
	t.Parallel()
	bars := series("2023-11-02", 5, flat("12"))
	// Y without P.
	r := runLast(append(baseFacts(), ytd("q3", "2023-11-01", "2023-01-01", "2023-09-30", "900")), bars).PE
	mustEq(t, "no P state", r.State, pepbapi.StateMissing)
	mustEq(t, "no P reason", r.Reason, "ttm_piece_missing")
	mustEq(t, "no P ratio", r.Ratio, "")
	// P without Y: only 3-month rows of the second and third quarters are stored, no
	// year-to-date row starts right after A. The three quarters are never summed.
	facts := append(baseFacts(),
		ytd("q3", "2023-11-01", "2023-04-01", "2023-06-30", "300"),
		ytd("q3", "2023-11-01", "2023-07-01", "2023-09-30", "300"),
		ytd("q3", "2023-11-01", "2022-01-01", "2022-09-30", "700"))
	r = runLast(facts, bars).PE
	mustEq(t, "no Y state", r.State, pepbapi.StateMissing)
	mustEq(t, "no Y reason", r.Reason, "ttm_piece_missing")
	mustEq(t, "no Y ratio", r.Ratio, "")
	if len(r.History) != 0 {
		t.Fatalf("history = %v", r.History)
	}
}

func TestLossGivesNoPE(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"-100", "0"} {
		facts := append(baseFacts()[:1:1],
			income("10-K", "k1", "2023-02-01", "2022-01-01", "2022-12-31", v),
			equity("k1", "2023-02-01", "2022-12-31", "5000"))
		r := runLast(facts, series("2023-03-01", 5, flat("10")))
		mustEq(t, v+" state", r.PE.State, pepbapi.StateLoss)
		mustEq(t, v+" reason", r.PE.Reason, "loss")
		mustEq(t, v+" text", r.PE.Text, "loss, no PE")
		mustEq(t, v+" ratio", r.PE.Ratio, "")
		mustEq(t, v+" PB", r.PB.Ratio, "0.2")
	}
}

func TestNegativeEquityGivesNoPB(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"-5", "0"} {
		facts := append(baseFacts()[:2:2], equity("k1", "2023-02-01", "2022-12-31", v))
		r := runLast(facts, series("2023-03-01", 5, flat("10")))
		mustEq(t, v+" state", r.PB.State, pepbapi.StateLoss)
		mustEq(t, v+" reason", r.PB.Reason, "negative_equity")
		mustEq(t, v+" text", r.PB.Text, "negative equity, no PB")
		mustEq(t, v+" ratio", r.PB.Ratio, "")
		mustEq(t, v+" PE", r.PE.Ratio, "1.0")
	}
}

func TestTTMToleranceAcceptsFiftyThreeWeekYear(t *testing.T) {
	t.Parallel()
	yEnd := day("2023-09-30")
	build := func(annualDays, priorGap int) pepbapi.Reading {
		aStart := day("2022-01-01")
		aEnd := aStart.AddDate(0, 0, annualDays)
		facts := []contracts.FundamentalFact{
			cover("k1", "2023-02-01", "100"),
			income("10-K", "k1", "2023-02-01", "2022-01-01", aEnd.Format("2006-01-02"), "1000"),
			ytd("q3", "2023-11-01", aEnd.AddDate(0, 0, 1).Format("2006-01-02"), "2023-09-30", "900"),
			ytd("q3", "2023-11-01", "2022-01-01", yEnd.AddDate(0, 0, -priorGap).Format("2006-01-02"), "700"),
		}
		return runLast(facts, series("2023-11-02", 5, flat("12"))).PE
	}
	r := build(371, 371)
	mustEq(t, "371/371 state", r.State, pepbapi.StateOK)
	mustEq(t, "371/371 ratio", r.Ratio, "1.0")
	r = build(371, 372)
	mustEq(t, "prior 372 reason", r.Reason, "ttm_piece_missing")
	r = build(372, 371)
	mustEq(t, "annual 372 reason", r.Reason, "no_annual")
}
