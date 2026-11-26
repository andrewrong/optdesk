package earningsrefresh

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// vendorSuffix spells every symbol with ".T": the vendor symbol of a JP code differs from the code.
type vendorSuffix struct{}

func (vendorSuffix) ToVendor(s string, _ symbol.Vendor) (string, error) { return s + ".T", nil }
func (vendorSuffix) OSIRoot(s string) string                            { return s }

// fbMaker records the calls of the maker and hands out one fake source per market.
type fbMaker struct {
	srcs  map[string]*fakeSource
	calls map[string][]string
}

func newFB(srcs map[string]*fakeSource) *fbMaker {
	return &fbMaker{srcs: srcs, calls: map[string][]string{}}
}

func (m *fbMaker) make(market string, symbols []string) (sources.EarningsSource, error) {
	m.calls[market] = symbols
	return m.srcs[market], nil
}

func runFB(t *testing.T, primary map[string]sources.EarningsSource, mk FallbackMaker, st *memStore, entries ...jobs.WatchEntry) contracts.JobResult {
	t.Helper()
	j, err := NewWithSymbols(primary, st, st, entries, fixedClock{}, vendorSuffix{})
	if err != nil {
		t.Fatal(err)
	}
	return WithFallback(j, mk).Run(context.Background(), day, nil)
}

