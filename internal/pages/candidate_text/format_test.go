package candidate_text

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func okRow(issuer, alias, strike string) store.Candidate {
	y := "0.1234"
	return store.Candidate{
		Issuer: issuer, Kind: "short_put", Alias: alias, Expiry: contracts.Day("2026-12-18"),
		Strike: strike, BidEod: "1.50", BreakEven: "98.50", AnnualizedYield: &y,
		Status: "ok", Gates: `[{"gate":"g1","status":"pass","reason":""}]`, Marks: `["near_earnings","low_iv"]`,
	}
}

func TestFormatOrderKept(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{okRow("Zeta", "", "50"), okRow("Alpha", "main", "100")}
	got := Format(rows, false)
	zi, ai := strings.Index(got, "Zeta"), strings.Index(got, "Alpha")
	if zi < 0 || ai < 0 || zi > ai {
		t.Fatalf("stored order lost:\n%s", got)
	}
	for _, want := range []string{"short_put", "main", "2026-12-18", "100", "1.50", "0.1234", "98.50", "near_earnings,low_iv"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestFormatAllShowsReasons(t *testing.T) {
	t.Parallel()
	b := okRow("Blocky", "", "70")
	b.Status = "blocked"
	b.Gates = `[{"gate":"g1","status":"pass","reason":""},{"gate":"earn","status":"block","reason":"earnings_soon"}]`
	u := okRow("Unk", "", "60")
	u.Status = "unknown"
	u.Gates = `[{"gate":"rate","status":"unknown","reason":"no_rate"}]`
	rows := []store.Candidate{okRow("Fine", "", "80"), b, u}

	short := Format(rows, false)
	if strings.Contains(short, "Blocky") || strings.Contains(short, "Unk") || strings.Contains(short, "earnings_soon") {
		t.Errorf("non-ok rows shown without all:\n%s", short)
	}
	all := Format(rows, true)
	for _, want := range []string{"Fine", "Blocky", "blocked", "earn=earnings_soon", "Unk", "unknown", "rate=no_rate"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "g1=") {
		t.Errorf("a pass gate is listed as a reason:\n%s", all)
	}
}

func TestFormatEmpty(t *testing.T) {
	t.Parallel()
	for _, rows := range [][]store.Candidate{nil, {}} {
		if got := Format(rows, true); got != noRows {
			t.Errorf("Format(empty) = %q, want %q", got, noRows)
		}
	}
	b := okRow("B", "", "1")
	b.Status = "blocked"
	if got := Format([]store.Candidate{b}, false); got != noRows {
		t.Errorf("only blocked without all = %q", got)
	}
}
