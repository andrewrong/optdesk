package companydata_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestSingleQuarterFrom3MFact(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{ni("q1", f1, s25, e1, "100", 2025, "Q1")})
	if len(qs) != 1 {
		t.Fatalf("quarters = %+v", qs)
	}
	wantCell(t, "Q1", qs[0].NetIncome, "100", false)
	if qs[0].PeriodEnd != e1 || qs[0].Label != "FY2025 Q1" {
		t.Fatalf("quarter = %+v", qs[0])
	}
}

func TestQ2FromYearToDate(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{
		ni("q1", f1, s25, e1, "100", 2025, "Q1"),
		ni("q2", f2, s25, e2, "250", 2025, "Q2"),
	})
	wantCell(t, "Q2", byNumber(t, qs, 2).NetIncome, "150", true)
	if qs[0].Number != 2 {
		t.Fatalf("not newest first: %+v", qs)
	}
}

func TestQ4FromYearMinusNineMonths(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{
		ni("q3", f3, s25, e3, "700", 2025, "Q3"),
		ni("k", fK, s25, e4, "1000", 2025, "FY"),
	})
	q := byNumber(t, qs, 4)
	wantCell(t, "Q4", q.NetIncome, "300", true)
	if q.Label != "FY2025 Q4" || q.FilingDate != fK {
		t.Fatalf("quarter = %+v", q)
	}
}

func TestSeptemberFiscalYearLabels(t *testing.T) {
	t.Parallel()
	s := "2024-09-29"
	facts := []contracts.FundamentalFact{
		ni("q1", "2025-02-01", s, "2024-12-28", "10", 2025, "Q1"),
		ni("q2", "2025-05-01", s, "2025-03-29", "30", 2025, "Q2"),
		ni("q3", "2025-08-01", s, "2025-06-28", "60", 2025, "Q3"),
		ni("k", "2025-11-01", s, "2025-09-27", "100", 2025, "FY"),
	}
	qs := must(t, facts)
	if len(qs) != 4 {
		t.Fatalf("quarters = %+v", qs)
	}
	for n := 1; n <= 4; n++ {
		q := byNumber(t, qs, n)
		if q.Label != fmt.Sprintf("FY2025 Q%d", n) || q.FiscalYear != 2025 {
			t.Fatalf("Q%d = %+v", n, q)
		}
	}
	wantCell(t, "Q2", byNumber(t, qs, 2).NetIncome, "20", true)
}

const (
	epsC = "EarningsPerShareDiluted"
	shC  = "WeightedAverageNumberOfDilutedSharesOutstanding"
)

func eps(doc, filed, start, end, val, fp string) contracts.FundamentalFact {
	return fact(epsC, "USD/shares", doc, filed, start, end, val, 2025, fp)
}

func sh(doc, filed, start, end, val, fp string) contracts.FundamentalFact {
	return fact(shC, "shares", doc, filed, start, end, val, 2025, fp)
}

// q3Start is the start of the 3M fact of Q2 to Q4.
const (
	q2s = "2025-04-01"
	q3s = "2025-07-01"
	q4s = "2025-10-01"
)

func epsYear(q2 bool) []contracts.FundamentalFact {
	fs := []contracts.FundamentalFact{
		eps("q1", f1, s25, e1, "1.00", "Q1"),
		eps("q3", f3, q3s, e3, "0.95", "Q3"),
		eps("k", fK, s25, e4, "4.00", "FY"),
		sh("q1", f1, s25, e1, "100", "Q1"),
		sh("q2", f2, q2s, e2, "101", "Q2"),
		sh("q3", f3, q3s, e3, "102", "Q3"),
		sh("k", fK, s25, e4, "101", "FY"),
	}
	if q2 {
		fs = append(fs, eps("q2", f2, q2s, e2, "0.90", "Q2"))
	}
	return fs
}

func TestEPSQ4YearMinusThreeQs(t *testing.T) {
	t.Parallel()
	wantCell(t, "Q4 eps", byNumber(t, must(t, epsYear(true)), 4).EPSDiluted, "1.15", true)
}

func TestEPSQ4MissingIfAQtrMissing(t *testing.T) {
	t.Parallel()
	wantMissing(t, "Q4 eps", byNumber(t, must(t, epsYear(false)), 4).EPSDiluted, companydataapi.WhyCannotDerive)
}

