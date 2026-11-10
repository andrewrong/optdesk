// Package positions_page builds the page.positions_page.v3 view model of one account per page: the
// stocks and options the account holds at its newest capture with the price of each (the stored mark),
// the remaining premium of each option from the replayed ledger, and the executions of the last 30
// session days. It computes no figure (the figures come from blockapi.HoldingFigures and
// blockapi.PageFigures), pools no account, reads no clock and makes no request.
package positions_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// Config is what the builder needs. Zone is the owner's zone (nil is an error of Build); Aliases are the
// configured account aliases, the first is the main account. A nil reader or constructor gives the gap
// page no_reader; a reader error is an error of Build, never a gap.
type Config struct {
	RunID, BuildID string
	Zone           *time.Location
	Aliases        []string
	Account        store.AccountReader
	Marks          mark.Reader
	Ledger         store.OptionLedgerReader
	Sessions       store.SessionDayReader
	Ledgers        compute.NewOptionLedgerFunc
	Figures        blockapi.NewPageFiguresFunc
	Holdings       blockapi.NewHoldingFiguresFunc
}

// NewPage is the positions_page page of one account; alias "" means the main account.
func NewPage(c Config, alias string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "positions_page", Schema: "page.positions_page.v3"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, alias, day, at)
		},
	}
}
