package basecheckapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// TestFuncServesCaller: a caller holds a Func and reads the verdict, counts and rows it returns.
func TestFuncServesCaller(t *testing.T) {
	t.Parallel()
	var f basecheckapi.Func = func(in basecheckapi.Input) basecheckapi.Output {
		if len(in.NotRead) > 0 {
			return basecheckapi.Output{
				Level:  basecheckapi.Unknown,
				Counts: map[basecheckapi.Level]int{basecheckapi.Unknown: len(in.NotRead)},
				Rows:   []basecheckapi.Row{{Dataset: in.NotRead[0], Level: basecheckapi.Unknown}},
			}
		}
		return basecheckapi.Output{Level: basecheckapi.Pass}
	}
	day := contracts.Day("2026-11-17")
	out := f(basecheckapi.Input{
		Facts:    []basecheckapi.Fact{{Dataset: "bars", Rule: "newest", Day: day, Value: "NVDA 181.42"}},
		NotRead:  []string{"quotes"},
		Cells:    []basecheckapi.Cell{{Day: day, Dataset: "bars", Keys: 3}},
		Days:     map[string]contracts.Day{"XNYS": day, "XTKS": day},
		Sessions: map[string][]contracts.Day{"XNYS": {day}, "XTKS": {day}},
	})
	if out.Level != basecheckapi.Unknown || out.Counts[basecheckapi.Unknown] != 1 || out.Rows[0].Dataset != "quotes" {
		t.Fatalf("unexpected output %+v", out)
	}
	if got := f(basecheckapi.Input{}); got.Level != basecheckapi.Pass {
		t.Fatalf("empty input level = %q", got.Level)
	}
}

// TestLevelValues: the four levels are these strings, which the page schema carries.
func TestLevelValues(t *testing.T) {
	t.Parallel()
	for got, want := range map[basecheckapi.Level]string{
		basecheckapi.Pass: "pass", basecheckapi.Warn: "warn",
		basecheckapi.Fail: "fail", basecheckapi.Unknown: "unknown",
	} {
		if string(got) != want {
			t.Errorf("level %q, want %q", got, want)
		}
	}
}

// TestInputCarriesPairs: the pair levels travel on Input and a caller reads them back by row.
func TestInputCarriesPairs(t *testing.T) {
	t.Parallel()
	in := basecheckapi.Input{Pairs: []basecheckapi.PairLevel{{Dataset: "bars", Pair: "R6", Level: basecheckapi.Warn}}}
	if len(in.Pairs) != 1 || in.Pairs[0].Dataset != "bars" || in.Pairs[0].Pair != "R6" || in.Pairs[0].Level != basecheckapi.Warn {
		t.Fatalf("unexpected pairs %+v", in.Pairs)
	}
}