func TestEPSQ4Uses3MonthFactIfFiled(t *testing.T) {
	t.Parallel()
	fs := append(epsYear(true), eps("k", fK, q4s, e4, "1.20", "FY"))
	wantCell(t, "Q4 eps", byNumber(t, must(t, fs), 4).EPSDiluted, "1.20", false)
}

func TestSharesQ4Only3MonthFact(t *testing.T) {
	t.Parallel()
	q := byNumber(t, must(t, epsYear(true)), 4)
	wantMissing(t, "Q4 shares", q.SharesDiluted, companydataapi.WhyNoFact)
	fs := append(epsYear(true), sh("k", fK, q4s, e4, "103", "FY"))
	wantCell(t, "Q4 shares", byNumber(t, must(t, fs), 4).SharesDiluted, "103", false)
}

func TestMissingOperandNotZero(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{ni("k", fK, s25, e4, "1000", 2025, "FY")})
	wantMissing(t, "Q4", byNumber(t, qs, 4).NetIncome, companydataapi.WhyCannotDerive)
}

// yearOfDays is a fiscal year of n days (both ends in) from 2025-01-01 with a 9M fact and the 12M fact.
func yearOfDays(n int) []contracts.FundamentalFact {
	end := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, n-1).Format("2006-01-02")
	return []contracts.FundamentalFact{
		ni("q3", f3, s25, e3, "700", 2025, ""),
		ni("k", fK, s25, end, "1000", 2025, ""),
	}
}

func TestYearBandIs364To371(t *testing.T) {
	t.Parallel()
	for n, ok := range map[int]bool{363: false, 364: true, 371: true, 372: false} {
		c := byNumber(t, must(t, yearOfDays(n)), 4).NetIncome
		if ok && (c.Missing || c.Value != "300") || !ok && !c.Missing {
			t.Fatalf("year of %d days: cell %+v, accepted want %v; the band is that of internal/compute/pepb/ttm.go:102-105", n, c, ok)
		}
	}
}

func TestFiftyThreeWeekYearBand(t *testing.T) {
	t.Parallel()
	wantCell(t, "371 days", byNumber(t, must(t, yearOfDays(371)), 4).NetIncome, "300", true)
	wantMissing(t, "380 days", byNumber(t, must(t, yearOfDays(380)), 4).NetIncome, companydataapi.WhyCannotDerive)
}

func TestFpDisagreesGivesPeriodMismatch(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{
		ni("q1", f1, s25, e1, "100", 2025, "Q1"),
		ni("q3", f3, s25, e3, "700", 2025, "Q2"), // periods say Q3
		ni("k", fK, s25, e4, "1000", 2025, "FY"),
	})
	for _, q := range qs {
		wantMissing(t, fmt.Sprintf("Q%d", q.Number), q.NetIncome, companydataapi.WhyPeriodMismatch)
		wantMissing(t, "free", q.FreeCash, companydataapi.WhyPeriodMismatch)
	}
	if len(qs) != 3 {
		t.Fatalf("quarters = %d", len(qs))
	}
}

func TestFiscalYearEndMonth(t *testing.T) {
	t.Parallel()
	s := "2024-09-29"
	qs := must(t, []contracts.FundamentalFact{
		ni("q1", "2025-02-01", s, "2024-12-28", "10", 2025, "Q1"),
		ni("k", "2025-11-01", s, "2025-09-27", "100", 2025, "FY"),
	})
	for _, q := range qs {
		if q.FiscalYearEndMonth != 9 {
			t.Fatalf("Q%d month = %d", q.Number, q.FiscalYearEndMonth)
		}
	}
}

func TestEmptyFpLabelsFromPeriods(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{
		ni("q1", f1, s25, e1, "100", 0, ""),
		ni("q2", f2, s25, e2, "250", 0, ""),
	})
	if len(qs) != 2 {
		t.Fatalf("quarters = %+v", qs)
	}
	wantCell(t, "Q2", byNumber(t, qs, 2).NetIncome, "150", true)
	if q := byNumber(t, qs, 1); q.Label != "" || q.FiscalYear != 0 {
		t.Fatalf("Q1 = %+v", q)
	}
	qs = must(t, []contracts.FundamentalFact{ni("q1", f1, s25, e1, "100", 2025, "")})
	if qs[0].Label != "FY2025 Q1" {
		t.Fatalf("label = %q", qs[0].Label)
	}
}

