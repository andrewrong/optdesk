package replay_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy/replay"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
	"github.com/andrewrong/optdesk/internal/strategy/shortput"
)

// fixture is testdata/walk.json: synthetic lots, quotes and closes, no account data. L2 is an assigned lot in the
// money (strike 100, close 95), L1 a held lot whose ask falls to 0.90 on 2026-10-08, L3 a lot his habit closed early.
type fixture struct {
	AsOf        contracts.Day
	SessionDays []contracts.Day
	Lots        []replayapi.Lot
	Market      replayapi.Market
}

func load(t *testing.T) fixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/walk.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// early is the shortput variant of fixed parameters: close at half the credit kept, or at dte days to expiry.
func early(t *testing.T, name, dte string) replayapi.Variant {
	t.Helper()
	p := compute.CandidateParams{
		MinDTE: 21, MaxDTE: 90, MinBidSize: 1, MaxSpread: "0.10", MinOpenInterest: 10,
		DivGuardFactor: "1.0", MaxRateAgeDays: 7, MaxEventAgeSessions: 2, MinReportGapDays: 60,
	}
	s, err := shortput.New(optioncand.New(), p, shortput.Values{CloseMode: "early", EarlyKeep: "0.50", EarlyDTE: dte})
	if err != nil {
		t.Fatal(err)
	}
	return replayapi.Variant{Name: name, Strategy: s}
}

func run(t *testing.T, f fixture, minLots int, vs ...replayapi.Variant) replayapi.Report {
	t.Helper()
	return replay.New().Replay(f.Lots, f.Market, vs, f.SessionDays, f.AsOf, minLots)
}

func outcome(t *testing.T, rep replayapi.Report, lot, variant string) replayapi.Outcome {
	t.Helper()
	for _, o := range rep.Outcomes {
		if o.LotID == lot && o.Variant == variant {
			return o
		}
	}
	t.Fatalf("no outcome of %s under %s", lot, variant)
	return replayapi.Outcome{}
}

func TestHoldNeedsNoQuote(t *testing.T) {
	t.Parallel()
	rep := run(t, load(t), 1)
	// L2 and L8 have no stored quote at all (L8 not even an instrument); hold prices them from the close.
	for lot, want := range map[string]string{"L1": "2", "L2": "-3", "L8": "2"} {
		o := outcome(t, rep, lot, "hold")
		if !o.Priced || o.Gross != want || o.CloseReason != "expiry" {
			t.Errorf("%s hold = %+v, want gross %s", lot, o, want)
		}
	}
	if !outcome(t, rep, "L2", "hold").InTheMoney || outcome(t, rep, "L1", "hold").InTheMoney {
		t.Error("only L2 settles in the money")
	}
}

func TestEarlyKeepClosesAtAsk(t *testing.T) {
	t.Parallel()
	rep := run(t, load(t), 1, early(t, "early_50_21", "21"))
	o := outcome(t, rep, "L1", "early_50_21")
	// credit 2.00, ask 0.90 on 2026-10-08 keeps 55%: closed there at the ask, gross 1.10.
	if !o.Priced || o.ClosedOn != "2026-10-08" || o.CloseReason != "early_keep" || o.Gross != "1.1" {
		t.Errorf("outcome = %+v", o)
	}
}

func TestEarlyDTEIgnoredForLotOpenedInsideWindow(t *testing.T) {
	t.Parallel()
	f := load(t)
	// L3 was opened 11 days before expiry: inside a 21 day window, so early_dte does not close it the day after entry.
	rep := run(t, f, 1, early(t, "early_50_21", "21"), early(t, "early_50_5", "5"))
	o := outcome(t, rep, "L3", "early_50_21")
	if !o.Priced || o.CloseReason != "expiry" || o.Gross != "2" {
		t.Errorf("window 21: %+v", o)
	}
	// With a 5 day window the lot was opened outside it and early_dte fires on 2026-10-12, 4 days before expiry, at ask 1.50.
	o = outcome(t, rep, "L3", "early_50_5")
	if !o.Priced || o.CloseReason != "early_dte" || o.ClosedOn != "2026-10-12" || o.Gross != "0.5" {
		t.Errorf("window 5: %+v", o)
	}
}

func TestGapBeforeCloseIsUnpriced(t *testing.T) {
	t.Parallel()
	f := load(t)
	delete(f.Market.Quotes["i3"], "2026-10-09") // a hole in a lot that would otherwise run to expiry
	rep := run(t, f, 1, early(t, "early_50_21", "21"))
	for _, lot := range []string{"L2", "L3"} { // L2 has no quote at all
		o := outcome(t, rep, lot, "early_50_21")
		if o.Priced || o.Reason != "gap_before_close" {
			t.Errorf("%s: %+v", lot, o)
		}
		if !outcome(t, rep, lot, "hold").Priced {
			t.Errorf("%s: a missing quote must not make hold unpriced", lot)
		}
	}
	// L1 closes on 2026-10-08, before any hole of its own, and every day up to it has a quote.
	if o := outcome(t, rep, "L1", "early_50_21"); !o.Priced {
		t.Errorf("L1: %+v", o)
	}
	if rep.Excluded["early_50_21:gap_before_close"] != 2 {
		t.Errorf("excluded = %v", rep.Excluded)
	}
}

func TestNotExpiredIsNotMissingPrice(t *testing.T) {
	t.Parallel()
	rep := run(t, load(t), 1, early(t, "early_50_21", "21"))
	for _, v := range []string{"hold", "early_50_21"} {
		o := outcome(t, rep, "L5", v)
		if o.Priced || o.Reason != "not_expired" {
			t.Errorf("%s: %+v", v, o)
		}
	}
	if o := outcome(t, rep, "L7", "hold"); o.Reason != "no_settle_price" {
		t.Errorf("L7: %+v", o)
	}
	if rep.StillOpen != 1 || rep.Excluded["hold:not_expired"] != 1 {
		t.Errorf("still open %d, excluded %v", rep.StillOpen, rep.Excluded)
	}
}

func TestAssignedItmLotActualEqualsHoldBasis(t *testing.T) {
	t.Parallel()
	f := load(t)
	rep := run(t, f, 1)
	o := outcome(t, rep, "L2", "hold")
	if !o.InTheMoney || o.Gross != "-3" || f.Lots[1].ActualGross != "-3.00" {
		t.Fatalf("hold %+v, actual %s", o, f.Lots[1].ActualGross)
	}
	for _, ln := range rep.Lines {
		if ln.Variant == "actual:assigned" && ln.Currency == "USD" && (ln.MeanGross != "-3" || ln.Assigned != 1) {
			t.Errorf("assigned row = %+v", ln)
		}
	}
}

func TestSplitInLifeExcluded(t *testing.T) {
	t.Parallel()
	rep := run(t, load(t), 1, early(t, "early_50_21", "21"))
	for _, v := range []string{"hold", "early_50_21"} {
		if o := outcome(t, rep, "L6", v); o.Priced || o.Reason != "split_in_life" {
			t.Errorf("%s: %+v", v, o)
		}
	}
	if rep.Excluded["hold:split_in_life"] != 1 {
		t.Errorf("excluded = %v", rep.Excluded)
	}
}
