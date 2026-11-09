// Package structures_page builds the view models of the option structures list and detail pages.
package structures_page

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

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
	Ledgers        compute.NewOptionLedgerFunc
	Structures     compute.NewOptionStructuresFunc
}

func notBuilt(context.Context, store.SeriesReader, contracts.Day, time.Time) ([]byte, error) {
	return nil, errors.New("structures_page: not built")
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
		Meta:  pages.Meta{Name: "structure_detail", Schema: "page.structure_detail.v3"},
		Build: notBuilt,
	}
}
