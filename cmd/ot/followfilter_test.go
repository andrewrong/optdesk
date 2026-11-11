package main

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

func TestFollowedByFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../contracts/fixtures/underlyings.follow.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Status       string   `json:"status"`
		Options      bool     `json:"options"`
		Fundamentals bool     `json:"fundamentals"`
		IssuerSet    bool     `json:"issuer_set"`
		Held         bool     `json:"held"`
		FollowedBy   []string `json:"followed_by"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 24 {
		t.Fatalf("fixture has %d cases", len(cases))
	}
	for _, c := range cases {
		r := underlying.Row{Status: c.Status, Options: c.Options, Fundamentals: c.Fundamentals}
		if c.IssuerSet {
			r.Issuer = "cik:0000000001"
		}
		got := followedBy(r, c.Held)
		if !slices.Equal(got, c.FollowedBy) && (len(got) != 0 || len(c.FollowedBy) != 0) {
			t.Errorf("%+v: got %v, want %v", c, got, c.FollowedBy)
		}
	}
}

func TestWatchEntryFields(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{underlying.KindStock, underlying.KindETF} {
		r := underlying.Row{Key: "US:AAA", Symbol: "AAA", Market: "US", Kind: kind, Exchange: "SMART", Currency: "USD",
			Issuer: "cik:0000000001", ConID: 42, ReportDateOverride: "2026-12-01", BuyPrice: "10.5", SalePrice: "12"}
		e := watchEntry(r)
		c := e.Contract
		if c.SecType != "STK" || c.ConID != 42 || c.Symbol != "AAA" || c.Exchange != "SMART" || c.Currency != "USD" {
			t.Errorf("%s contract %+v", kind, c)
		}
		if e.Market != "US" || e.ReportDateOverride != "2026-12-01" || e.BuyPrice != "10.5" || e.SalePrice != "12" || e.Issuer != "cik:0000000001" {
			t.Errorf("%s entry %+v", kind, e)
		}
	}
}

func TestWatchEntryEmptyIssuerStaysEmpty(t *testing.T) {
	t.Parallel()
	if e := watchEntry(underlying.Row{Symbol: "FFF", Kind: underlying.KindETF}); e.Issuer != "" {
		t.Fatalf("issuer %q", e.Issuer)
	}
}
