package optionstruct

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionledger"
)

type wantUnit struct {
	Kind string
	Lots []string
}

type matchCase struct {
	Name      string
	Positions []compute.LedgerPosition
	Cycles    []compute.LedgerCycle
	Want      []wantUnit
}

func loadMatchCases(t *testing.T) []matchCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "match_kinds.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Cases []matchCase }
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Cases
}

func lotsOf(s compute.Structure) []string {
	var out []string
	for _, l := range s.Legs {
		out = append(out, l.Quantity)
	}
	return out
}

func TestMatchKinds(t *testing.T) {
	t.Parallel()
	cases := loadMatchCases(t)
	if len(cases) != 11 {
		t.Fatalf("cases = %d, want one for each of 11 new kinds", len(cases))
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			res := evaluate(c.Positions, c.Cycles)
			if len(res.Other) != 0 || len(res.Structures) != len(c.Want) {
				t.Fatalf("structures = %v, other = %v, want %v", kinds(res.Structures), kinds(res.Other), c.Want)
			}
			for i, w := range c.Want {
				got := res.Structures[i]
				if got.Kind != w.Kind || strings.Join(lotsOf(got), ",") != strings.Join(w.Lots, ",") {
					t.Errorf("unit %d = %s %v, want %s %v", i, got.Kind, lotsOf(got), w.Kind, w.Lots)
				}
			}
		})
	}
}

// ledgerRow is an option trade row of one contract; expiry and date are yyyymmdd.
func ledgerRow(tx, qty, proceeds, oc, date, conid, strike, notes string) compute.LedgerTrade {
	return compute.LedgerTrade{TransactionID: tx, Revision: 1, Attributes: map[string]string{
		"assetCategory": "OPT", "symbol": "ABC", "underlyingSymbol": "ABC", "conid": conid,
		"strike": strike, "expiry": "20260220", "putCall": "P", "multiplier": "100",
		"quantity": qty, "proceeds": proceeds, "tradePrice": "1.00", "ibCommission": "-1.00", "currency": "USD",
		"openCloseIndicator": oc, "notes": notes, "tradeDate": date, "orderTime": date + ";100000",
	}}
}

func replayAndEvaluate(trades ...compute.LedgerTrade) compute.StructureResult {
	led := optionledger.New().Replay(compute.LedgerInput{Alias: "acct", Trades: trades, AsOf: "2026-02-01"},
		compute.LedgerParams{MarkMaxAgeSessions: 5})
	return evaluate(led.Positions, led.Cycles)
}

func TestPartialCloseSplitsVertical(t *testing.T) {
	t.Parallel()
	// A vertical of 2 lots, then 1 lot of the short put bought back: the live lots are a
	// vertical of 1 lot and 1 long put.
	res := replayAndEvaluate(
		ledgerRow("1", "-2", "300", "O", "20260105", "111", "50", ""),
		ledgerRow("2", "2", "-100", "O", "20260105", "222", "45", ""),
		ledgerRow("3", "1", "-100", "C", "20260106", "111", "50", ""),
	)
	got := map[string][]string{}
	for _, s := range res.Structures {
		got[s.Kind] = lotsOf(s)
	}
	if len(res.Structures) != 2 || len(res.Other) != 0 ||
		strings.Join(got["put_spread"], ",") != "1,-1" || strings.Join(got["long_put"], ",") != "1" {
		t.Errorf("structures = %v, want put_spread 1,-1 and long_put 1", got)
	}
}

