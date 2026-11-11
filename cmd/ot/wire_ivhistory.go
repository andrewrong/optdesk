package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute/ivbar"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/ivhistory"
)

// Design: docs/design/compute-stage2.md#8-iv-history-wire (Decision 11).

// jobIvHistory is the id of the iv_history job.
const jobIvHistory = "iv_history"

func init() {
	registerJob(jobFamily{
		id: jobIvHistory, order: 150,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			h := ivHistoryClient(ctx, jc)
			if h == nil {
				if jc.f.d.newHistory != nil && jc.f.compute.Readings != nil {
					a.log.Warn("job left out", "component", "ot", "job", jobIvHistory, "status", reasonBrokerDown)
				}
				return nil, "", nil
			}
			j, err := jc.f.ivHistory(h, jc.brk)
			return j, "", err
		},
	})
}

// ivHistoryClient is the history client the job gets, or nil when the job is left out. It is the client that
// bars_refresh and fx_refresh hold (one dial for the three jobs); when their step did not run it is dialled here,
// and closed through the closer set. With the broker down, or a bar line that left bars_refresh on its stand-in
// client, or without the history constructor or the reading store, there is none.
func ivHistoryClient(ctx context.Context, jc *jobCtx) broker.HistoryClient {
	f := jc.f
	if f.d.newHistory == nil || f.compute.Readings == nil || !jc.brokerUp {
		return nil
	}
	if f.hist != nil {
		if _, stub := f.hist.(unreachableHistory); stub {
			return nil
		}
		return f.hist
	}
	c, err := f.d.newHistory(ctx, jc.cfg.socketPath, broker.Options{})
	if err != nil {
		return nil
	}
	jc.extra.addCloser(c)
	return c
}

// ivHistory builds iv_history over the history client, the reading store opened for compute_figures and the
// watch list; the close of a bar becomes the stored text through the ivbar package.
func (f jobFactory) ivHistory(h broker.HistoryClient, b broker.Broker) (contracts.Job, error) {
	j, err := ivhistory.New(ivHistoryResolving(h, b), f.compute.Readings, f.compute.Readings, f.watch, ivbar.FromClose, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build iv_history job failed: %w", err)
	}
	return j, nil
}

// resolvingHistory gives a query whose contract has no con_id the one the broker names, as bars_refresh does:
// one ResolveContract per contract for the life of the client. pyside refuses a bar query without a con_id.
type resolvingHistory struct {
	broker.HistoryClient
	b   broker.Broker
	mu  sync.Mutex
	ids map[broker.Contract]int64
}

func ivHistoryResolving(h broker.HistoryClient, b broker.Broker) *resolvingHistory {
	return &resolvingHistory{HistoryClient: h, b: b, ids: map[broker.Contract]int64{}}
}

// notListedError is the Permanent answer for a contract the broker does not list.
type notListedError struct{ symbol string }

func (e notListedError) Error() string           { return "iv_history: " + e.symbol + " is not listed" }
func (notListedError) Kind() contracts.ErrorKind { return contracts.Permanent }

// Bars reads the bars of q, with the con_id of its contract resolved when it has none.
func (r *resolvingHistory) Bars(ctx context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	if q.Contract.ConID == 0 {
		id, err := r.conID(ctx, q.Contract)
		if err != nil {
			return nil, err
		}
		q.Contract.ConID = id
	}
	return r.HistoryClient.Bars(ctx, q)
}

func (r *resolvingHistory) conID(ctx context.Context, c broker.Contract) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.ids[c]; ok {
		return id, nil
	}
	got, err := r.b.ResolveContract(ctx, c)
	if err != nil {
		return 0, err
	}
	if !got.Listed || got.Contract.ConID == 0 {
		return 0, notListedError{symbol: c.Symbol}
	}
	r.ids[c] = got.Contract.ConID
	return got.Contract.ConID, nil
}
