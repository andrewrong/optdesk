package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/fxrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

type fxSecondClock struct{}

func (fxSecondClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 40, 0, 0, time.UTC) }

type fxSecondBroker struct{ broker.Broker }

func (fxSecondBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	c.ConID = 9001
	return broker.Resolved{Listed: true, Contract: c}, nil
}

type fxSecondHistory struct{ broker.HistoryClient }

func (fxSecondHistory) Bars(context.Context, broker.BarQuery) ([]broker.Bar, error) { return nil, nil }

// fxSecondStore is a bar store that keeps the rows InsertFxRates is given.
type fxSecondStore struct {
	bar.BarStore
	mu   sync.Mutex
	rows []contracts.FxRate
}

func (s *fxSecondStore) InsertFxRates(_ context.Context, rows []contracts.FxRate) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, rows...)
	return len(rows), nil
}

// fxSecondFred is a FRED source that counts its calls.
type fxSecondFred struct {
	mu    sync.Mutex
	calls int
}

func (f *fxSecondFred) Fetch(context.Context, sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	v := 152.25
	d := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	return []contracts.SeriesPoint{{SeriesID: "DEXJPUS", ObservationDate: types.SerializableDate{Time: d}, Value: &v}}, nil
}

// runFx builds fx_refresh through the factory with src as its FRED source and runs it once.
func runFx(t *testing.T, src sources.Source) *fxSecondStore {
	t.Helper()
	st := &fxSecondStore{}
	f := jobFactory{d: deps{newFxRefresh: fxrefresh.New}, clk: fxSecondClock{}, src: src, bars: st}
	j, err := f.fxRefresh(fxSecondBroker{}, fxSecondHistory{})
	if err != nil {
		t.Fatal(err)
	}
	keys := []contracts.ExpectedKey{{DatasetID: "obs.fx_rate", SubjectKey: "USDJPY"}}
	j.Run(context.Background(), "2026-11-09", keys)
	return st
}

func TestFxRefreshGetsFredSourceWithKey(t *testing.T) {
	t.Parallel()
	fred := &fxSecondFred{}
	st := runFx(t, fred)
	if fred.calls != 1 {
		t.Fatalf("fred calls %d, want 1", fred.calls)
	}
	if len(st.rows) != 1 || st.rows[0].Source != "fred_h10" || st.rows[0].Rate != "152.25" {
		t.Fatalf("rows %+v", st.rows)
	}
}

func TestFxRefreshWithoutKeyHasNoFill(t *testing.T) {
	t.Parallel()
	st := runFx(t, noKeySource{})
	if len(st.rows) != 0 {
		t.Fatalf("rows %+v", st.rows)
	}
}
