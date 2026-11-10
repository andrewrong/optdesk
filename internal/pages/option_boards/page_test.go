package option_boards

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeCandidates struct{}

func (fakeCandidates) Candidates(context.Context, contracts.Day, string) ([]store.Candidate, error) {
	return []store.Candidate{}, nil
}

type fakeEod struct{}

func (fakeEod) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	return nil, nil
}

type fakeNames struct{}

func (fakeNames) InstrumentNames(context.Context, []int64) ([]store.InstrumentName, error) {
	return nil, nil
}

var testAt = time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)

func optionConfig() OptionConfig {
	return OptionConfig{
		RunID: "run", BuildID: "build", Scope: "options", Zone: time.UTC,
		Candidates: fakeCandidates{}, Eod: fakeEod{},
	}
}

func stockConfig() StockConfig {
	return StockConfig{
		RunID: "run", BuildID: "build", Scope: "stock-options", Zone: time.UTC,
		Candidates: fakeCandidates{}, Eod: fakeEod{}, Names: fakeNames{}, Key: "AAPL",
	}
}

func TestPageMeta(t *testing.T) {
	t.Parallel()
	if got, want := NewOptionPage(optionConfig()).Meta, (pages.Meta{Name: "option_page", Schema: "page.option_page.v1"}); got != want {
		t.Fatalf("option meta = %+v", got)
	}
	if got, want := NewStockPage(stockConfig()).Meta, (pages.Meta{Name: "stock_option_page", Schema: "page.stock_option_page.v1"}); got != want {
		t.Fatalf("stock meta = %+v", got)
	}
}

func TestBuildNotBuilt(t *testing.T) {
	t.Parallel()
	const want = "option_boards: not built"
	if _, err := BuildOption(t.Context(), optionConfig(), nil, "2026-11-10", testAt); err == nil || err.Error() != want {
		t.Fatalf("BuildOption err = %v", err)
	}
	if _, err := BuildStock(t.Context(), stockConfig(), nil, "2026-11-10", testAt); err == nil || err.Error() != want {
		t.Fatalf("BuildStock err = %v", err)
	}
	if _, err := NewOptionPage(optionConfig()).Build(t.Context(), nil, "2026-11-10", testAt); err == nil || err.Error() != want {
		t.Fatalf("option page Build err = %v", err)
	}
	if _, err := NewStockPage(stockConfig()).Build(t.Context(), nil, "2026-11-10", testAt); err == nil || err.Error() != want {
		t.Fatalf("stock page Build err = %v", err)
	}
}
