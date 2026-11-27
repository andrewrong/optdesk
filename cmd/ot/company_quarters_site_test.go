package main

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/companydata"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/pepb"
	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestCompanyQuartersRouteStatus(t *testing.T) {
	t.Parallel()
	var subjects []string
	rd := companySiteReaders(&subjects)
	code, body := companySiteGet(t, rd, http.MethodGet, "/company-quarters")
	if code != http.StatusOK || !strings.Contains(body, `"schema":"company_quarters_page"`) {
		t.Fatalf("GET: %d %s", code, body)
	}
	if code, _ := companySiteGet(t, rd, http.MethodHead, "/company-quarters"); code != http.StatusOK {
		t.Errorf("HEAD: %d", code)
	}
	if code, _ := companySiteGet(t, rd, http.MethodPost, "/company-quarters"); code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", code)
	}
	if code, _ := companySiteGet(t, rd, http.MethodGet, "/company-quarters/x"); code != http.StatusNotFound {
		t.Errorf("below the route: %d", code)
	}
	code, body = companySiteGet(t, siteReaders{}, http.MethodGet, "/company-quarters")
	if code != http.StatusServiceUnavailable || strings.TrimSpace(body) != "unavailable" {
		t.Errorf("build error: %d %q", code, body)
	}
}

func TestCompanyQuartersQueryKey(t *testing.T) {
	t.Parallel()
	var subjects []string
	code, body := companySiteGet(t, companySiteReaders(&subjects), http.MethodGet, "/company-quarters?c=US:AAPL")
	if code != http.StatusOK || !strings.Contains(body, `"company":{"key":"US:AAPL"`) {
		t.Fatalf("%d %s", code, body)
	}
	if !slices.Equal(subjects, []string{"cik:0000320193"}) {
		t.Fatalf("facts read for %v", subjects)
	}
}

func sdate(s string) types.SerializableDate {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: d}
}

// agreeFact is one made-up USD fact: form for pepb, fy and fp for the quarters.
func agreeFact(form, doc, filed, start, end, val string, fp string) contracts.FundamentalFact {
	s, fy := sdate(start), 2025
	return contracts.FundamentalFact{
		Taxonomy: "us-gaap", Concept: "NetIncomeLoss", Unit: "USD", Form: form, DocID: doc, Filed: sdate(filed),
		PeriodStart: &s, PeriodEnd: sdate(end), Value: val, Source: "sec", Subject: "cik:0000000001", Fy: &fy, Fp: fp,
	}
}

func TestPepbAndQuartersAgree(t *testing.T) {
	t.Parallel()
	cover := contracts.FundamentalFact{
		Taxonomy: "dei", Concept: "EntityCommonStockSharesOutstanding", Unit: "shares", Form: "10-K", DocID: "k",
		Filed: sdate("2026-02-15"), PeriodEnd: sdate("2026-02-15"), Value: "100", Source: "sec", Subject: "cik:0000000001",
	}
	annual := []contracts.FundamentalFact{
		agreeFact("10-K", "k", "2026-02-15", "2025-01-01", "2025-12-31", "1000", "FY"),
		// the annual value restated by a later filing
		agreeFact("10-K/A", "ka", "2026-04-01", "2025-01-01", "2025-12-31", "1200", "FY"),
		// the restating filing repeats the nine months, so the fourth quarter is taken on one basis
		agreeFact("10-K/A", "ka", "2026-04-01", "2025-01-01", "2025-09-30", "700", "Q3"),
	}
	quarterly := []contracts.FundamentalFact{
		agreeFact("10-Q", "q1", "2025-05-01", "2025-01-01", "2025-03-31", "100", "Q1"),
		agreeFact("10-Q", "q2", "2025-08-01", "2025-01-01", "2025-06-30", "250", "Q2"),
		agreeFact("10-Q", "q3", "2025-11-01", "2025-01-01", "2025-09-30", "700", "Q3"),
	}
	all := append(append([]contracts.FundamentalFact{cover}, annual...), quarterly...)

	// pepb: price 120 on 100 shares is a market value of 12000; the P/E gives the trailing net income back.
	bars := make([]contracts.PriceBar, 5)
	for i := range bars {
		bars[i] = contracts.PriceBar{Close: "120", Currency: "USD", SessionDay: sdate("2026-04-20")}
		bars[i].SessionDay.Time = bars[i].SessionDay.AddDate(0, 0, i)
	}
	ttmOf := func(facts []contracts.FundamentalFact) decimal.Decimal {
		t.Helper()
		r := pepb.New(gonumstats.New())(pepbapi.Input{Facts: facts, Bars: bars, Today: "2026-04-24"}).PE
		if r.State != pepbapi.StateOK {
			t.Fatalf("pepb state %s %s", r.State, r.Reason)
		}
		ratio := decimal.RequireFromString(r.Ratio)
		return decimal.RequireFromString(r.Shares).Mul(decimal.NewFromInt(120)).Div(ratio)
	}
	fy := ttmOf([]contracts.FundamentalFact{cover, annual[0], annual[1]})

	qs, err := companydata.Quarters(all)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 4 {
		t.Fatalf("%d quarters", len(qs))
	}
	sum := decimal.Zero
	for _, q := range qs {
		if q.NetIncome.Missing {
			t.Fatalf("quarter %d has no net income: %+v", q.Number, q.NetIncome)
		}
		sum = sum.Add(decimal.RequireFromString(q.NetIncome.Value))
	}
	if !fy.Equal(decimal.NewFromInt(1200)) || !sum.Equal(fy) {
		t.Fatalf("pepb FY net income %s, sum of the four quarters %s, want 1200 both", fy, sum)
	}
	if ttm := ttmOf(all); !ttm.Equal(sum) {
		t.Fatalf("pepb trailing net income %s, quarters %s", ttm, sum)
	}
}
