// Package risk_page builds the page.household_risk.v1 view model of one account: the delta block (rows per
// underlying) and the vega block (rows per expiry) from blockapi.HoldingFigures.Risk over the stocks of the
// newest capture and the greeks of the option structures. The beta_delta, sigma and standard_metrics blocks
// are the gap not_built. It computes no figure, reads no clock and makes no request.
package risk_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Config is what the builder needs; the first fields are those of the positions page. Zone is the owner's
// zone (nil is an error of Build); Aliases are the configured account aliases, the first is the main
// account. Eod and Terms may be nil: the structures then have no quotes and their greeks are gaps. A nil
// other reader or constructor gives the gap page no_reader; a reader error is an error of Build.
type Config struct {
	RunID, BuildID string
	Zone           *time.Location
	Aliases        []string
	Account        store.AccountReader
	Ledger         store.OptionLedgerReader
	Sessions       store.SessionDayReader
	Ledgers        compute.NewOptionLedgerFunc
	Figures        blockapi.NewPageFiguresFunc
	Holdings       blockapi.NewHoldingFiguresFunc
	Eod            store.OptionEodReader
	Terms          store.OptionTermsReader
	Structures     compute.NewOptionStructuresFunc
}

// NewPage is the household_risk page of one account; alias "" means the main account.
func NewPage(c Config, alias string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "household_risk", Schema: "page.household_risk.v1"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return build(ctx, c, alias, day, at)
		},
	}
}
