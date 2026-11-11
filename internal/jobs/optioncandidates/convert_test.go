package optioncandidates

import (
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestFloatToDecimal(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   float64
		want string
	}{{1.99, "1.99"}, {0.1, "0.1"}, {100.00049, "100.0005"}, {100.00004, "100"}, {1e-7, "0"},
		{0.00005, "0"}, {0.00015, "0.0002"}, {-0.25, "-0.25"}, {-1e-7, "0"}, {12, "12"}} {
		if got := FloatToDecimal(c.in); got != c.want {
			t.Errorf("FloatToDecimal(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRateRatio(t *testing.T) {
	t.Parallel()
	if got, ok := RateRatio("4.20"); !ok || got != "0.042" {
		t.Fatalf("RateRatio(4.20) = %q, %v", got, ok)
	}
	for _, bad := range []string{"", "x", "-1", "101"} {
		if _, ok := RateRatio(bad); ok {
			t.Errorf("RateRatio(%q) accepted", bad)
		}
	}
}

func TestRankRunOrdersAcrossUnderlyings(t *testing.T) {
	t.Parallel()
	y := func(s string) *string { return &s }
	rows := []store.Candidate{
		{Status: "ok", AnnualizedYield: y("0.10"), DTE: 30, Strike: "50", InstrumentID: 1},
		{Status: "blocked", AnnualizedYield: y("0.90"), DTE: 30, Strike: "50", InstrumentID: 2},
		{Status: "ok", AnnualizedYield: y("0.20"), DTE: 30, Strike: "50", InstrumentID: 3},
	}
	if err := rankRun(rows); err != nil {
		t.Fatal(err)
	}
	if *rows[2].Rank != 1 || *rows[0].Rank != 2 || rows[1].Rank != nil {
		t.Fatalf("ranks %v %v %v", rows[0].Rank, rows[1].Rank, rows[2].Rank)
	}
}

func TestRankRunRefusesBadYield(t *testing.T) {
	t.Parallel()
	bad := "abc"
	if err := rankRun([]store.Candidate{{Status: "ok", AnnualizedYield: &bad, Strike: "50"}}); err == nil {
		t.Fatal("a yield that is not a number was ranked")
	}
}

func TestRunIDOf(t *testing.T) {
	t.Parallel()
	ask, rank, ver := "2.05", 1, int64(3)
	base := func() []store.Candidate {
		return []store.Candidate{
			{SessionDay: "2026-11-09", RunID: "x", Kind: "short_put", InstrumentID: 11, Strike: "90", Ask: &ask, Rank: &rank},
			{SessionDay: "2026-11-09", RunID: "x", Kind: "short_put", InstrumentID: 12, Strike: "85"},
		}
	}
	id := runIDOf("2026-11-09", base())
	if len(id) != 36 || id != strings.ToLower(id) {
		t.Fatalf("id %q", id)
	}
	same := base()
	for i := range same {
		same[i].RunID, same[i].ComputedAt, same[i].SettingsVersion = "other", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), &ver
	}
	if got := runIDOf("2026-11-09", same); got != id {
		t.Fatalf("run id, computed_at or settings version changed the id: %q %q", got, id)
	}
	if runIDOf("2026-11-10", base()) == id {
		t.Error("day not in id")
	}
	if runIDOf("2026-11-09", base()[:1]) == id {
		t.Error("row count not in id")
	}
	for name, mod := range map[string]func(*store.Candidate){
		"strike": func(c *store.Candidate) { c.Strike = "91" },
		"ask":    func(c *store.Candidate) { c.Ask = nil },
		"gates":  func(c *store.Candidate) { c.Gates = `{"a":"fail"}` },
		"rank":   func(c *store.Candidate) { c.Rank = nil },
	} {
		rows := base()
		mod(&rows[0])
		if runIDOf("2026-11-09", rows) == id {
			t.Errorf("column %s not in id", name)
		}
	}
	rev := base()
	rev[0], rev[1] = rev[1], rev[0]
	if runIDOf("2026-11-09", rev) == id {
		t.Error("row order not in id")
	}
}
