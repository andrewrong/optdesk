// Package option_result_page builds the view model of the option result page.
package option_result_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Config is what the option_result_page builder needs besides the series reader. A nil reader is the gap
// no_reader; a nil Zone or an alias not in Aliases is an error of Build.
type Config struct {
	RunID, BuildID string
	Zone           *time.Location // generated_at is written in it
	Aliases        []string
	Ledger         store.OptionLedgerReader
	Sessions       store.SessionDayReader
	Ledgers        compute.NewOptionLedgerFunc
	Results        compute.NewOptionResultsFunc
}

// NewPage is the option_result_page page of alias.
func NewPage(c Config, alias string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "option_result_page", Schema: "page.option_result_page.v1"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return build(ctx, c, alias, day, at)
		},
	}
}