func TestFallbackFillsAbsent(t *testing.T) {
	t.Parallel()
	fb := newFB(map[string]*fakeSource{"US": {listings: []sources.EarningsListing{
		{Symbol: "AAPL", ReportDate: "2026-11-01"},
		{Symbol: "AAPL", ReportDate: "2026-12-20"},
		{Symbol: "AAPL", ReportDate: "2026-11-12", TimeOfDay: "pre-market", Estimate: "1", FiscalPeriodEnd: "2026-09-30"},
	}}})
	st := &memStore{}
	res := runFB(t, map[string]sources.EarningsSource{"US": &fakeSource{}}, fb.make, st, us("cik:1", "AAPL"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	r := byIssuer(st, "cik:1")
	if len(r) != 1 || r[0].Status != "listed" || r[0].Source != "yfinance" || r[0].ReportDate.Format(dayLayout) != "2026-11-12" ||
		r[0].TimeOfDay != "" || r[0].FiscalPeriodEnd != nil || r[0].Estimate != nil || r[0].ErrorCode != "" {
		t.Fatalf("row %+v", r)
	}
	if got := fb.calls["US"]; len(got) != 1 || got[0] != "AAPL" {
		t.Fatalf("asked %v", got)
	}
}

func TestFallbackFillsErrorRow(t *testing.T) {
	t.Parallel()
	fb := newFB(map[string]*fakeSource{"US": {listings: []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-11-12"}}}})
	st := &memStore{}
	res := runFB(t, map[string]sources.EarningsSource{"US": &fakeSource{err: codedErr{"rate_limited"}}}, fb.make, st, us("cik:1", "AAPL"))
	if res.Err == nil {
		t.Fatal("primary failure must still fail the run")
	}
	r := byIssuer(st, "cik:1")
	if len(r) != 1 || r[0].Status != "listed" || r[0].Source != "yfinance" || r[0].ErrorCode != "" {
		t.Fatalf("row %+v", r)
	}
}

func TestFallbackFillsUnsupportedJP(t *testing.T) {
	t.Parallel()
	fb := newFB(map[string]*fakeSource{"JP": {listings: []sources.EarningsListing{{Symbol: "7203", ReportDate: "2026-11-09"}}}})
	st := &memStore{}
	res := runFB(t, map[string]sources.EarningsSource{}, fb.make, st, jp("edinet:E1", "7203"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	r := byIssuer(st, "edinet:E1")
	if len(r) != 1 || r[0].Status != "listed" || r[0].Source != "yfinance" || r[0].Market != "JP" ||
		r[0].ReportDate.Format(dayLayout) != "2026-11-09" {
		t.Fatalf("row %+v", r)
	}
}

func TestFallbackNeverReplacesListed(t *testing.T) {
	t.Parallel()
	fb := newFB(map[string]*fakeSource{"US": {listings: []sources.EarningsListing{
		{Symbol: "AAPL", ReportDate: "2026-11-20"}, {Symbol: "NVDA", ReportDate: "2026-11-21"}}}})
	prim := &fakeSource{listings: []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-11-12"}}}
	st := &memStore{}
	runFB(t, map[string]sources.EarningsSource{"US": prim}, fb.make, st, us("cik:1", "AAPL"), us("cik:2", "NVDA"))
	a := byIssuer(st, "cik:1")[0]
	if a.Source != "alphavantage" || a.ReportDate.Format(dayLayout) != "2026-11-12" {
		t.Fatalf("listed row changed: %+v", a)
	}
	if got := fb.calls["US"]; len(got) != 1 || got[0] != "NVDA" {
		t.Fatalf("asked %v", got)
	}
	if n := byIssuer(st, "cik:2")[0]; n.Source != "yfinance" || n.Status != "listed" {
		t.Fatalf("row %+v", n)
	}
}

func TestFallbackKeepsOwnerRow(t *testing.T) {
	t.Parallel()
	fb := newFB(map[string]*fakeSource{"US": {listings: []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-11-20"}}}})
	e := us("cik:1", "AAPL")
	e.ReportDateOverride = "2026-11-15"
	st := &memStore{}
	runFB(t, map[string]sources.EarningsSource{"US": &fakeSource{}}, fb.make, st, e)
	r := byIssuer(st, "cik:1")[0]
	if r.Source != "owner" || r.ReportDate.Format(dayLayout) != "2026-11-15" {
		t.Fatalf("owner row changed: %+v", r)
	}
	if len(fb.calls) != 0 {
		t.Fatalf("fallback asked for an owner entry: %v", fb.calls)
	}
}

func TestFallbackNoDateLeavesAbsent(t *testing.T) {
	t.Parallel()
	fb := newFB(map[string]*fakeSource{"US": {listings: []sources.EarningsListing{
		{Symbol: "AAPL", ReportDate: "2026-11-01"}, {Symbol: "MSFT", ReportDate: "2026-11-20"}}}})
	st := &memStore{}
	runFB(t, map[string]sources.EarningsSource{"US": &fakeSource{}}, fb.make, st, us("cik:1", "AAPL"))
	r := byIssuer(st, "cik:1")[0]
	if r.Status != "absent" || r.Source != "alphavantage" || r.ReportDate != nil {
		t.Fatalf("row %+v", r)
	}
}

func TestFallbackErrorKeepsPrimaryRowsAndResult(t *testing.T) {
	t.Parallel()
	run := func(mk FallbackMaker) (contracts.JobResult, []contracts.EarningsDate) {
		st := &memStore{}
		prim := map[string]sources.EarningsSource{"US": &fakeSource{err: codedErr{"network"}}}
		res := runFB(t, prim, mk, st, us("cik:1", "AAPL"), jp("edinet:E1", "7203"))
		return res, st.rows
	}
	fb := newFB(map[string]*fakeSource{
		"US": {err: codedErr{"rate_limited"}}, "JP": {err: codedErr{"timeout"}}})
	got, rows := run(fb.make)
	want, wantRows := run(nil)
	if got.Returned != want.Returned || got.Usable != want.Usable || (got.Err == nil) != (want.Err == nil) ||
		(got.Err != nil && !strings.Contains(got.Err.Error(), "network")) {
		t.Fatalf("result %+v want %+v", got, want)
	}
	for i := range rows {
		if rows[i].Status != wantRows[i].Status || rows[i].Source != wantRows[i].Source || rows[i].ErrorCode != wantRows[i].ErrorCode {
			t.Fatalf("row %+v want %+v", rows[i], wantRows[i])
		}
	}
	if len(rows) != 2 || rows[0].Status != "unsupported" || rows[1].Status != "error" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestFallbackOneFetchPerMarket(t *testing.T) {
	t.Parallel()
	usSrc, jpSrc := &fakeSource{}, &fakeSource{}
	fb := newFB(map[string]*fakeSource{"US": usSrc, "JP": jpSrc})
	st := &memStore{}
	runFB(t, map[string]sources.EarningsSource{"US": &fakeSource{}}, fb.make, st,
		us("cik:1", "AAPL"), us("cik:2", "NVDA"), jp("edinet:E1", "7203"), jp("edinet:E2", "6758"))
	if usSrc.calls != 1 || jpSrc.calls != 1 {
		t.Fatalf("fetches US %d JP %d", usSrc.calls, jpSrc.calls)
	}
	if len(fb.calls["US"]) != 2 || len(fb.calls["JP"]) != 2 {
		t.Fatalf("asked %v", fb.calls)
	}
}

func TestWithFallbackForeignJobUnchanged(t *testing.T) {
	t.Parallel()
	var other contracts.Job
	if got := WithFallback(other, newFB(nil).make); got != other {
		t.Fatalf("got %v", got)
	}
}
