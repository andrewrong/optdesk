package optioncandidates

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestAliasesAandBAreSeparate(t *testing.T) {
	t.Parallel()
	f := newFakes()
	st := f.stores()
	st.Aliases = []string{"A", "B"}
	f.values = map[string][]store.AccountValue{
		"A": {{Tag: "AvailableFunds", Value: "5000", Currency: "USD"}},
		"B": {{Tag: "AvailableFunds", Value: "20000", Currency: "USD"}},
	}
	f.positions = map[string][]store.Position{
		"A": {{Symbol: "ABC", SecType: "stock", Quantity: "300", AvgCost: "88"}},
		"B": {{Symbol: "ABC", SecType: "option", Right: "call", Quantity: "-2"}},
	}
	build(t, st, f, usEntry()).Run(context.Background(), runDay, nil)
	got := f.inputs[0].Accounts
	if len(got) != 2 || got[0].Alias != "A" || got[0].Funds != "5000" || got[0].Shares != 300 || got[0].AvgCost != "88" ||
		got[1].Alias != "B" || got[1].Funds != "20000" || got[1].Shares != 0 || got[1].OpenShortCalls != 2 ||
		got[0].CaptureDay != "2026-11-09" {
		t.Fatalf("accounts %+v", got)
	}
}

func TestAccountsReadByGivenAliases(t *testing.T) {
	t.Parallel()
	f := newFakes()
	st := f.stores()
	st.Aliases = []string{"mine_account", "wife_account"}
	f.values = map[string][]store.AccountValue{
		"mine_account": {{Tag: "AvailableFunds", Value: "5000", Currency: "USD"}},
		"wife_account": {{Tag: "AvailableFunds", Value: "20000", Currency: "USD"}},
	}
	f.positions = map[string][]store.Position{}
	build(t, st, f, usEntry()).Run(context.Background(), runDay, nil)
	got := f.inputs[0].Accounts
	if len(got) != 2 || got[0].Alias != "mine_account" || got[0].Funds != "5000" ||
		got[1].Alias != "wife_account" || got[1].Funds != "20000" {
		t.Fatalf("accounts %+v", got)
	}
}

func TestNilAliasesReadNoAccount(t *testing.T) {
	t.Parallel()
	f := newFakes()
	st := f.stores()
	st.Aliases = nil
	f.values = map[string][]store.AccountValue{
		"A": {{Tag: "AvailableFunds", Value: "5000", Currency: "USD"}},
		"B": {{Tag: "AvailableFunds", Value: "20000", Currency: "USD"}},
	}
	f.positions = map[string][]store.Position{}
	build(t, st, f, usEntry()).Run(context.Background(), runDay, nil)
	if got := f.inputs[0].Accounts; len(got) != 0 {
		t.Fatalf("accounts %+v", got)
	}
}

func TestTermsChunkedAt500(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.eodRows = nil
	for i := 1; i <= 1001; i++ {
		f.eodRows = append(f.eodRows, contracts.OptionEod{InstrumentID: i, Status: 1, CaptureID: "c"})
	}
	build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil)
	if len(f.termIDs) != 3 || len(f.termIDs[0]) != 500 || len(f.termIDs[1]) != 500 || len(f.termIDs[2]) != 1 {
		t.Fatalf("chunks %d", len(f.termIDs))
	}
}

func TestStaleChainDay(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.chainDay = "2026-11-06"
	if res := build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	in := f.inputs[0]
	if in.ChainDay != "2026-11-06" || in.RunDay != runDay {
		t.Fatalf("chain %q run %q", in.ChainDay, in.RunDay)
	}
}

func TestSessionDaysSupplied(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.chainDay = "2026-11-06"
	f.earnings = map[string][]contracts.EarningsDate{"cik:0000000001": {
		{Issuer: "cik:0000000001", Status: "listed", ObservedOn: day("2026-11-05"), ReportDate: dayPtr("2026-12-20")},
	}}
	build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil)
	in := f.inputs[0]
	want := []contracts.Day{"2026-11-05", "2026-11-06", "2026-11-09"}
	if len(in.SessionDays) != 3 || in.SessionDays[0] != want[0] || in.SessionDays[2] != want[2] {
		t.Fatalf("session days %v", in.SessionDays)
	}
	e := in.Earnings
	if e.State != "listed" || e.Next != "2026-12-20" || e.RowDay != "2026-11-05" {
		t.Fatalf("earnings %+v", e)
	}
}

func TestEarningsFutureRowSurvivesAbsentCheck(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.chainDay = "2026-11-06"
	f.earnings = map[string][]contracts.EarningsDate{"cik:0000000001": {
		{Issuer: "cik:0000000001", Status: "listed", ObservedOn: day("2026-11-01"), ReportDate: dayPtr("2026-12-20")},
		{Issuer: "cik:0000000001", Status: "absent", ObservedOn: day("2026-11-05")},
	}}
	build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil)
	e := f.inputs[0].Earnings
	if e.State != "listed" || e.Next != "2026-12-20" || e.RowDay != "2026-11-05" {
		t.Fatalf("earnings %+v", e)
	}

	g := newFakes()
	g.chainDay = "2026-11-06"
	g.earnings = map[string][]contracts.EarningsDate{"cik:0000000001": {
		{Issuer: "cik:0000000001", Status: "listed", ObservedOn: day("2026-11-01"), ReportDate: dayPtr("2026-12-20")},
		{Issuer: "cik:0000000001", Status: "listed", ObservedOn: day("2026-11-05"), ReportDate: dayPtr("2026-12-10")},
	}}
	build(t, g.stores(), g, usEntry()).Run(context.Background(), runDay, nil)
	if e := g.inputs[0].Earnings; e.State != "listed" || e.Next != "2026-12-10" {
		t.Fatalf("earnings %+v", e)
	}
}

