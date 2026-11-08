// Package gonumstats is compute.Stats over gonum.
package gonumstats

import (
	"math"

	"gonum.org/v1/gonum/stat"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// permanent is the TypedError of a bad input.
type permanent string

func (e permanent) Error() string             { return "gonumstats: " + string(e) }
func (e permanent) Kind() contracts.ErrorKind { return contracts.Permanent }

type statsImpl struct{}

// New is a compute.NewStatsFunc. Empty input, NaN or Inf in the input, one value for the
// standard deviation and an unknown Compare are Permanent errors; no result is NaN.
func New() compute.Stats { return statsImpl{} }

func finite(values []float64, min int) error {
	if len(values) < min {
		return permanent("too few values")
	}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return permanent("non-finite value")
		}
	}
	return nil
}

// result turns an overflowed (Inf) or NaN result into a Permanent error.
func result(v float64) (float64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, permanent("result is not finite")
	}
	return v, nil
}

func (statsImpl) Mean(values []float64) (float64, error) {
	if err := finite(values, 1); err != nil {
		return 0, err
	}
	return result(stat.Mean(values, nil))
}

// StdDev is the sample standard deviation (ddof 1): stat.StdDev divides by n-1.
func (statsImpl) StdDev(values []float64) (float64, error) {
	if err := finite(values, 2); err != nil {
		return 0, err
	}
	return result(stat.StdDev(values, nil))
}

// PercentileRank counts the values below x, or at or below x for AtOrBelow (the tie rule
// of the glossary); gonum has no such rank, so the count is ours.
func (statsImpl) PercentileRank(values []float64, x float64, cmp compute.Compare) (float64, error) {
	if cmp != compute.Below && cmp != compute.AtOrBelow {
		return 0, permanent("unknown compare")
	}
	if err := finite(values, 1); err != nil {
		return 0, err
	}
	if math.IsNaN(x) {
		return 0, permanent("x is NaN")
	}
	n := 0
	for _, v := range values {
		if v < x || (cmp == compute.AtOrBelow && v == x) {
			n++
		}
	}
	return float64(n) / float64(len(values)), nil
}
