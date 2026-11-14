package bookrisk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

func (w *world) loadAlias(t *testing.T, alias string) loaded {
	t.Helper()
	l, err := w.newJob(t).load(context.Background(), alias, runDay)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestBookRiskNewIndexClassLegIsCashSettledWithEmptyTerms(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.addPut("mine", "SPX", "USD")
	w.addPut("mine", "AAA", "USD")
	got := map[string]bool{}
	for _, h := range w.loadAlias(t, "mine").book.Holdings {
		got[h.Underlying] = h.Option.CashSettled
		if h.Underlying == "SPX" && h.Class != "broad_high" {
			t.Fatalf("SPX class = %q", h.Class)
		}
	}
	if !got["SPX"] || got["AAA"] {
		t.Fatalf("cash settled = %v", got)
	}
}

func TestBookRiskNewCashSettledFromTermsFields(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		style, settle string
		want          bool
	}{{"european", "cash", true}, {"European", "Cash", true}, {"american", "physical", false},
		{"european", "physical", false}, {"unknown", "unknown", false}, {"", "", false}} {
		w := newWorld()
		w.addPut("mine", "AAA", "USD")
		w.capture["mine"] = append(w.capture["mine"], store.Position{Symbol: "AAA", SecType: "STOCK", Currency: "USD", ConID: 5, Quantity: "100"})
		w.eod[5] = []contracts.OptionEod{{InstrumentID: 9, SessionDay: date("2026-11-06"), Bid: f64(1), Ask: f64(2), UnderlyingPrice: f64(55)}}
		w.terms = []store.OptionTerms{{InstrumentID: 9, Right: "put", Strike: "50", ExpirationDate: "2026-12-18",
			ExerciseStyle: c.style, SettlementType: c.settle}}
		var opt *bool
		for _, h := range w.loadAlias(t, "mine").book.Holdings {
			if h.Option != nil {
				v := h.Option.CashSettled
				opt = &v
			}
		}
		if opt == nil || *opt != c.want {
			t.Fatalf("%s/%s: cash settled = %v, want %v", c.style, c.settle, opt, c.want)
		}
	}
}

func TestBookRiskNewCaptureOnlyPositionIsNotInLedger(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine"}
	w.addPut("mine", "AAA", "USD")
	w.capture["mine"] = append(w.capture["mine"], store.Position{Symbol: "ZZZ", SecType: "OPTION", Right: "call", Strike: "10",
		Expiry: "2026-12-18", Quantity: "-1", Currency: "USD"})
	l := w.loadAlias(t, "mine")
	if len(l.notInLedger) != 1 || l.notInLedger[0].Underlying != "ZZZ" || l.notInLedger[0].Reason != "not_in_ledger" ||
		l.notInLedger[0].Currency != "USD" {
		t.Fatalf("notInLedger = %+v", l.notInLedger)
	}
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	rows := w.run(t)
	r := rows["book_risk/mine:USD"]
	f := factsOf(t, r)
	if r.Status != "unknown" || !strings.Contains(f["excluded"], "not_in_ledger") {
		t.Fatalf("row = %s %v", r.Status, f)
	}
	if _, has := f["worst_loss_USD"]; has {
		t.Fatal("a currency with a capture-only position keeps its total")
	}
}

func TestBookRiskNewSharePriceFromQuoteElseMark(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.addPut("mine", "AAA", "USD")
	w.capture["mine"] = append(w.capture["mine"],
		store.Position{Symbol: "AAA", SecType: "STOCK", Currency: "USD", ConID: 5, Quantity: "100"},
		store.Position{Symbol: "BBB", SecType: "STOCK", Currency: "USD", ConID: 6, Quantity: "10"},
		store.Position{Symbol: "CCC", SecType: "STOCK", Currency: "USD", ConID: 7, Quantity: "10"})
	w.eod[5] = []contracts.OptionEod{{InstrumentID: 9, SessionDay: date("2026-11-06"), Bid: f64(1), Ask: f64(2), UnderlyingPrice: f64(55.5)}}
	w.terms = []store.OptionTerms{{InstrumentID: 9, Right: "put", Strike: "50", ExpirationDate: "2026-12-18"}}
	w.marks["mine"] = []mark.PositionMark{{ConID: 5, Price: "1", Status: "ok", Basis: "mid", SessionDay: "2026-11-06"},
		{ConID: 6, Price: "20.25", Status: "ok", Basis: "mid", SessionDay: "2026-11-06"},
		{ConID: 7, Price: "", Status: "ok", Basis: "none", SessionDay: "2026-11-06"}}
	spots := map[string]string{}
	usable := map[string]bool{}
	for _, h := range w.loadAlias(t, "mine").book.Holdings {
		if h.Option == nil {
			spots[h.Underlying], usable[h.Underlying] = h.Spot, h.MarkUsable
		}
	}
	if spots["AAA"] != "55.5" || spots["BBB"] != "20.25" || spots["CCC"] != "" || !usable["AAA"] || !usable["BBB"] || usable["CCC"] {
		t.Fatalf("spots %v usable %v", spots, usable)
	}
}

