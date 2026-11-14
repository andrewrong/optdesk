package pepb_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestNonUSDGivesNoFigure(t *testing.T) {
	t.Parallel()
	facts := []contracts.FundamentalFact{
		cover("k1", "2023-02-01", "100"),
		fact("us-gaap", "NetIncomeLoss", "JPY", "10-K", "k1", "2023-02-01", "2022-01-01", "2022-12-31", "1000"),
		fact("us-gaap", "StockholdersEquity", "EUR", "10-K", "k1", "2023-02-01", "", "2022-12-31", "5000"),
	}
	r := runLast(facts, series("2023-03-01", 5, flat("10")))
	mustEq(t, "PE state", r.PE.State, pepbapi.StateMissing)
	mustEq(t, "PE reason", r.PE.Reason, "non_usd")
	mustEq(t, "PB reason", r.PB.Reason, "non_usd")
	mustEq(t, "PE ratio", r.PE.Ratio, "")
	mustEq(t, "PB ratio", r.PB.Ratio, "")
}

func TestEquityTakesLatestPeriodEnd(t *testing.T) {
	t.Parallel()
	facts := []contracts.FundamentalFact{
		cover("k1", "2024-02-01", "100"),
		equity("k1", "2024-02-01", "2023-12-31", "5000"),
		// filed later, but a prior-year balance
		equity("k2", "2024-02-02", "2022-12-31", "2500"),
	}
	r := runLast(facts, series("2024-03-01", 5, flat("10"))).PB
	mustEq(t, "state", r.State, pepbapi.StateOK)
	mustEq(t, "basis end", string(r.BasisEnd), "2023-12-31")
	mustEq(t, "ratio", r.Ratio, "0.2") // 10 x 100 / 5000
}

func TestAmendmentAndLatestFiledRowWin(t *testing.T) {
	t.Parallel()
	facts := append(baseFacts(),
		income("10-K/A", "ka", "2023-03-01", "2022-01-01", "2022-12-31", "2000"))
	bars := series("2023-02-10", 40, flat("10")) // to 2023-03-21
	// Filed after the day: the 10-K value 1000 stands.
	r := run(facts, bars, "2023-02-28").PE
	mustEq(t, "before amendment", r.Ratio, "1.0")
	// Filed before the day: the 10-K/A value 2000 replaces it.
	r = run(facts, bars, "2023-03-10").PE
	mustEq(t, "after amendment", r.Ratio, "0.5")
	mustEq(t, "basis filed", string(r.BasisFiled), "2023-03-01")
	mustEq(t, "history keeps the old day", pointText(t, r.History, "2023-02-28"), "1.0")
}

func TestNoAnnualFactGivesNoAnnualReason(t *testing.T) {
	t.Parallel()
	facts := []contracts.FundamentalFact{
		cover("q1", "2023-05-01", "100"),
		ytd("q1", "2023-05-01", "2023-01-01", "2023-03-31", "300"),
		equity("q1", "2023-05-01", "2023-03-31", "5000"),
	}
	r := runLast(facts, series("2023-05-05", 5, flat("10")))
	mustEq(t, "PE state", r.PE.State, pepbapi.StateMissing)
	mustEq(t, "PE reason", r.PE.Reason, "no_annual")
	mustEq(t, "PE text", r.PE.Text, "PE: no 10-K stored")
	mustEq(t, "PB state", r.PB.State, pepbapi.StateOK)
}

func diluted(doc, start, end, val string) contracts.FundamentalFact {
	return fact("us-gaap", "WeightedAverageNumberOfDilutedSharesOutstanding", "shares", "10-Q", doc,
		"2023-11-01", start, end, val)
}

func TestShareCountMismatchGivesNoFigure(t *testing.T) {
	t.Parallel()
	base := func(extra ...contracts.FundamentalFact) pepbapi.Result {
		facts := append(baseFacts()[1:],
			cover("q3", "2023-11-01", "100"))
		return runLast(append(facts, extra...), series("2023-11-02", 5, flat("10")))
	}
	// The 3-month row (50) is compared and differs by more than 25 percent; the year-to-date
	// row (100) would have matched.
	r := base(diluted("q3", "2023-07-01", "2023-09-30", "50"), diluted("q3", "2023-01-01", "2023-09-30", "100"))
	mustEq(t, "mismatch reason", r.PE.Reason, "share_count_mismatch")
	mustEq(t, "PB reason", r.PB.Reason, "share_count_mismatch")
	// The 3-month row matches (within 25 percent); the year-to-date row alone would not.
	r = base(diluted("q3", "2023-07-01", "2023-09-30", "90"), diluted("q3", "2023-01-01", "2023-09-30", "40"))
	mustEq(t, "match state", r.PE.State, pepbapi.StateOK)
	// A diluted row of another filing is not compared.
	r = base(diluted("other", "2023-07-01", "2023-09-30", "10"))
	mustEq(t, "other doc state", r.PE.State, pepbapi.StateOK)
	// A filing with only the BasicAndDiluted tag is not checked.
	r = base(fact("us-gaap", "WeightedAverageNumberOfShareOutstandingBasicAndDiluted", "shares", "10-Q", "q3",
		"2023-11-01", "2023-07-01", "2023-09-30", "10"))
	mustEq(t, "basic and diluted state", r.PE.State, pepbapi.StateOK)
}
