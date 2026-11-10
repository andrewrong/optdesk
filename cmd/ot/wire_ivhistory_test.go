package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// ivHistoryHistory answers one bar for every query and records the kinds asked.
type ivHistoryHistory struct {
	mu    sync.Mutex
	kinds []broker.BarKind
}

func (h *ivHistoryHistory) Bars(_ context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.kinds = append(h.kinds, q.Kind)
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
		compute: jobs.ComputeStores{Readings: store.ReadingStore(rd)},
		watch:   []jobs.WatchEntry{{Market: "US", Contract: broker.Contract{SecType: "STK", Symbol: "AAPL", ConID: 265598}}},
	}
}

func TestIvHistoryFamilyPlans(t *testing.T) {
	t.Parallel()
	f := ivHistoryFactory(t, &ivHistoryReadings{})
	j, err := f.ivHistory(&ivHistoryHistory{})
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
	j, err := f.ivHistory(h)
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
