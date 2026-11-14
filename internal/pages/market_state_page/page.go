// Package market_state_page builds the view model of the market state page.
package market_state_page

import (
	"context"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// Groups of a Symbol.
const (
	GroupSector   = "sector"
	GroupIndustry = "industry"
)

// Symbol is one listed fund: its ticker, its label and its group, sector or industry.
type Symbol struct{ Symbol, Label, Group string }

// StockIDReader reads the instruments of type stock whose name is in names.
type StockIDReader interface {
	StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
}

// Config is what the market_state_page builder needs besides the series reader. A nil Market,
// Liquidity, Readings, Bars or StockIDs gives the gap no_reader on the rows that need it, never
// a number and never an error. Figures, Units and Rules are copied into marketstateapi.Input.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location // generated_at is written in it; nil is an error of Build
	Readings              compute.ReadingFunc
	Bars                  bar.PriceBarReader
	StockIDs              StockIDReader
	Market                marketstateapi.Func
	Liquidity             liquidityapi.Func
	Figures               compute.Figures
	Units                 compute.Units
	Rules                 compute.Rules
	MarketParams          marketstateapi.Params
	LiquidityParams       liquidityapi.Params
	Symbols               []Symbol // the funds (group sector) and SMH (group industry), in page order
}

// NewPage is the market_state_page page for c; its Symbols are copied at construction.
func NewPage(c Config) pages.Page {
	c.Symbols = slices.Clone(c.Symbols)
	return pages.Page{
		Meta: pages.Meta{Name: "market_state_page", Schema: "page.market_state_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}
