package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/store"
)

// ivHistoryHistory answers one bar for every query and records the kinds asked.
type ivHistoryHistory struct {
	mu     sync.Mutex
	kinds  []broker.BarKind
	conIDs []int64
}

func (h *ivHistoryHistory) Bars(_ context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.kinds = append(h.kinds, q.Kind)
	h.conIDs = append(h.conIDs, q.Contract.ConID)
	return []broker.Bar{{Day: "2026-11-09", Open: "0.27", High: "0.30", Low: "0.26", Close: "0.281"}}, nil
}
func (*ivHistoryHistory) Close() error { return nil }

// ivHistoryReadings is an in-memory reading store.
type ivHistoryReadings struct {
	mu   sync.Mutex
	rows []contracts.ReadingRecord
}

func (s *ivHistoryReadings) LatestReadings(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
	return nil, nil
}

func (s *ivHistoryReadings) InsertReadings(_ context.Context, rows []contracts.ReadingRecord) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, rows...)
	return len(rows), nil
}

func (s *ivHistoryReadings) Close() error { return nil }

func ivHistoryFactory(t *testing.T, rd *ivHistoryReadings) jobFactory {
	t.Helper()
	return jobFactory{
		clk:     fakeClock{time.Date(2026, 11, 9, 23, 0, 0, 0, time.UTC)},
		compute: computefigures.ComputeStores{Readings: store.ReadingStore(rd)},
		watch:   []jobs.WatchEntry{{Market: "US", Contract: broker.Contract{SecType: "STK", Symbol: "AAPL", ConID: 265598}}},
	}
}

func TestIvHistoryFamilyPlans(t *testing.T) {
	t.Parallel()
	f := ivHistoryFactory(t, &ivHistoryReadings{})
	j, err := f.ivHistory(&ivHistoryHistory{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if j.ID() != jobIvHistory {
		t.Fatalf("id %q", j.ID())
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].SubjectKey != "market.iv_ibkr|underlying|AAPL" {
		t.Fatalf("keys %+v", keys)
	}
}

func TestIvHistoryFamilyStores(t *testing.T) {
	t.Parallel()
	rd := &ivHistoryReadings{}
	h := &ivHistoryHistory{}
	f := ivHistoryFactory(t, rd)
	j, err := f.ivHistory(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), "2026-11-09", keys); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.kinds) != 1 || h.kinds[0] != broker.BarOptionImpliedVol {
		t.Fatalf("kinds %v", h.kinds)
	}
	if len(rd.rows) != 1 || rd.rows[0].Name != "market.iv_ibkr" || rd.rows[0].Subject != "AAPL" ||
		rd.rows[0].Value == nil || *rd.rows[0].Value != "0.281000" {
		t.Fatalf("rows %+v", rd.rows)
	}
}

// ivHistoryBroker answers ResolveContract from a fixed answer and counts the calls.
type ivHistoryBroker struct {
	broker.Broker
	mu    sync.Mutex
	calls int
	got   broker.Resolved
	err   error
}

func (b *ivHistoryBroker) ResolveContract(context.Context, broker.Contract) (broker.Resolved, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	return b.got, b.err
}

type ivHistoryTyped struct{ kind contracts.ErrorKind }

func (e ivHistoryTyped) Error() string             { return "typed" }
func (e ivHistoryTyped) Kind() contracts.ErrorKind { return e.kind }

func jpContract() broker.Contract {
	return broker.Contract{SecType: "STK", Symbol: "7203", Exchange: "TSEJ", Currency: "JPY"}
}

func TestIvHistoryResolvesMissingConID(t *testing.T) {
	t.Parallel()
	b := &ivHistoryBroker{got: broker.Resolved{Listed: true, Contract: broker.Contract{ConID: 4321}}}
	h := &ivHistoryHistory{}
	c := ivHistoryResolving(h, b)
	ctx := context.Background()
	for range 2 {
		if _, err := c.Bars(ctx, broker.BarQuery{Contract: jpContract()}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Bars(ctx, broker.BarQuery{Contract: broker.Contract{Symbol: "AAPL", ConID: 265598}}); err != nil {
		t.Fatal(err)
	}
	if b.calls != 1 {
		t.Fatalf("resolve calls %d", b.calls)
	}
	want := []int64{4321, 4321, 265598}
	if len(h.conIDs) != 3 || h.conIDs[0] != want[0] || h.conIDs[1] != want[1] || h.conIDs[2] != want[2] {
		t.Fatalf("con ids %v", h.conIDs)
	}
}

func TestIvHistoryNotListedIsPermanent(t *testing.T) {
	t.Parallel()
	for _, got := range []broker.Resolved{{Listed: false}, {Listed: true}} {
		b := &ivHistoryBroker{got: got}
		h := &ivHistoryHistory{}
		_, err := ivHistoryResolving(h, b).Bars(context.Background(), broker.BarQuery{Contract: jpContract()})
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent || !strings.Contains(err.Error(), "7203 is not listed") {
			t.Fatalf("err %v", err)
		}
		if len(h.conIDs) != 0 {
			t.Fatalf("Bars called: %v", h.conIDs)
		}
	}
}

func TestIvHistoryResolveErrorStaysRetryable(t *testing.T) {
	t.Parallel()
	want := ivHistoryTyped{contracts.Retryable}
	b := &ivHistoryBroker{err: want}
	h := &ivHistoryHistory{}
	_, err := ivHistoryResolving(h, b).Bars(context.Background(), broker.BarQuery{Contract: jpContract()})
	if err != want {
		t.Fatalf("err %v", err)
	}
	if len(h.conIDs) != 0 {
		t.Fatal("Bars called")
	}
}

func TestIvHistoryJobResolvesJPEntry(t *testing.T) {
	t.Parallel()
	rd := &ivHistoryReadings{}
	h := &ivHistoryHistory{}
	b := &ivHistoryBroker{got: broker.Resolved{Listed: true, Contract: broker.Contract{ConID: 4321}}}
	f := ivHistoryFactory(t, rd)
	f.watch = []jobs.WatchEntry{{Market: "JP", Contract: jpContract()}}
	j, err := f.ivHistory(h, b)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), "2026-11-09", keys); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.conIDs) != 1 || h.conIDs[0] != 4321 {
		t.Fatalf("con ids %v", h.conIDs)
	}
}
