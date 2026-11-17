// Package option_pnl_page builds the view model of the options P&L page.
package option_pnl_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// StockIDReader reads the instruments of type stock whose name is in names; a name returned once is one
// instrument, a name returned twice is shared and gives no price.
type StockIDReader interface {
	StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
}

// Config is what the option_pnl_page builder needs besides the series reader.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location // generated_at is written in it
	Ledger                store.OptionLedgerReader
	Sessions              store.SessionDayReader
	Days                  optionday.DayReader
	StockIDs              StockIDReader
	Bars                  bar.PriceBarReader
	Replay                compute.OptionLedger
	Attributor            optionattrapi.Attributor
	Booker                optionattrapi.Booker
	MarkMaxAgeSessions    int
	RecentDays            int
	MaxRateAgeDays        int
	CalendarID            string
}

// NewPage is the option_pnl_page page for c.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "option_pnl_page", Schema: "page.option_pnl_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}
