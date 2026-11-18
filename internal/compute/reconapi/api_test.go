package reconapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
)

// TestFuncServesCaller: a caller holds a Func and a VerdictFunc and reads the items, counts and level.
func TestFuncServesCaller(t *testing.T) {
	t.Parallel()
	var f reconapi.Func = func(in reconapi.Input) reconapi.Output {
		o := reconapi.Output{}
		for _, v := range in.Ours {
			if reason, ok := in.Skip[v.Key]; ok {
				o.Items = append(o.Items, reconapi.Item{Key: v.Key, Kind: reconapi.Skipped, Reason: reason})
				o.Skipped++
				continue
			}
			o.Items = append(o.Items, reconapi.Item{Key: v.Key, Ours: v.Amount, Kind: reconapi.MissingOther})
			o.MissingOther++
			o.N++
		}
		return o
	}
	out := f(reconapi.Input{
		Ours: []reconapi.Value{{Key: "NVDA|2026-11-17", Amount: "181.42"}, {Key: "AAPL|2026-11-17", Amount: "1"}},
		Skip: map[string]string{"AAPL|2026-11-17": "source_lacks"},
		Tol:  reconapi.Tolerance{Abs: "0.01", Rel: "0.001"},
	})
	if out.N != 1 || out.Skipped != 1 || out.Items[1].Reason != "source_lacks" {
		t.Fatalf("unexpected output %+v", out)
	}
	var v reconapi.VerdictFunc = func(_ string, o reconapi.Output, asked, compared int) basecheckapi.Level {
		if o.N == 0 || compared*2 < asked {
			return basecheckapi.Unknown
		}
		return basecheckapi.Pass
	}
	if got := v("R6", out, 2, 2); got != basecheckapi.Pass {
		t.Fatalf("level = %q", got)
	}
	if got := v("R6", reconapi.Output{}, 2, 2); got != basecheckapi.Unknown {
		t.Fatalf("empty level = %q", got)
	}
}

// TestKindValues: the five kinds are these strings, which the store and the page carry.
func TestKindValues(t *testing.T) {
	t.Parallel()
	for got, want := range map[reconapi.Kind]string{
		reconapi.Match: "match", reconapi.Diff: "diff", reconapi.MissingOurs: "missing_ours",
		reconapi.MissingOther: "missing_other", reconapi.Skipped: "skipped",
	} {
		if string(got) != want {
			t.Errorf("kind %q, want %q", got, want)
		}
	}
}