func TestSplitAfterQ1GivesEPSGap(t *testing.T) {
	t.Parallel()
	split := epsYear(true)
	for i := range split {
		if split[i].Concept == shC && split[i].DocID == "k" {
			split[i].Value = "200"
		}
	}
	wantMissing(t, "split", byNumber(t, must(t, split), 4).EPSDiluted, companydataapi.WhyShareBasisChanged)
	wantCell(t, "no split", byNumber(t, must(t, epsYear(true)), 4).EPSDiluted, "1.15", true)

	// a missing share fact among Q1, Q2, Q3 and FY is cannot_derive.
	for _, drop := range []string{"q1", "q2", "q3", "k"} {
		var fs []contracts.FundamentalFact
		for _, f := range epsYear(true) {
			if f.Concept == shC && f.DocID == drop {
				continue
			}
			if f.Concept == shC && f.DocID == "k" {
				f.Value = "200" // would be a split if it were compared
			}
			fs = append(fs, f)
		}
		wantMissing(t, "without shares of "+drop, byNumber(t, must(t, fs), 4).EPSDiluted, companydataapi.WhyCannotDerive)
	}
}

func TestOneConceptPerDerivedCell(t *testing.T) {
	t.Parallel()
	a := usd("Revenues", "k", fK, s25, e4, "1000", 2025, "FY")
	b := usd("SalesRevenueNet", "q3", f3, s25, e3, "700", 2025, "Q3")
	wantMissing(t, "mixed", byNumber(t, must(t, []contracts.FundamentalFact{a, b}), 4).Revenue, companydataapi.WhyCannotDerive)
	b2 := usd("SalesRevenueNet", "k", fK, s25, e4, "1010", 2025, "FY")
	c := byNumber(t, must(t, []contracts.FundamentalFact{a, b, b2}), 4).Revenue
	wantCell(t, "second concept", c, "310", true)
	if c.Concept != "SalesRevenueNet" {
		t.Fatalf("concept = %q", c.Concept)
	}
}

func TestInstantTakesQuarterEnd(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{
		ni("q1", f1, s25, e1, "100", 2025, "Q1"),
		usd("CashAndCashEquivalentsAtCarryingValue", "q1", f1, "", e1, "55", 2025, "Q1"),
		usd("CashAndCashEquivalentsAtCarryingValue", "q1", f1, "", "2024-12-31", "44", 2025, "Q1"),
		usd("LongTermDebtNoncurrent", "q1", f1, "", e1, "300", 2025, "Q1"),
		usd("DebtCurrent", "q1", f1, "", e1, "20", 2025, "Q1"),
	})
	q := qs[0]
	wantCell(t, "cash", q.Cash, "55", false)
	wantCell(t, "long debt", q.LongTermDebt, "300", false)
	wantCell(t, "short debt", q.ShortTermDebt, "20", false)
}

func TestConceptOrderFirstWithValue(t *testing.T) {
	t.Parallel()
	one := usd("SalesRevenueNet", "q1", f1, s25, e1, "5", 2025, "Q1")
	two := usd("Revenues", "q1", f1, s25, e1, "6", 2025, "Q1")
	three := usd("RevenueFromContractWithCustomerExcludingAssessedTax", "q1", f1, s25, e1, "7", 2025, "Q1")
	c := byNumber(t, must(t, []contracts.FundamentalFact{one, two}), 1).Revenue
	if c.Value != "6" || c.Concept != "Revenues" {
		t.Fatalf("cell = %+v", c)
	}
	c = byNumber(t, must(t, []contracts.FundamentalFact{one, two, three}), 1).Revenue
	if c.Value != "7" || !strings.HasPrefix(c.Concept, "RevenueFromContractWithCustomerExcluding") {
		t.Fatalf("cell = %+v", c)
	}
}

func TestDebtColumnsAreSeparate(t *testing.T) {
	t.Parallel()
	qs := must(t, []contracts.FundamentalFact{
		usd("LongTermDebt", "q1", f1, "", e1, "300", 2025, "Q1"),
		ni("q1", f1, s25, e1, "1", 2025, "Q1"),
	})
	q := qs[0]
	wantCell(t, "long", q.LongTermDebt, "300", false)
	wantMissing(t, "short", q.ShortTermDebt, companydataapi.WhyNoFact)
}
