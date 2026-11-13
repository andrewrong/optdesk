package optioneod

import (
	"context"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ NewOptionEodIndexFunc = NewIndex

// NewIndex is the stub of the option_eod_index constructor: it builds a job whose Run ends Permanent until the
// index chains are built.
func NewIndex(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, IndexCloseReader, clock.Clock) (contracts.Job, error) {
	return indexStub{}, nil
}

type indexStub struct{}

func (indexStub) ID() string { return "option_eod_index" }

func (indexStub) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}

func (indexStub) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{Err: newErr(contracts.Permanent, "optioneod: index chains are not built yet")}
}