func TestRateIsPercentText(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.rate = f64(4.2)
	build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil)
	if in := f.inputs[0]; in.Rate != "4.2" || in.RateDay != "2026-11-06" {
		t.Fatalf("rate %q day %q", in.Rate, in.RateDay)
	}
}

func dayPtr(s string) *types.SerializableDate {
	d := day(s)
	return &d
}

func TestChainDayNotAfterRunDay(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.chainDay = "2026-11-10" // the catalog's newest chain is after the run day
	build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil)
	for _, in := range f.inputs {
		if in.ChainDay > in.RunDay {
			t.Fatalf("evaluated chain %q for run day %q", in.ChainDay, in.RunDay)
		}
	}
}

func TestRerunUsesNewestChainOnOrBeforeRunDay(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.chainDay = "2026-11-06"
	f.catalogDay = "2026-11-10" // a later capture exists
	build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil)
	if len(f.inputs) != 1 || f.inputs[0].ChainDay != "2026-11-06" {
		t.Fatalf("inputs %+v", f.inputs)
	}
}

func TestTrimSessionDaysKeepsRunDay(t *testing.T) {
	t.Parallel()
	all := []contracts.Day{"2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09"}
	got := trimSessionDays(all, "2026-11-06")
	want := []contracts.Day{"2026-11-06", "2026-11-09"}
	if len(got) != len(want) || got[0] != want[0] || got[len(got)-1] != "2026-11-09" {
		t.Fatalf("trimmed %v", got)
	}
}

type keepRow struct {
	Status     string `json:"status"`
	ObservedOn string `json:"observed_on"`
	ReportDate string `json:"report_date"`
}

type keepCase struct {
	Override  string    `json:"override"`
	Rows      []keepRow `json:"rows"`
	WantState string    `json:"want_state"`
	WantNext  string    `json:"want_next"`
}

// runKeepCase reads one case of testdata/earnings_keep.json through earnings() and returns the rule input and
// the chain-side result.
func runKeepCase(t *testing.T, name string) (keepCase, *compute.EarningsInfo, chainEarnings) {
	t.Helper()
	b, err := os.ReadFile("testdata/earnings_keep.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		ChainDay string              `json:"chain_day"`
		Issuer   string              `json:"issuer"`
		Cases    map[string]keepCase `json:"cases"`
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	c, ok := fx.Cases[name]
	if !ok {
		t.Fatalf("no case %q", name)
	}
	var rows []contracts.EarningsDate
	for _, r := range c.Rows {
		row := contracts.EarningsDate{Issuer: fx.Issuer, Status: gen.EarningsDateStatus(r.Status), ObservedOn: day(r.ObservedOn)}
		if r.ReportDate != "" {
			row.ReportDate = dayPtr(r.ReportDate)
		}
		rows = append(rows, row)
	}
	f := newFakes()
	f.earnings = map[string][]contracts.EarningsDate{fx.Issuer: rows}
	e := usEntry()
	e.Issuer = fx.Issuer
	e.ReportDateOverride = c.Override
	j := &job{st: f.stores()}
	info, ce, err := j.earnings(context.Background(), e, contracts.Day(fx.ChainDay))
	if err != nil {
		t.Fatal(err)
	}
	return c, info, ce
}

func TestKeepLastKnownDate(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"keep_last_known", "keep_after_error"} {
		c, info, ce := runKeepCase(t, name)
		if string(info.Next) != c.WantNext || info.State != c.WantState || string(ce.next) != c.WantNext {
			t.Fatalf("%s: info %+v chain %+v", name, info, ce)
		}
	}
}

func TestKeepIgnoresPastDate(t *testing.T) {
	t.Parallel()
	c, info, ce := runKeepCase(t, "past_date_ignored")
	if info.Next != "" || info.State != c.WantState || ce.next != "" {
		t.Fatalf("info %+v chain %+v", info, ce)
	}
}

func TestOverrideWins(t *testing.T) {
	t.Parallel()
	c, info, ce := runKeepCase(t, "override")
	if string(info.Override) != c.WantNext || string(ce.next) != c.WantNext {
		t.Fatalf("info %+v chain %+v", info, ce)
	}
}

func TestKeepNewestListedNotEarliest(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.earnings = map[string][]contracts.EarningsDate{"cik:0000000001": {
		{Issuer: "cik:0000000001", Status: "listed", ObservedOn: day("2026-11-01"), ReportDate: dayPtr("2026-12-05")},
		{Issuer: "cik:0000000001", Status: "listed", ObservedOn: day("2026-11-05"), ReportDate: dayPtr("2026-12-20")},
	}}
	e := usEntry()
	e.Issuer = "cik:0000000001"
	j := &job{st: f.stores()}
	info, ce, err := j.earnings(context.Background(), e, contracts.Day("2026-11-06"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Next != "2026-12-20" || ce.next != "2026-12-20" {
		t.Fatalf("want newest listed 2026-12-20, got %s / %s", info.Next, ce.next)
	}
}
