// Package structures_page builds the view models of the option structures list and detail pages.
package structures_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegsapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

// StockIDReader reads the instruments of type stock whose name is in names; a name returned once is one stock.
type StockIDReader interface {
	StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
}

// Config is what the structures pages need besides the series reader. A nil reader is the gap no_reader; a nil
// Zone or an alias not in Aliases is an error of Build.
type Config struct {
	RunID, BuildID string
	Zone           *time.Location // generated_at is written in it
	Aliases        []string
	Ledger         store.OptionLedgerReader
	Account        store.AccountReader
	Eod            store.OptionEodReader
	Sessions       store.SessionDayReader
	Terms          store.OptionTermsReader
	StockIDs       StockIDReader      // the stock instrument id of a held underlying by name, the key of option_eod; nil gives no quotes
	Bars           bar.PriceBarReader // nil gives no bars: the close then comes from the option_eod prints
	Dividends      dividend.Reader    // nil gives the Chinese gap no_reader in the Ex-dividend cell of the list
	Ledgers        compute.NewOptionLedgerFunc
	Structures     compute.NewOptionStructuresFunc
	Spreads        spreadlegsapi.NewSpreadFunc // nil gives the gap no_reader on the spread legs page; the other pages ignore it
}

// NewListPage is the structures_list page of alias.
func NewListPage(c Config, alias string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "structures_list", Schema: "page.structures_list.v1"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return buildList(ctx, c, alias, day, at)
		},
	}
}

// NewDetailPage is the structure_detail page of the structure structureID of alias.
func NewDetailPage(c Config, alias, structureID string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "structure_detail", Schema: "page.structure_detail.v3"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return buildDetail(ctx, c, alias, structureID, day, at)
		},
	}
}

// NewSpreadsPage is the spread_legs_page of alias: one block per spread with its legs, net credit, max loss and
// break-evens. An alias not in Aliases and a nil Spreads give the gap page, not an error.
func NewSpreadsPage(c Config, alias string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "spread_legs_page", Schema: "page.spread_legs_page.v1"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return buildSpreads(ctx, c, alias, day, at)
		},
	}
}
