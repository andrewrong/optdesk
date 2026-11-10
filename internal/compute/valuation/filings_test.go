package valuation

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

const (
	ocf = "NetCashProvidedByUsedInOperatingActivities"
	cap = "PaymentsToAcquirePropertyPlantAndEquipment"
	sbc = "ShareBasedCompensation"
	dil = "WeightedAverageNumberOfDilutedSharesOutstanding"
)

func sd(t *testing.T, day string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(dayLayout, day)
	noErr(t, err)
	return types.SerializableDate{Time: d}
}

// mk builds one SEC fact. start is empty for an instant fact.
func mk(t *testing.T, concept, unit, start, end, filed, form, value string) contracts.FundamentalFact {
	t.Helper()
	f := contracts.FundamentalFact{
		Source: gen.FundamentalFactSource("sec"), Subject: "cik:0000000001", Taxonomy: "us-gaap",
		Concept: concept, Unit: unit, Currency: "USD", PeriodEnd: sd(t, end), Filed: sd(t, filed),
		Form: form, DocID: "acc-" + filed + "-" + form, Value: value,
	}
	if start != "" {
		s := sd(t, start)
		f.PeriodStart = &s
	}
	return f
}

// annual returns the four figures of fiscal year y (1 January to 31 December), filed
// the next February on a 10-K.
func annual(t *testing.T, y int) []contracts.FundamentalFact {
	t.Helper()
	ys := strconv.Itoa(y)
	filed := strconv.Itoa(y+1) + "-02-15"
	return []contracts.FundamentalFact{
		mk(t, ocf, "USD", ys+"-01-01", ys+"-12-31", filed, "10-K", "1000"),
		mk(t, cap, "USD", ys+"-01-01", ys+"-12-31", filed, "10-K", "200"),
		mk(t, sbc, "USD", ys+"-01-01", ys+"-12-31", filed, "10-K", "50"),
		mk(t, dil, "shares", ys+"-01-01", ys+"-12-31", filed, "10-K", "100000000"),
	}
}

func sic(t *testing.T, code string) contracts.FundamentalFact {
	f := mk(t, "StandardIndustrialClassification", "sic", "", "2024-06-01", "2024-06-01", "submissions", code)
	f.Taxonomy = "sec"
	return f
}

// company has fiscal years from..to, skipping any year in skip, and the given SIC.
func company(t *testing.T, from, to int, code string, skip ...int) []contracts.FundamentalFact {
	t.Helper()
	var facts []contracts.FundamentalFact
	for y := from; y <= to; y++ {
		omit := false
		for _, s := range skip {
			omit = omit || s == y
		}
		if !omit {
			facts = append(facts, annual(t, y)...)
		}
	}
	if code != "" {
		facts = append(facts, sic(t, code))
	}
	return facts
}

func TestAsOfLatestFiledWins(t *testing.T) {
	t.Parallel()
	facts := company(t, 2019, 2023, "3571")
	facts = append(facts,
		mk(t, ocf, "USD", "2023-01-01", "2023-12-31", "2024-06-10", "10-K/A", "1100"),
		mk(t, ocf, "USD", "2023-01-01", "2023-12-31", "2025-01-10", "10-K/A", "9999"),
	)
	got := BuildFilings(facts, "2024-12-31", valuationapi.USGAAP)
	mustEq(t, valuationapi.StateOK, got.State, got.Reasons)
	eq(t, "1100", got.Filings.Years[0].Values["operating_cash_flow"])
	eq(t, contracts.Day("2023-12-31"), got.Filings.Years[0].FYEnd)
	eq(t, contracts.Day("2024-06-10"), got.Filings.Years[0].Filed)

	before := BuildFilings(facts, "2024-03-01", valuationapi.USGAAP)
	eq(t, "1000", before.Filings.Years[0].Values["operating_cash_flow"])
	eq(t, contracts.Day("2024-02-15"), before.Filings.Years[0].Filed)

	// the same latest filed day with two values is a conflict, not a pick
	facts = append(facts, mk(t, ocf, "USD", "2023-01-01", "2023-12-31", "2024-06-10", "10-K/A", "1111"))
	conflict := BuildFilings(facts, "2024-12-31", valuationapi.USGAAP)
	eq(t, valuationapi.StateMissingInput, conflict.State)
	contains(t, conflict.Reasons, "input_conflict:operating_cash_flow:2023-12-31")
}

func TestRecastKeepsYearEligible(t *testing.T) {
	t.Parallel()
	facts := company(t, 2019, 2023, "3571")
	// an 8-K recasts 2019 later: its value replaces the 10-K value, but the year stays,
	// so five years remain and the oldest one is not dropped.
	facts = append(facts, mk(t, ocf, "USD", "2019-01-01", "2019-12-31", "2023-09-01", "8-K", "777"))
	got := BuildFilings(facts, "2024-12-31", valuationapi.USGAAP)
	mustEq(t, valuationapi.StateOK, got.State, got.Reasons)
	mustLen(t, got.Filings.Years, 5)
	oldest := got.Filings.Years[4]
	eq(t, contracts.Day("2019-12-31"), oldest.FYEnd)
	eq(t, "777", oldest.Values["operating_cash_flow"])
	eq(t, "200", oldest.Values["capital_spending"])

	// an 8-K alone does not make a year
	only8K := []contracts.FundamentalFact{mk(t, ocf, "USD", "2018-01-01", "2018-12-31", "2023-09-01", "8-K", "5")}
	got = BuildFilings(append(company(t, 2019, 2023, "3571"), only8K...), "2024-12-31", valuationapi.USGAAP)
	mustLen(t, got.Filings.Years, 5)
	eq(t, contracts.Day("2019-12-31"), got.Filings.Years[4].FYEnd)
}

func TestSicRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, code, state, sicState, reason string
	}{
		{"industrial", "3571", valuationapi.StateOK, "ok", ""},
		{"short code padded", "100", valuationapi.StateOK, "ok", ""},
		{"bank", "6021", valuationapi.StateUnvalued, "ok", "financial_sic"},
		{"oil", "2911", valuationapi.StateUnvalued, "ok", "cyclical_sic"},
		{"none", "", valuationapi.StateMissingInput, "missing", "no_sic"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := BuildFilings(company(t, 2019, 2023, c.code), "2024-12-31", valuationapi.USGAAP)
			eq(t, c.state, got.State)
			eq(t, c.sicState, got.SICState)
			if c.reason == "" {
				empty(t, got.Reasons)
			} else {
				eq(t, []string{c.reason}, got.Reasons)
			}
			if c.sicState == "ok" {
				eq(t, got.SIC, got.Filings.SIC)
				if c.code == "100" {
					eq(t, "0100", got.SIC)
				}
			}
		})
	}
	got := BuildFilings(company(t, 2019, 2023, "6021"), "2024-12-31", valuationapi.USGAAP)
	eq(t, "6021", got.SIC)
	hasLen(t, got.Filings.Years, 5, "an unvalued company still has its years for the diagnosis")
}

func TestParamsForUsOnly(t *testing.T) {
	t.Parallel()
	p, ok := ParamsFor(valuationapi.USGAAP)
	mustTrue(t, ok)
	eq(t, valuationapi.USGAAP, p.Standard)
	eq(t, "USD", p.Currency)
	eq(t, 10, p.Years)
	for _, std := range []valuationapi.Standard{valuationapi.JGAAP, valuationapi.IFRS, ""} {
		_, ok := ParamsFor(std)
		isFalse(t, ok, string(std))
	}
	got := BuildFilings(company(t, 2019, 2023, "3571"), "2024-12-31", valuationapi.IFRS)
	eq(t, valuationapi.StateMissingInput, got.State)
	eq(t, []string{"unsupported_standard"}, got.Reasons)
}

func TestStandardOfFromFacts(t *testing.T) {
	t.Parallel()
	us := company(t, 2023, 2023, "3571")
	eq(t, valuationapi.USGAAP, StandardOf(us))

	jp := func(unit string) contracts.FundamentalFact {
		f := mk(t, "AccountingStandard", unit, "", "2024-03-31", "2024-06-25", "annual", "1")
		f.Source, f.Taxonomy, f.Currency = gen.FundamentalFactSource("edinet"), "dei", "JPY"
		return f
	}
	eq(t, valuationapi.IFRS, StandardOf([]contracts.FundamentalFact{jp("IFRS")}))
	eq(t, valuationapi.JGAAP, StandardOf([]contracts.FundamentalFact{jp("J_GAAP")}))
	eq(t, valuationapi.Standard(""), StandardOf([]contracts.FundamentalFact{jp("US_GAAP_X")}))
	eq(t, valuationapi.Standard(""), StandardOf(nil))

	got := BuildFilings(nil, "2024-12-31", StandardOf(nil))
	eq(t, valuationapi.StateMissingInput, got.State)
	eq(t, []string{"no_standard"}, got.Reasons)
}

func eq(t *testing.T, want, got any, msg ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v %v", want, got, msg)
	}
}

func mustEq(t *testing.T, want, got any, msg ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %v, got %v %v", want, got, msg)
	}
}

func contains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, s := range list {
		if s == want {
			return
		}
	}
	t.Errorf("%v does not contain %q", list, want)
}

func hasLen[T any](t *testing.T, list []T, n int, msg ...any) {
	t.Helper()
	if len(list) != n {
		t.Errorf("len %d, want %d %v", len(list), n, msg)
	}
}

func mustLen[T any](t *testing.T, list []T, n int) {
	t.Helper()
	if len(list) != n {
		t.Fatalf("len %d, want %d", len(list), n)
	}
}

func empty[T any](t *testing.T, list []T) {
	t.Helper()
	if len(list) != 0 {
		t.Errorf("want empty, got %v", list)
	}
}

func isTrue(t *testing.T, b bool, msg ...any) {
	t.Helper()
	if !b {
		t.Errorf("want true %v", msg)
	}
}

func isFalse(t *testing.T, b bool, msg ...any) {
	t.Helper()
	if b {
		t.Errorf("want false %v", msg)
	}
}

func mustTrue(t *testing.T, b bool, msg ...any) {
	t.Helper()
	if !b {
		t.Fatalf("want true %v", msg)
	}
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
