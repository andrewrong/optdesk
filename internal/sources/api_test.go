package sources_test

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

type fakeSource struct{}

func (fakeSource) Fetch(_ context.Context, p sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, id := range p.SeriesIDs {
		out = append(out, contracts.SeriesPoint{SeriesID: id, Source: "fake"})
	}
	return out, nil
}

var _ sources.NewFREDFunc = func(cfg sources.FREDConfig) (sources.Source, error) { return fakeSource{}, nil }

func TestSourceContract(t *testing.T) {
	t.Parallel()
	newSource := sources.NewFREDFunc(func(sources.FREDConfig) (sources.Source, error) { return fakeSource{}, nil })
	src, err := newSource(sources.FREDConfig{BaseURL: "http://unused.invalid", Clock: clockwork.NewFakeClock()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2026-11-01", To: "2026-11-04"})
	if err != nil || len(got) != 1 || got[0].SeriesID != "DGS10" {
		t.Fatalf("got %+v err %v", got, err)
	}
}
