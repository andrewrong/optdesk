package gonumstats_test

import (
	"errors"
	"math"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/contracttest"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ compute.NewStatsFunc = gonumstats.New

func TestContract(t *testing.T) {
	t.Parallel()
	contracttest.Stats(contracttest.Wrap(t), gonumstats.New())
}

func TestPercentileRankBothWays(t *testing.T) {
	t.Parallel()
	s := gonumstats.New()
	v := []float64{1, 2, 2, 2, 3}
	cases := []struct {
		name string
		x    float64
		cmp  compute.Compare
		want float64
	}{
		{"tie below", 2, compute.Below, 0.2},
		{"tie at or below", 2, compute.AtOrBelow, 0.8},
		{"under the range below", 0, compute.Below, 0},
		{"under the range at or below", 0, compute.AtOrBelow, 0},
		{"over the range below", 4, compute.Below, 1},
		{"over the range at or below", 4, compute.AtOrBelow, 1},
		{"max below", 3, compute.Below, 0.8},
		{"max at or below", 3, compute.AtOrBelow, 1},
	}
	for _, c := range cases {
		got, err := s.PercentileRank(v, c.x, c.cmp)
		if err != nil || math.Abs(got-c.want) > 1e-12 {
			t.Errorf("%s: got %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestBadInput(t *testing.T) {
	t.Parallel()
	s := gonumstats.New()
	nan := math.NaN()
	check := func(name string, v float64, err error) {
		t.Helper()
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("%s: err = %v, want Permanent TypedError", name, err)
		}
		if v != 0 || math.IsNaN(v) {
			t.Errorf("%s: value = %v, want 0", name, v)
		}
	}
	v, err := s.Mean(nil)
	check("mean empty", v, err)
	v, err = s.StdDev(nil)
	check("std empty", v, err)
	v, err = s.StdDev([]float64{5})
	check("std one value", v, err)
	v, err = s.PercentileRank(nil, 1, compute.Below)
	check("rank empty", v, err)
	v, err = s.Mean([]float64{1, nan})
	check("mean nan", v, err)
	v, err = s.StdDev([]float64{1, nan})
	check("std nan", v, err)
	v, err = s.PercentileRank([]float64{1, nan}, 1, compute.Below)
	check("rank nan value", v, err)
	v, err = s.PercentileRank([]float64{1, 2}, nan, compute.Below)
	check("rank nan x", v, err)
	v, err = s.PercentileRank([]float64{1, 2}, 1, compute.Compare("sideways"))
	check("rank bad compare", v, err)
}
