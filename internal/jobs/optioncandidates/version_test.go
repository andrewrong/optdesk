package optioncandidates

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// ruleDigests is the sha256 of the rows the rule writes for testdata/rule_digest_input.json, by RuleVersion. A
// change of what the put gates or tiers write changes the digest: bump RuleVersion and add a row; the old rows stay.
var ruleDigests = map[string]string{
	"put_v2": "19918056fc6c163d12e735edcfd40d674479065580fa0bba07a0e380836b0eef",
}

type digestInput struct {
	RunDay contracts.Day `json:"run_day"`
	Params struct {
		MinDTE              int    `json:"min_dte"`
		MaxDTE              int    `json:"max_dte"`
		MinBidSize          int    `json:"min_bid_size"`
		MaxSpread           string `json:"max_spread"`
		MinOpenInterest     int    `json:"min_open_interest"`
		DivGuardFactor      string `json:"div_guard_factor"`
		MaxRateAgeDays      int    `json:"max_rate_age_days"`
		MaxEventAgeSessions int    `json:"max_event_age_sessions"`
		MinReportGapDays    int    `json:"min_report_gap_days"`
	} `json:"params"`
	Rate        string          `json:"rate"`
	RateDay     contracts.Day   `json:"rate_day"`
	SessionDays []contracts.Day `json:"session_days"`
	Underlyings []struct {
		Issuer   string        `json:"issuer"`
		Symbol   string        `json:"symbol"`
		ID       int64         `json:"id"`
		ChainDay contracts.Day `json:"chain_day"`
		Spot     string        `json:"spot"`
		BuyPrice string        `json:"buy_price"`
		Earnings *struct {
			State  string        `json:"state"`
			Next   contracts.Day `json:"next"`
			RowDay contracts.Day `json:"row_day"`
		} `json:"earnings"`
		Accounts []struct {
			Alias      string        `json:"alias"`
			Funds      string        `json:"funds"`
			Currency   string        `json:"currency"`
			CaptureDay contracts.Day `json:"capture_day"`
		} `json:"accounts"`
		Quotes []struct {
			ID           string        `json:"id"`
			Expiry       contracts.Day `json:"expiry"`
			Strike       string        `json:"strike"`
			Bid          string        `json:"bid"`
			Ask          string        `json:"ask"`
			BidSize      int           `json:"bid_size"`
			OpenInterest int           `json:"open_interest"`
			Delta        string        `json:"delta"`
			IV           string        `json:"iv"`
			QuoteOK      bool          `json:"quote_ok"`
		} `json:"quotes"`
	} `json:"underlyings"`
}

// digestRows runs the real rule and the row mapping on the fixed input and returns the rows as the job writes them.
func digestRows(t *testing.T) []store.Candidate {
	t.Helper()
	raw, err := os.ReadFile("testdata/rule_digest_input.json")
	if err != nil {
		t.Fatal(err)
	}
	var in digestInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	p := compute.CandidateParams{
		MinDTE: in.Params.MinDTE, MaxDTE: in.Params.MaxDTE, MinBidSize: in.Params.MinBidSize, MaxSpread: in.Params.MaxSpread,
		MinOpenInterest: in.Params.MinOpenInterest, DivGuardFactor: in.Params.DivGuardFactor,
		MaxRateAgeDays: in.Params.MaxRateAgeDays, MaxEventAgeSessions: in.Params.MaxEventAgeSessions,
		MinReportGapDays: in.Params.MinReportGapDays,
	}
	now := time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC)
	rule := optioncand.New()
	var rows []store.Candidate
	for _, u := range in.Underlyings {
		ci := compute.CandidateInput{
			RunDay: in.RunDay, ChainDay: u.ChainDay, SessionDays: in.SessionDays, Underlying: u.Symbol, Spot: u.Spot,
			BuyPrice: u.BuyPrice, Rate: in.Rate, RateDay: in.RateDay, Kinds: []compute.CandidateKind{compute.KindShortPut},
		}
		ev := chainEarnings{state: "unknown"}
		if u.Earnings != nil {
			ci.Earnings = &compute.EarningsInfo{State: u.Earnings.State, Next: u.Earnings.Next, RowDay: u.Earnings.RowDay}
			ev = chainEarnings{state: u.Earnings.State, next: u.Earnings.Next}
		}
		for _, a := range u.Accounts {
			ci.Accounts = append(ci.Accounts, compute.AliasFunds{Alias: a.Alias, Funds: a.Funds, Currency: a.Currency, CaptureDay: a.CaptureDay})
		}
		for _, q := range u.Quotes {
			ci.Quotes = append(ci.Quotes, compute.OptionQuote{
				InstrumentID: q.ID, Expiry: q.Expiry, Right: "P", Strike: q.Strike, Multiplier: "100", Bid: q.Bid, Ask: q.Ask,
				BidSize: q.BidSize, OpenInterest: q.OpenInterest, Delta: q.Delta, IV: q.IV, MarketDataType: 1, QuoteOK: q.QuoteOK,
			})
		}
		ref := underlyingRef{issuer: u.Issuer, symbol: u.Symbol, id: u.ID, chainDay: u.ChainDay}
		for _, c := range rule.Evaluate(ci, p) {
			row, err := toStoreRow(c, in.RunDay, ref, ev, now)
			if err != nil {
				t.Fatal(err)
			}
			rows = append(rows, row)
		}
	}
	if err := rankRun(rows); err != nil {
		t.Fatal(err)
	}
	setRunID(in.RunDay, rows)
	return rows
}

func TestRuleVersionDigest(t *testing.T) {
	t.Parallel()
	rows := digestRows(t)
	if len(rows) < 8 {
		t.Fatalf("the digest input gives %d rows, want at least 8", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.Status] = true
		if r.RuleVersion != RuleVersion {
			t.Fatalf("row of instrument %d has rule_version %q, want %q", r.InstrumentID, r.RuleVersion, RuleVersion)
		}
	}
	if !seen["ok"] || !seen["blocked"] {
		t.Fatalf("the digest input must give ok and blocked rows, got %v", seen)
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	want, ok := ruleDigests[RuleVersion]
	if !ok {
		t.Fatalf("RuleVersion %q has no digest: bump RuleVersion and add the new digest %s", RuleVersion, got)
	}
	if got != want {
		t.Fatalf("the rule output changed for RuleVersion %q: bump RuleVersion and add the new digest %s (was %s)", RuleVersion, got, want)
	}
}
