package replayapi_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// fakeReplayer reports a lot with no quote on a day as not priced, and echoes every field of the types.
type fakeReplayer struct{}

func (fakeReplayer) Replay(lots []replayapi.Lot, mkt replayapi.Market, variants []replayapi.Variant,
	days []contracts.Day, asOf contracts.Day, minLots int) replayapi.Report {
	rep := replayapi.Report{Lots: len(lots), Excluded: map[string]int{}}
	for _, l := range lots {
		for _, v := range variants {
			o := replayapi.Outcome{LotID: l.ID, Variant: v.Name}
			missing := false
			for _, d := range days {
				if d > l.EntryDay && d <= l.Expiry {
					if _, ok := mkt.Quotes[l.InstrumentID][d]; !ok {
						missing = true
					}
				}
			}
			if missing {
				o.Reason = "gap_before_close"
				rep.Excluded[v.Name+":"+o.Reason]++
			} else {
				o.Priced, o.ClosedOn, o.CloseReason, o.Gross, o.InTheMoney = true, asOf, "expiry", l.Credit, true
			}
			rep.Outcomes = append(rep.Outcomes, o)
		}
	}
	rep.Lines = []replayapi.Line{{
		Set: "all", Variant: "hold", ParamHash: "h", Currency: "USD",
		Lots: 1, Cycles: 2, ClosedEarly: 3, Held: 4, Assigned: 5, Rolled: 6, InTheMoney: 7,
		KeptShare: "0.5", MeanGross: "1.2", WorstGross: "-3", HoldKeptShare: "0.4", ActualKeptShare: "0.3",
		UnderMinimum: len(lots) < minLots,
	}}
	rep.LotsWithQuote, rep.StillOpen, rep.LotDays, rep.QuoteDays = 1, 2, 3, 4
	rep.FirstQuoteDay, rep.LastQuoteDay = "2026-11-07", "2026-11-12"
	return rep
}

func TestReplayerFromBothSides(t *testing.T) {
	t.Parallel()
	var r replayapi.Replayer = fakeReplayer{}
	lot := replayapi.Lot{
		ID: "L1", Underlying: "AAPL", Right: "P", Strike: "50", Currency: "USD", Multiplier: "100",
		InstrumentID: "7", Expiry: "2026-11-13", EntryDay: "2026-11-09", Credit: "1.5", Fate: "held",
		SettleDay: "2026-11-13", ActualGross: "1.5", ActualNet: "1.4",
	}
	mkt := replayapi.Market{
		Quotes:         map[string]map[contracts.Day]replayapi.Quote{"7": {"2026-11-10": {Bid: "1", Ask: "1.1"}}},
		Closes:         map[string]map[contracts.Day]string{"AAPL": {"2026-11-13": "55"}},
		AdjustedCloses: map[string]map[contracts.Day]string{"AAPL": {"2026-11-13": "55"}},
	}
	v := replayapi.Variant{Name: "early_50_21", Strategy: nil}
	days := []contracts.Day{"2026-11-10", "2026-11-11"} // 11-11 has no quote
	rep := r.Replay([]replayapi.Lot{lot}, mkt, []replayapi.Variant{v}, days, "2026-11-14", 20)
	if len(rep.Outcomes) != 1 || rep.Outcomes[0].Priced || rep.Outcomes[0].Reason != "gap_before_close" ||
		rep.Excluded["early_50_21:gap_before_close"] != 1 {
		t.Fatalf("absent day must give an unpriced outcome: %+v", rep)
	}
	if q, ok := mkt.Quotes["7"]["2026-11-11"]; ok || q != (replayapi.Quote{}) {
		t.Fatal("absent day must stay absent")
	}
	// every field written and read back
	mkt.Quotes["7"]["2026-11-11"] = replayapi.Quote{Bid: "1", Ask: "1.2"}
	rep = r.Replay([]replayapi.Lot{lot}, mkt, []replayapi.Variant{v}, days, "2026-11-14", 20)
	wantO := replayapi.Outcome{LotID: "L1", Variant: "early_50_21", Priced: true, ClosedOn: "2026-11-14",
		CloseReason: "expiry", Gross: "1.5", InTheMoney: true}
	if rep.Outcomes[0] != wantO {
		t.Fatalf("outcome = %+v", rep.Outcomes[0])
	}
	wantL := replayapi.Line{
		Set: "all", Variant: "hold", ParamHash: "h", Currency: "USD",
		Lots: 1, Cycles: 2, ClosedEarly: 3, Held: 4, Assigned: 5, Rolled: 6, InTheMoney: 7,
		KeptShare: "0.5", MeanGross: "1.2", WorstGross: "-3", HoldKeptShare: "0.4", ActualKeptShare: "0.3", UnderMinimum: true,
	}
	if rep.Lines[0] != wantL {
		t.Fatalf("line = %+v", rep.Lines[0])
	}
	checkReport(t, rep)
	checkLot(t, lot)
}

func checkReport(t *testing.T, rep replayapi.Report) {
	t.Helper()
	if rep.Lots != 1 || rep.LotsWithQuote != 1 || rep.StillOpen != 2 || rep.LotDays != 3 || rep.QuoteDays != 4 ||
		rep.FirstQuoteDay != "2026-11-07" || rep.LastQuoteDay != "2026-11-12" {
		t.Fatalf("report = %+v", rep)
	}
}

func checkLot(t *testing.T, lot replayapi.Lot) {
	t.Helper()
	if lot.Fate != "held" || lot.SettleDay != "2026-11-13" || lot.ActualGross != "1.5" || lot.ActualNet != "1.4" {
		t.Fatalf("lot = %+v", lot)
	}
	if lot.Underlying != "AAPL" || lot.Right != "P" || lot.Strike != "50" || lot.Currency != "USD" || lot.Multiplier != "100" {
		t.Fatalf("lot = %+v", lot)
	}
}

func TestReplayapiImportsNoStoreJobsPages(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		n++
		if _, err := os.Stat(f); err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			p := strings.Trim(im.Path.Value, `"`)
			for _, bad := range []string{"internal/store", "internal/jobs", "internal/pages"} {
				if strings.Contains(p, bad) {
					t.Errorf("%s imports %s", f, p)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no non-test file found")
	}
}
