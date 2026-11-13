package shortput_test

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/strategy"
)

func row(id, status, yield, dte, strike string) strategy.Evaluation {
	return strategy.Evaluation{
		Key: id, Status: status, RankInUnderlying: 9,
		Legs:  []strategy.Leg{{InstrumentID: id, Side: "short", Right: "P", Strike: strike, Quantity: 1}},
		Facts: map[string]string{"annualized_yield": yield, "dte": dte},
	}
}

// The order is the one rankRun of the job writes: yield descending as a rational, then DTE, strike and instrument id
// ascending; a row that is not ok has no rank and comes after the ranked ones.
func TestShortPutRankEqualsRankRun(t *testing.T) {
	t.Parallel()
	in := []strategy.Evaluation{
		row("3", "ok", "0.30", "30", "100"),
		row("1", "ok", "0.300000", "30", "100"),
		row("2", "ok", "0.3", "25", "100"),
		row("4", "ok", "0.50", "30", "100"),
		row("5", "ok", "0.30", "30", "95"),
		row("6", "blocked", "0.90", "30", "100"),
		row("10", "ok", "0.20", "30", "100"),
		row("9", "ok", "0.20", "30", "100"),
		row("7", "unknown", "0.40", "30", "100"),
	}
	wantKeys := []string{"4", "2", "5", "1", "3", "9", "10", "6", "7"}
	wantRanks := []int{1, 2, 3, 4, 5, 6, 7, 0, 0}
	got := newPut(t).Rank(in)
	var keys []string
	var ranks []int
	for _, e := range got {
		keys = append(keys, e.Key)
		ranks = append(ranks, e.Rank)
		if e.RankInUnderlying != 9 {
			t.Errorf("%s: RankInUnderlying changed to %d", e.Key, e.RankInUnderlying)
		}
	}
	if !reflect.DeepEqual(keys, wantKeys) || !reflect.DeepEqual(ranks, wantRanks) {
		t.Fatalf("order %v ranks %v", keys, ranks)
	}
	if in[0].Key != "3" || in[0].Rank != 0 {
		t.Fatalf("input changed: %+v", in[0])
	}
}
