package serieshistory_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/serieshistory"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeLoader struct{ plans []sources.FetchPlan }

func (l *fakeLoader) Load(_ context.Context, p sources.FetchPlan) ([]serieshistory.SeriesLoaded, error) {
	l.plans = append(l.plans, p)
	out := make([]serieshistory.SeriesLoaded, len(p.SeriesIDs))
	for i, id := range p.SeriesIDs {
		out[i] = serieshistory.SeriesLoaded{SeriesID: id, Fetched: i + 1}
	}
	return out, nil
}

func TestNewSeriesHistoryFuncContract(t *testing.T) {
	t.Parallel()
	fl := &fakeLoader{}
	var newLoader serieshistory.NewSeriesHistoryFunc = func(sources.Source, store.SeriesWriter) (serieshistory.SeriesHistoryLoader, error) {
		return fl, nil
	}
	l, err := newLoader(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := l.Load(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10", "VIXCLS"}, From: "2025-01-01", To: "2025-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1] != (serieshistory.SeriesLoaded{SeriesID: "VIXCLS", Fetched: 2}) || len(fl.plans) != 1 {
		t.Fatalf("got %+v", got)
	}
}