func TestBookRiskNewAliasWithNothingHasNoBook(t *testing.T) {
	t.Parallel()
	w := newWorld()
	l := w.loadAlias(t, "mine")
	if !l.empty {
		t.Fatal("an alias with no ledger row and no capture has a book")
	}
	if rows := w.run(t); len(rows) != 0 || len(w.inserted) != 0 {
		t.Fatalf("rows written for an empty book: %v", keysOf(rows))
	}
}

// pageSequence is a copy of the sequence of internal/pages/risk_page/load.go (ledger rows, Replay, capture of
// OPTION and STOCK rows, quotes through the option terms) followed by Evaluate; the page package is not imported.
func pageSequence(t *testing.T, w *world, alias string) []compute.StructureLeg {
	t.Helper()
	ctx := context.Background()
	rows, _ := ledgerFake{w}.FlexTradeRows(ctx, alias)
	var trades []compute.LedgerTrade
	for _, r := range rows {
		trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: map[string]string{}})
	}
	days := []contracts.Day{"2026-11-05", "2026-11-06", "2026-11-09"}
	res := replayFake{w}.Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: days, AsOf: runDay}, compute.LedgerParams{})
	in := compute.StructureInput{Alias: alias, AsOf: runDay, SessionDays: days, Positions: res.Positions, Cycles: res.Cycles, HeldDay: "2026-11-06"}
	conIDs := map[string]int64{}
	for _, p := range w.capture[alias] {
		switch p.SecType {
		case "OPTION":
			in.Held = append(in.Held, compute.HeldOption{Underlying: p.Symbol, Right: strings.ToUpper(p.Right[:1]),
				Strike: p.Strike, Expiry: p.Expiry, Quantity: p.Quantity})
		case "STOCK":
			conIDs[p.Symbol] = p.ConID
		}
	}
	for _, p := range in.Positions {
		for _, e := range w.eod[conIDs[p.Underlying]] {
			for _, tm := range w.terms {
				if int(tm.InstrumentID) == e.InstrumentID {
					in.Quotes = append(in.Quotes, quoteOf(p.Underlying, tm, e))
				}
			}
		}
	}
	var legs []compute.StructureLeg
	for _, s := range (structFake{}).Evaluate(in, compute.StructureParams{}).Structures {
		legs = append(legs, s.Legs...)
	}
	return legs
}

func TestBookRiskNewLegsEqualRiskPageStructureLegs(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.addPut("mine", "AAA", "USD")
	w.addPut("mine", "BBB", "USD")
	w.capture["mine"] = append(w.capture["mine"], store.Position{Symbol: "AAA", SecType: "STOCK", Currency: "USD", ConID: 5, Quantity: "100"})
	w.eod[5] = []contracts.OptionEod{{InstrumentID: 9, SessionDay: date("2026-11-06"), Bid: f64(1), Ask: f64(2), UnderlyingPrice: f64(55)}}
	w.terms = []store.OptionTerms{{InstrumentID: 9, Right: "put", Strike: "50", ExpirationDate: "2026-12-18"}}
	want := pageSequence(t, w, "mine")
	var got []compute.StructureLeg
	for _, h := range w.loadAlias(t, "mine").book.Holdings {
		if h.Option != nil {
			got = append(got, compute.StructureLeg{Right: h.Option.Right, Strike: h.Option.Strike, Expiry: h.Option.Expiry,
				Quantity: h.Quantity, Multiplier: h.Multiplier, Mid: h.Mark})
		}
	}
	if len(want) != 2 || len(got) != len(want) {
		t.Fatalf("legs: got %d want %d", len(got), len(want))
	}
	for i := range want {
		w := want[i]
		w.PositionID, w.OpenPrice, w.MarkStatus, w.LegPnL = "", "", "", ""
		if got[i] != w {
			t.Fatalf("leg %d: got %+v want %+v", i, got[i], w)
		}
	}
	if want[0].Mid != "1.5" && want[1].Mid != "1.5" {
		t.Fatalf("the fake world gives no mark: %+v", want)
	}
}
