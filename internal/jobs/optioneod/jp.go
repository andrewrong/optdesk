package optioneod

import (
	_ "embed"
	"errors"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ jobs.NewOptionEodFunc = NewJP

var _ jobs.NewOptionEodWatchFunc = NewJPWithWatch

//go:embed job_jp.toml
var manifestJPBytes []byte

// NewJP builds the job option_eod_jp from the embedded job_jp.toml.
func NewJP(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, clk clock.Clock) (contracts.Job, error) {
	if b == nil || res == nil || w == nil || clk == nil {
		return nil, errors.New("optioneod: broker, resolver, writer and clock are required")
	}
	return build(manifestJPBytes, b, res, w, clk)
}

// NewJPWithWatch is NewJP with one more key per JP stock entry of watch (market JP, sec_type STK), taking the stock
// values from the [stock] table of job_jp.toml. The plan lists N225 first, then the codes sorted, so the stocks never
// starve the index; no JP entry gives the job of NewJP; a repeated code is an error.
func NewJPWithWatch(
	b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, watch []jobs.WatchEntry, clk clock.Clock,
) (contracts.Job, error) {
	if b == nil || res == nil || w == nil || clk == nil {
		return nil, errors.New("optioneod: broker, resolver, writer and clock are required")
	}
	j, err := build(manifestJPBytes, b, res, w, clk)
	if err != nil {
		return nil, err
	}
	us, buy, err := jpWatchUnderlyings(watch, j.m.Stock)
	if err != nil {
		return nil, err
	}
	j.m.Underlying = append(j.m.Underlying, us...)
	j.m.buy = buy
	if err := j.m.validate(); err != nil {
		return nil, err
	}
	return j, nil
}
