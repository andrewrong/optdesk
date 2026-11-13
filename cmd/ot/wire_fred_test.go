package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/fredseries"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

// registrySeries is a series store whose registry read is counted and fails, so a run that asks for the
// registry reaches no second source and makes no call.
type registrySeries struct {
	store.SeriesStore
	reads atomic.Int32
}

func (*registrySeries) Close() error { return nil }

func (s *registrySeries) ActiveSeries(context.Context) ([]seriesdef.SeriesDefinition, error) {
	s.reads.Add(1)
	return nil, errors.New("registry not read in this test")
}

type emptySource struct{}

func (emptySource) Fetch(context.Context, sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	return nil, nil
}

// TestFredBuiltWithSecondSource builds the job through fred(), the path ot runs: the job reads the registry
// for a gap only when it has a second source.
func TestFredBuiltWithSecondSource(t *testing.T) {
	t.Parallel()
	ser := &registrySeries{}
	f := jobFactory{d: deps{newFredSeries: fredseries.New}, clk: fakeClock{time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)}, src: emptySource{}, ser: ser}
	j, err := f.fred()
	if err != nil {
		t.Fatal(err)
	}
	keys := []contracts.ExpectedKey{{DatasetID: "obs.series_point", SubjectKey: "DGS10"}}
	j.Run(context.Background(), "2026-11-05", keys)
	if ser.reads.Load() == 0 {
		t.Fatal("the fred_series job built by fred() did not look for a gap to fill: no second source")
	}
}
