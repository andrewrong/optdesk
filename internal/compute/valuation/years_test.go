package valuation

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
)

func TestNonConsecutiveRefused(t *testing.T) {
	t.Parallel()
	// six eligible years, but 2021 is missing inside the latest five
	got := BuildFilings(company(t, 2018, 2023, "3571", 2021), "2024-12-31", valuationapi.USGAAP)
	eq(t, valuationapi.StateMissingInput, got.State)
	eq(t, []string{"non_consecutive_years"}, got.Reasons)

	// a gap in older years only keeps the newest run; the five latest are consecutive
	ok := BuildFilings(company(t, 2014, 2023, "3571", 2016), "2024-12-31", valuationapi.USGAAP)
	mustEq(t, valuationapi.StateOK, ok.State, ok.Reasons)
	hasLen(t, ok.Filings.Years, 7)
	eq(t, "2017-12-31", string(ok.Filings.Years[6].FYEnd))
}

func TestQuarterlyNeverMakesYear(t *testing.T) {
	t.Parallel()
	facts := company(t, 2020, 2023, "3571") // four annual years
	// quarters on 10-Q, a year-long period on a 10-Q, and a 100-day period on a 10-K
	facts = append(facts,
		mk(t, ocf, "USD", "2019-10-01", "2019-12-31", "2020-02-01", "10-Q", "1"),
		mk(t, ocf, "USD", "2019-01-01", "2019-12-31", "2020-02-01", "10-Q", "1"),
		mk(t, ocf, "USD", "2019-01-01", "2019-04-11", "2020-02-01", "10-K", "1"),
		mk(t, ocf, "USD", "2018-01-01", "2018-12-31", "2019-02-01", "8-K", "1"),
	)
	got := BuildFilings(facts, "2024-12-31", valuationapi.USGAAP)
	eq(t, valuationapi.StateMissingInput, got.State)
	eq(t, []string{"fewer_than_5_years"}, got.Reasons)
	hasLen(t, got.Filings.Years, 0)
}

func TestDivKeeps34Digits(t *testing.T) {
	t.Parallel()
	d := decimal.RequireFromString
	cases := []struct{ a, b, want string }{
		{"1", "3", "0.3333333333333333333333333333333333"},
		{"2", "3", "0.6666666666666666666666666666666667"},
		{"10", "3", "3.333333333333333333333333333333333"},
		{"-1", "3", "-0.3333333333333333333333333333333333"},
		{"1", "-3", "-0.3333333333333333333333333333333333"},
		{"22", "7", "3.142857142857142857142857142857143"},
		{"1000000", "3", "333333.3333333333333333333333333333"},
		{"0.0001", "3", "0.00003333333333333333333333333333333333"},
		{"6", "3", "2"},
		{"1", "4", "0.25"},
		// 1/2^50 has 35 significant digits ending in 5: the tie goes to the even digit
		{"1", "1125899906842624", "8.881784197001252323389053344726562E-16"},
		// 3/2^50 is exact in 36 digits; 266453525910037569701671600341796875 drops to ...797 (up)
		{"3", "1125899906842624", "2.664535259100375697016716003417969E-15"},
	}
	for _, c := range cases {
		got, ok := div(d(c.a), d(c.b))
		mustTrue(t, ok, c.a+"/"+c.b)
		isTrue(t, got.Equal(d(c.want)), "%s/%s = %s, want %s", c.a, c.b, got, c.want)
	}
	// a quotient with more digits than 34 never keeps more than 34
	q, _ := div(d("1"), d("7"))
	eq(t, 34, len(q.Coefficient().String()))

	zero, ok := div(d("5"), decimal.Zero)
	isFalse(t, ok)
	isTrue(t, zero.IsZero())
}
