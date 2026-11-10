package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7.

func init() {
	registerJob(jobFamily{
		id: jobQuoteSnapshot, order: 60, startOff: brokerDownOff,
		build: func(_ context.Context, _ *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.quoteSnapshot(jc.brk)
			return j, "", err
		},
	})
}

func (f jobFactory) quoteSnapshot(b broker.Broker) (contracts.Job, error) {
	j, err := f.d.newQuoteSnapshot(b, f.inst, f.snap, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build quote_snapshot job failed: %w", err)
	}
	return j, nil
}
