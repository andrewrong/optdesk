// Package household_page builds the page.household_page.v1 view model. Build and NewPage give the page
// of one account: the blocks that need only the account summary values and the replayed option ledger
// are built, the blocks that need the price of a position are the gap missing_quote. BuildFamily gives
// the family page of a book of two or more aliases: one row per account and the total of
// blockapi.FamilyFigure, with a named line for every account the total does not hold. It computes no
// figure (the figures come from blockapi.PageFigures and blockapi.FamilyFigure), reads no clock and
// makes no request.
package household_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/underlying"
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
	// Marks and Holdings together build composition, allocation and concentration from the
	// positions of the capture day; either nil leaves them the gap missing_quote.
	Marks    mark.Reader
	Holdings blockapi.NewHoldingFiguresFunc
	// Fx reads the stored USDJPY rate of the capture day, only when the capture holds JPY; nil, not
	// found and an error all leave the JPY rows in yen with a gap.
	Fx bar.FxRateReader
	// Statement lists the aliases whose capture comes from the broker statement; Build adds the note
	// "From the broker statement of <data day>" to the page of such an alias when a capture was found.
	// Read only by Build.
	Statement []string
	// Family is the figure of the family page; nil gives BuildFamily the gap page no_reader.
	Family blockapi.NewFamilyFigureFunc
	// FamilyTrend is the figure of the family trend block; nil leaves the block the gap not_built.
	FamilyTrend blockapi.NewFamilyTrendFigureFunc
	// Underlying, Earnings and Held are read only by BuildHeld: the underlying rows (company and issuer of a
	// symbol), the stored earnings rows of an issuer and the figure of the held companies. A nil one gives
	// the page gaps no_reader.
	Underlying underlying.Reader
	Earnings   store.EarningsDateReader
	Held       blockapi.HeldFigure
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