func TestAssignedPutLeavesLongPut(t *testing.T) {
	t.Parallel()
	res := replayAndEvaluate(
		ledgerRow("1", "-1", "150", "O", "20260105", "111", "50", ""),
		ledgerRow("2", "1", "-50", "O", "20260105", "222", "45", ""),
		ledgerRow("3", "1", "0", "C", "20260220", "111", "50", "A"),
		compute.LedgerTrade{TransactionID: "4", Revision: 1, Attributes: map[string]string{
			"assetCategory": "STK", "symbol": "ABC", "quantity": "100", "tradePrice": "50",
			"proceeds": "-5000", "ibCommission": "-1", "currency": "USD",
			"tradeDate": "20260220", "orderTime": "20260220;100000",
		}},
	)
	if len(res.Structures) != 1 || res.Structures[0].Kind != "long_put" || len(res.Other) != 0 ||
		strings.Join(lotsOf(res.Structures[0]), ",") != "1" {
		t.Errorf("structures = %v, other = %v, want the long put alone", kinds(res.Structures), kinds(res.Other))
	}
}

func TestCollarThenCoveredCall(t *testing.T) {
	t.Parallel()
	build := func(shares string) compute.StructureResult {
		cycles := []compute.LedgerCycle{{CycleID: "c1", Alias: "acct", Underlying: "AAPL", Currency: "USD",
			Shares: shares, CostBasisPerShare: "50"}}
		return evaluate([]compute.LedgerPosition{
			pos("c1", "C", "55", "-1", "1.00"), pos("p1", "P", "45", "1", "1.00"), pos("c2", "C", "60", "-1", "0.50"),
		}, cycles)
	}
	for shares, second := range map[string]string{"100": "naked_call", "200": "covered_call"} {
		res := build(shares)
		got := map[string]string{}
		for _, s := range res.Structures {
			got[s.Kind] = s.ID
		}
		if len(res.Structures) != 2 || got["collar"] == "" || got[second] != "acct:c2" {
			t.Errorf("shares %s: %v, want a collar and a %s of c2", shares, kinds(res.Structures), second)
		}
	}
}

func TestSliceCommissionsSum(t *testing.T) {
	t.Parallel()
	// A short put of 3 lots with a commission of 1 is shared by three one-lot verticals: the
	// slices carry 0.3333, 0.3333 and 0.3334, which add up to the commission of the piece.
	s := pos("s1", "P", "50", "-3", "1.50")
	s.Fills = []compute.LedgerFill{{Day: "2026-01-02", Action: "open", Quantity: "-3", Price: "1.50", Commission: "1"}}
	in := compute.StructureInput{Alias: "acct", Positions: []compute.LedgerPosition{
		s, pos("b1", "P", "45", "1", "0.50"), pos("b2", "P", "44", "1", "0.50"), pos("b3", "P", "43", "1", "0.50"),
	}}
	sum, slices := new(big.Rat), 0
	for _, u := range recognise(in) {
		for _, l := range u.legs {
			if l.p.PositionID != "s1" {
				continue
			}
			slices++
			c, ok := parseDec(l.p.Fills[0].Commission)
			if !ok {
				t.Fatalf("commission %q does not parse", l.p.Fills[0].Commission)
			}
			sum = add(sum, c)
			if l.p.Fills[0].Quantity != "-3" || l.p.Quantity != "-1" {
				t.Errorf("slice quantity %s, fill quantity %s, want -1 and the fill's own -3", l.p.Quantity, l.p.Fills[0].Quantity)
			}
		}
	}
	if slices != 3 || sum.Cmp(big.NewRat(1, 1)) != 0 {
		t.Errorf("%d slices add up to %s, want 3 slices adding to 1", slices, sum.FloatString(4))
	}
}

func TestWholePieceKeepsText(t *testing.T) {
	t.Parallel()
	s := pos("s1", "P", "50", "-1.0", "1.50")
	in := compute.StructureInput{Alias: "acct", Positions: []compute.LedgerPosition{s, pos("b1", "P", "45", "1", "0.50")}}
	for _, u := range recognise(in) {
		for _, l := range u.legs {
			if l.p.PositionID == "s1" && l.p.Quantity != "-1.0" {
				t.Errorf("whole piece quantity = %q, want its own text -1.0", l.p.Quantity)
			}
		}
	}
}
