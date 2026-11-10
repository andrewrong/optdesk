package option_boards

import (
	"context"
	"encoding/json"
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

func TestConfigNamesField(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	day := contracts.Day("2026-01-02")
	for _, names := range []store.InstrumentNameReader{fakeNames{}, nil} {
		oc, sc := optionConfig(), stockConfig()
		oc.Names, sc.Names = names, names
		for name, build := range map[string]func() ([]byte, error){
			"BuildOption":   func() ([]byte, error) { return BuildOption(context.Background(), oc, nil, day, at) },
			"NewOptionPage": func() ([]byte, error) { return NewOptionPage(oc).Build(context.Background(), nil, day, at) },
			"BuildStock":    func() ([]byte, error) { return BuildStock(context.Background(), sc, nil, day, at) },
			"NewStockPage":  func() ([]byte, error) { return NewStockPage(sc).Build(context.Background(), nil, day, at) },
		} {
			out, err := build()
			if err != nil || !json.Valid(out) {
				t.Fatalf("%s (names %T) = %q, %v", name, names, out, err)
			}
		}
	}
}
