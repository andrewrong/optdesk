package market_page_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/market_page"
)

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := market_page.NewPage(market_page.Config{})
	if p.Meta.Name != "market_page" || p.Meta.Schema != "page.market_page.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

type noSeries struct{}

func (noSeries) SeriesPoints(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SeriesPoint, error) {
	return nil, nil
}

func TestConfigContract(t *testing.T) {
	t.Parallel()
	var readings compute.ReadingFunc = func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, nil
	}
	p := market_page.NewPage(market_page.Config{
		RunID: "r", BuildID: "b", Scope: "s",
		Zone: time.UTC, Readings: readings,
	})
	out, err := p.Build(context.Background(), noSeries{}, contracts.Day("2026-01-02"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil || m["schema"] != "market_page" || m["run_id"] != "r" {
		t.Fatalf("out = %s, %v", out, err)
	}
}
