// Package option_boards builds the option page (board 5) and the one-stock option page (board 18) from stored
// candidate rows. Both builders live here because they share the candidate lookback, the gate JSON, the decimal
// text and the envelope.
package option_boards

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// OptionConfig is what BuildOption reads. A nil Zone is an error of BuildOption.
// Names is the ticker source of the display name; nil gives the stored issuer text as the display name.
type OptionConfig struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location
	Candidates            store.CandidateReader
	Eod                   store.OptionEodReader
	Names                 store.InstrumentNameReader
}

// StockConfig is what BuildStock reads. Key is the ticker or a stored issuer text of the company shown; Names is the ticker source.
// A nil Zone is an error of BuildStock.
type StockConfig struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location
	Candidates            store.CandidateReader
	Eod                   store.OptionEodReader
	Names                 store.InstrumentNameReader
	Key                   string
}

// NewOptionPage is the option_page page for c; its Build ignores the series reader and reads those of c.
func NewOptionPage(c OptionConfig) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "option_page", Schema: "page.option_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
			return BuildOption(ctx, c, r, day, generatedAt)
		},
	}
}

// NewStockPage is the stock_option_page page for c; its Build ignores the series reader and reads those of c.
func NewStockPage(c StockConfig) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "stock_option_page", Schema: "page.stock_option_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
			return BuildStock(ctx, c, r, day, generatedAt)
		},
	}
}
