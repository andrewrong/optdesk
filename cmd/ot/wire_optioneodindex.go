package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// Design: docs/design/option-capture-need.md#Issues (Decisions 5, 10, 11).

const jobOptionEodIndex = "option_eod_index"

const indexNoClosesLine = "option_eod_index runs without closes"

func init() {
	registerJob(jobFamily{
		id: jobOptionEodIndex, order: 75, startOff: brokerDownOff,
		build: func(_ context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.optionEodIndex(jc.brk, optioneod.NewIndex, a.log)
			return j, "", err
		},
	})
}

// optionEodIndex builds option_eod_index through newIndex over the stores of the factory. The close reader is
// built when both the price bar reader and the index id reader exist; otherwise the job gets a nil reader and
// log (when not nil) gets one Warn line.
func (f jobFactory) optionEodIndex(b broker.Broker, newIndex optioneod.NewOptionEodIndexFunc, log *slog.Logger) (contracts.Job, error) {
	var rd optioneod.IndexCloseReader
	if f.compute.PriceBars != nil && f.compute.IndexIDs != nil {
		rd = indexCloses{bars: f.compute.PriceBars, ids: f.compute.IndexIDs}
	} else if log != nil {
		log.Warn(indexNoClosesLine, "component", "ot", "job", jobOptionEodIndex)
	}
	j, err := newIndex(b, f.inst, f.eod, rd, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build %s job failed: %w", jobOptionEodIndex, err)
	}
	return withOptionEodReasons(j, nil, f.reasons)
}

// indexCloses is the IndexCloseReader over the price bar reader and the index id reader.
type indexCloses struct {
	bars bar.PriceBarReader
	ids  computefigures.IndexIDReader
}

// IndexCloses returns the Close texts of the trades rows of the named index from `from` to `to`, oldest first.
func (r indexCloses) IndexCloses(ctx context.Context, index string, from, to contracts.Day) ([]string, error) {
	found, err := r.ids.IndexIDs(ctx, []string{index})
	if err != nil {
		return nil, fmt.Errorf("index id of %s: %w", index, err)
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("index %s is unknown", index)
	case 1:
	default:
		return nil, fmt.Errorf("index %s is ambiguous: %d instruments", index, len(found))
	}
	rows, err := r.bars.PriceBars(ctx, found[0].InstrumentID, "trades", from, to)
	if err != nil {
		return nil, fmt.Errorf("closes of %s: %w", index, err)
	}
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Close
	}
	return out, nil
}
