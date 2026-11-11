package option_candidates_page

import (
	"github.com/andrewrong/optdesk/internal/store"
	"reflect"
	"testing"
)

func TestReasonsFromGates(t *testing.T) {
	t.Parallel()
	gates, err := parseGates(`[{"gate":"buy_price","status":"pass","reason":""},` +
		`{"gate":"events","status":"block","reason":"earnings_before_expiry"},` +
		`{"gate":"chain_fresh","status":"unknown","reason":"stale_chain"}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := []reason{
		{"events", "block", "earnings_before_expiry", "Earnings date is before expiry"},
		{"chain_fresh", "unknown", "stale_chain", "Option chain is not from the latest trading day"},
	}
	if got := reasonsOf(gates); !reflect.DeepEqual(got, want) {
		t.Errorf("reasons = %+v", got)
	}
	// every code of option-candidates.md Decisions 7, 11 and 12 has a sentence
	for _, code := range []string{"no_buy_price", "earnings_before_expiry", "yield_below_cash", "stale_chain", "no_fx",
		"no_free_shares", "strike_floor", "early_assignment_risk"} {
		if s := reasonText(code); s == code || s == "" {
			t.Errorf("code %s has no sentence", code)
		}
	}
}

func TestUnknownCodeShownAsCode(t *testing.T) {
	t.Parallel()
	gates, err := parseGates(`[{"gate":"new_gate","status":"block","reason":"brand_new_code"}]`)
	if err != nil {
		t.Fatal(err)
	}
	rs := reasonsOf(gates)
	if len(rs) != 1 || rs[0].Text != "brand_new_code" || rs[0].Code != "brand_new_code" {
		t.Errorf("reasons = %+v", rs)
	}
}

func TestBadGatesJSONKeepsRow(t *testing.T) {
	t.Parallel()
	bad := cand("short_put", 7, "blocked")
	bad.Gates = "not json"
	badMarks := cand("short_put", 8, "blocked")
	badMarks.Marks = "{"
	g := buildGroup(groupSpecs[0], "2026-11-09", "2026-11-09", []store.Candidate{bad, badMarks})
	if g.Other == nil || len(g.Other.Rows) != 2 {
		t.Fatalf("rows dropped: %+v", g.Other)
	}
	r := g.Other.Rows[0]
	if c, ok := r.Cells["reasons"].(gap); !ok || c.Reason != "unreadable" {
		t.Errorf("reasons cell = %+v", r.Cells["reasons"])
	}
	if len(r.Reasons) != 0 || r.Cells["company"].(valueCell).Text != "ACME" {
		t.Errorf("row = %+v", r)
	}
	for _, g := range []string{`[{"gate":"Bad","status":"block","reason":"x"}]`, `[{"gate":"a","status":"maybe","reason":"x"}]`,
		`[{"gate":"a","status":"block","reason":""}]`, "null"} {
		if _, err := parseGates(g); err == nil {
			t.Errorf("parseGates(%s) has no error", g)
		}
	}
}
