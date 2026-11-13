package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/nyfed"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

type emptySource struct{}

func (emptySource) Fetch(context.Context, sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	return nil, nil
}

// registrySeries names SOFR as backed up by nyfed and records the filled rows.
type registrySeries struct {
	store.SeriesStore
	filled atomic.Int32
}

func (*registrySeries) InsertSeriesPoints(context.Context, []contracts.SeriesPoint) error { return nil }

func (*registrySeries) ActiveSeries(context.Context) ([]seriesdef.SeriesDefinition, error) {
	return []seriesdef.SeriesDefinition{{SeriesID: "SOFR", SecondarySource: "nyfed", Active: true}}, nil
}

func (r *registrySeries) FillSeriesPoints(_ context.Context, rows []contracts.SeriesPoint, _ string) (seriesdef.WriteCount, error) {
	r.filled.Add(int32(len(rows)))
	return seriesdef.WriteCount{Written: len(rows)}, nil
}

func TestFredJobIsBuiltWithSecondSource(t *testing.T) {
	t.Parallel()
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked.Add(1)
		_, _ = w.Write([]byte(`{"refRates":[{"effectiveDate":"2026-11-05","percentRate":4.3}]}`))
	}))
	t.Cleanup(srv.Close)
	clk := fakeClock{time.Date(2026, 11, 5, 20, 0, 0, 0, time.UTC)}
	ser := &registrySeries{}
	f := jobFactory{src: emptySource{}, ser: ser, clk: clk}
	j, err := f.fredWithSecond(nyfed.Config{BaseURL: srv.URL, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	keys := []contracts.ExpectedKey{{DatasetID: "obs.series_point", SubjectKey: "SOFR"}}
	if res := j.Run(context.Background(), "2026-11-05", keys); res.Err != nil {
		t.Fatalf("run: %+v", res)
	}
	if asked.Load() != 1 || ser.filled.Load() != 1 {
		t.Fatalf("second source asked %d times, filled %d rows", asked.Load(), ser.filled.Load())
	}
}
