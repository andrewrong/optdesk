package optionopen

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

const (
	weeklyDec11  = "20261211" // a Friday that is not the third
	monthlyDec18 = "20261218" // the third Friday
	weeklyDec25  = "20261225"
)

func gridChain(expiries ...string) broker.Chain {
	return broker.Chain{Exchange: "SMART", TradingClass: "AAPL", Multiplier: "100", Expirations: expiries,
		Strikes: []string{"30", "31", "32", "35", "45"}}
}

func storedAt(expiry contracts.Day, strikes ...string) []openquote.OptionRef {
	var out []openquote.OptionRef
	for _, s := range strikes {
		out = append(out, openquote.OptionRef{Expiry: expiry, Strike: s, Right: "P", TradingClass: "AAPL"})
	}
	return out
}

// gridAsked runs candidates over the chain, for a spot of 40, and returns what is asked by expiry and strike.
func gridAsked(t *testing.T, chain broker.Chain, refs []openquote.OptionRef, pos []store.Position) (map[string]bool, int) {
	t.Helper()
	m, err := parseManifest(resolveManifest)
	if err != nil {
		t.Fatal(err)
	}
	r := &keyRun{j: &resolveJob{m: m}, st: stock{Symbol: "AAPL", Currency: "USD"}, t: mustDay(runDay), pos: pos}
	cands, skipped := r.candidates([]broker.Chain{chain}, refs, 40, 1.0, len(pos) > 0)
	got := map[string]bool{}
	for _, c := range cands {
		got[c.req.Expiry+"|"+c.req.Strike] = true
	}
	return got, skipped
}

func TestWeeklyIsAskedOnItsStoredGrid(t *testing.T) {
	t.Parallel()
	refs := append(storedAt("2026-12-11", "30", "35", "40"), storedAt("2026-12-18", "30", "31", "32", "33", "34", "35")...)
	got, skipped := gridAsked(t, gridChain(weeklyDec11, monthlyDec18), refs, nil)
	if got[weeklyDec11+"|31"] || got[weeklyDec11+"|32"] {
		t.Fatalf("the weekly steps by 5 and is not asked 31 or 32: %v", got)
	}
	if !got[weeklyDec11+"|45"] {
		t.Fatalf("45 is on the grid of 5 and is asked: %v", got)
	}
	for _, s := range []string{"31", "32", "45"} {
		if !got[monthlyDec18+"|"+s] {
			t.Fatalf("the monthly steps by 1 and is asked %s: %v", s, got)
		}
	}
	if skipped == 0 {
		t.Fatal("the dropped pairs are counted")
	}
}

func TestExpiryWithNoStoredStrikeTakesTheStepOfItsKind(t *testing.T) {
	t.Parallel()
	// Dec 25 is a weekly: it takes the step of the stored weekly (5), not that of the nearer monthly (1).
	refs := append(storedAt("2026-12-11", "30", "35", "40"), storedAt("2026-12-18", "30", "31", "32", "33")...)
	got, _ := gridAsked(t, gridChain(weeklyDec25), refs, nil)
	if got[weeklyDec25+"|31"] || got[weeklyDec25+"|32"] || !got[weeklyDec25+"|45"] {
		t.Fatalf("an unstored weekly takes the weekly step: %v", got)
	}
}

func TestHeldContractIsAskedAtItsExpiryWhateverTheGrid(t *testing.T) {
	t.Parallel()
	pos := []store.Position{{SecType: "option", Symbol: "AAPL", Expiry: "2026-12-11", Strike: "31", Right: "put", Quantity: "-1", Multiplier: "100"}}
	got, _ := gridAsked(t, gridChain(weeklyDec11), storedAt("2026-12-11", "30", "35", "40"), pos)
	if !got[weeklyDec11+"|31"] {
		t.Fatalf("a held contract off the grid is asked: %v", got)
	}
}

func TestExpiryWithNoStoredExpiryOfItsKindAsksTheWholeUnion(t *testing.T) {
	t.Parallel()
	// Only a monthly is stored: the weekly has no expiry of its kind, so nothing is dropped.
	got, skipped := gridAsked(t, gridChain(weeklyDec11), storedAt("2026-12-18", "30", "35", "40"), nil)
	if skipped != 0 || !got[weeklyDec11+"|31"] || !got[weeklyDec11+"|32"] || !got[weeklyDec11+"|45"] {
		t.Fatalf("the whole union is asked, skipped %d: %v", skipped, got)
	}
	got, skipped = gridAsked(t, gridChain(weeklyDec11), nil, nil)
	if skipped != 0 || !got[weeklyDec11+"|31"] {
		t.Fatalf("with nothing stored the whole union is asked, skipped %d: %v", skipped, got)
	}
}
