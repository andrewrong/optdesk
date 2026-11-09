// Package household_page builds the page.household_page.v1 view model of one account per page: the
// blocks that need only the account summary values and the replayed option ledger are built, the
// blocks that need the price of a position are the gap missing_quote. It computes no figure (the
// figures come from blockapi.PageFigures), pools no account, reads no clock and makes no request.
package household_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Config is what the builder needs. Zone is the owner's zone (nil is an error of Build); Aliases are
// the configured account aliases, the first is the main account. A nil reader or constructor gives
// the gap page no_reader; a reader error is an error of Build, never a gap.
type Config struct {
	RunID, BuildID string
	Zone           *time.Location
	Aliases        []string
	Account        store.AccountReader
	Ledger         store.OptionLedgerReader
	Sessions       store.SessionDayReader
	Ledgers        compute.NewOptionLedgerFunc
	Figures        blockapi.NewPageFiguresFunc
}

// NewPage is the household_page page of one account; alias "" means the main account.
func NewPage(c Config, alias string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "household_page", Schema: "page.household_page.v1"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, alias, day, at)
		},
	}
}
