package optionshortlistapi_test

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
)

// fakeShortlister answers a fixed made-up result and records nothing.
type fakeShortlister struct{ res optionshortlistapi.Result }

func (f fakeShortlister) Select(in optionshortlistapi.Input) (optionshortlistapi.Result, error) {
	r := f.res
	r.Scanned = len(in.Rows)
	return r, nil
}

var _ optionshortlistapi.Shortlister = fakeShortlister{}

func TestShortlisterContractBothSides(t *testing.T) {
	t.Parallel()
	want := optionshortlistapi.Result{
		Groups: []optionshortlistapi.Group{{
			Company: "AAA", Best: "0.21",
			Rows: []optionshortlistapi.Placed{{Key: "k1", OTM: "6.1", AbsDelta: "0.22", Liquidity: "wide_spread"}},
		}},
		Dropped:    []optionshortlistapi.Drop{{Code: "no_bid", N: 2}, {Code: "delta_out", N: 1}},
		Scanned:    4,
		Shown:      1,
		StaleChain: 0,
		DTEMin:     "20",
		DTEMax:     "50",
		Bounds: optionshortlistapi.Bounds{
			OTMMinPct: "5", OTMMaxPct: "15", DeltaMin: "0.15", DeltaMax: "0.30", PerUnderlying: 3,
		},
	}
	var s optionshortlistapi.Shortlister = fakeShortlister{res: want}
	in := optionshortlistapi.Input{Rows: []optionshortlistapi.Row{{Key: "k1"}, {Key: "k2"}, {Key: "k3"}, {Key: "k4"}}}
	got, err := s.Select(in)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result changed in transit:\n got %+v\nwant %+v", got, want)
	}
}
