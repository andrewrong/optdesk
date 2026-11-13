package optioneod

import (
	"context"
	_ "embed"
	"errors"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ NewOptionEodIndexFunc = NewIndex

//go:embed job_index.toml
var manifestIndexBytes []byte

// closeLookbackDays is how far back the closes are read: enough for sigma_days returns over holidays.
const closeLookbackDays = 70

// NewIndex builds the job option_eod_index from the embedded job_index.toml. The reader may be nil: the job then
// uses sigma_fallback and has no stored-close spot. A nil broker, resolver, writer or clock is an error naming it.
func NewIndex(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, closes IndexCloseReader, clk clock.Clock) (contracts.Job, error) {
	var missing []string
	if b == nil {
		missing = append(missing, "broker")
	}
	if res == nil {
		missing = append(missing, "resolver")
	}
	if w == nil {
		missing = append(missing, "writer")
	}
	if clk == nil {
		missing = append(missing, "clock")
	}
	if len(missing) > 0 {
		return nil, errors.New("optioneod: option_eod_index needs a " + joinAnd(missing))
	}
	j, err := build(manifestIndexBytes, b, res, w, clk)
	if err != nil {
		return nil, err
	}
	j.closes = closes
	return j, nil
}

func joinAnd(s []string) string {
	out := s[0]
	for _, x := range s[1:] {
		out += " and " + x
	}
	return out
}

// closesOf reads the stored closes of index from 70 calendar days before the day up to the day; nil without a reader.
func (r *runState) closesOf(ctx context.Context, index string) ([]string, error) {
	if r.j.closes == nil {
		return nil, errors.New("no close reader")
	}
	from := r.dayTime.AddDate(0, 0, -closeLookbackDays).Format(time.DateOnly)
	return r.j.closes.IndexCloses(ctx, index, contracts.Day(from), r.day)
}

// storedSpot is the last stored close of the row's spot_index over spot_divisor; false when there is none.
func (r *runState) storedSpot(ctx context.Context, row indexRow) (float64, bool) {
	closes, err := r.closesOf(ctx, row.SpotIndex)
	if err != nil || len(closes) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(closes[len(closes)-1], 64)
	if err != nil || !(v > 0) {
		return 0, false
	}
	return v / float64(row.SpotDivisor), true
}

// sigma is the annualised deviation of the closes of the row's sigma_index, sigma_fallback when they cannot give
// one; a reader error never fails the key.
func (r *runState) sigma(ctx context.Context, row indexRow) float64 {
	closes, err := r.closesOf(ctx, row.SigmaIndex)
	if err != nil {
		return r.j.m.SigmaFallback
	}
	return annualisedSigma(closes, r.j.m.SigmaDays, r.j.m.SigmaFallback)
}
