package companies_page

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestNilReadersGiveGaps(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Read, cfg.Facts, cfg.Band = nil, nil, nil
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		WatchList struct {
			Rows []struct {
				Cells []testCell `json:"cells"`
			} `json:"rows"`
		} `json:"watch_list"`
		Core      map[string]any `json:"core"`
		Diagnosis map[string]any `json:"diagnosis"`
		Questions map[string]any `json:"questions"`
		Gaps      []struct {
			Reason string `json:"reason"`
		} `json:"gaps"`
		RunStatus string `json:"run_status"`
		DataTimes []struct {
			Source   string `json:"source"`
			DataTime string `json:"data_time"`
		} `json:"data_times"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.RunStatus != "partial" {
		t.Fatalf("run_status = %q, want partial", page.RunStatus)
	}
	if len(page.DataTimes) != 1 || page.DataTimes[0].Source != "companies" || page.DataTimes[0].DataTime != "2026-11-10T00:00:00+08:00" {
		t.Fatalf("data_times = %+v", page.DataTimes)
	}
	if c := page.WatchList.Rows[0].Cells[1]; c.PriceState != "" || c.Text == "" {
		t.Fatalf("cell = %+v", c)
	}
	if len(page.Gaps) != 1 || page.Gaps[0].Reason != "no_reader" {
		t.Fatalf("gaps = %+v", page.Gaps)
	}
	for name, blk := range map[string]map[string]any{"core": page.Core, "diagnosis": page.Diagnosis, "questions": page.Questions} {
		if blk["reason"] != "not_built" {
			t.Fatalf("%s = %v, want a not_built gap", name, blk)
		}
	}
}

func TestHeldRowsCarryTheMoatText(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Held = func(context.Context) (map[string]bool, error) { return map[string]bool{"cik:0000000001": true}, nil }
	cfg.Text = fakeText{"SYN1": "brand and scale"}
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Holdings struct {
			Rows []struct {
				Moat []string `json:"moat"`
			} `json:"rows"`
		} `json:"holdings"`
		WatchList struct {
			Rows []any `json:"rows"`
		} `json:"watch_list"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Holdings.Rows) != 1 || page.Holdings.Rows[0].Moat[0] != "brand and scale" || len(page.WatchList.Rows) != 0 {
		t.Fatalf("page = %s", b)
	}
}

func TestUnvaluedLineStatesEachReasonOnce(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Subjects = []Subject{
		{Issuer: "cik:0000000001", Market: "US", Symbol: "S1"},
		{Issuer: "cik:0000000002", Market: "US", Symbol: "S2"},
		{Issuer: "cik:0000000003", Market: "US", Symbol: "S3"},
	}
	cfg.Facts = fakeFactsByIssuer{}
	cfg.BuildFilings = func(f []contracts.FundamentalFact, _ contracts.Day, _ valuationapi.Standard) valuationapi.FilingsResult {
		reason := "fewer_than_5_years"
		if len(f) == 3 {
			reason = "no_standard"
		}
		return valuationapi.FilingsResult{State: valuationapi.StateUnvalued, Reasons: []string{reason}}
	}
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Unvalued  *string `json:"unvalued"`
		WatchList struct {
			Rows []struct {
				Cells []testCell `json:"cells"`
			} `json:"rows"`
		} `json:"watch_list"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.Unvalued == nil || *page.Unvalued != "fewer than 5 years: S1, S2; no standard: S3" {
		t.Fatalf("unvalued = %v", page.Unvalued)
	}
	for _, r := range page.WatchList.Rows {
		if r.Cells[1].Text != "unvalued" {
			t.Errorf("price cell = %q", r.Cells[1].Text)
		}
	}
}

// fakeFactsByIssuer gives as many facts as the issuer's last digit, so a test can tell issuers apart.
type fakeFactsByIssuer struct{}

func (fakeFactsByIssuer) FactsFiledBy(_ context.Context, _, issuer string, _ contracts.Day) ([]contracts.FundamentalFact, error) {
	return make([]contracts.FundamentalFact, int(issuer[len(issuer)-1]-'0')), nil
}

func TestFullyBuiltPageIsOk(t *testing.T) {
	t.Parallel()
	cfg := fixtureConfig(loadInput(t))
	var subjects []Subject
	for _, s := range cfg.Subjects {
		if s.Symbol != "SYN2" { // the fixture's cyclical company is listed as unvalued
			subjects = append(subjects, s)
		}
	}
	cfg.Subjects = subjects
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		RunStatus string  `json:"run_status"`
		Unvalued  *string `json:"unvalued"`
		Gaps      []any   `json:"gaps"`
		DataTimes []struct {
			Source string `json:"source"`
		} `json:"data_times"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.Unvalued != nil || len(page.Gaps) != 0 {
		t.Fatalf("fixture is not fully built: unvalued=%q gaps=%v", *page.Unvalued, page.Gaps)
	}
	if page.RunStatus != "ok" || len(page.DataTimes) != 1 || page.DataTimes[0].Source != "companies" {
		t.Fatalf("run_status = %q, data_times = %+v", page.RunStatus, page.DataTimes)
	}
}
