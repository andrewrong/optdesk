// Package option_tracking_page builds the view model of the option tracking page.
package option_tracking_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// Config is what the option_tracking_page builder needs besides the series reader.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location           // nil is an error of Build
	Ledger                store.OptionLedgerReader // nil gives the gap no_reader
	Account               store.AccountReader      // nil gives the gap no_account of the check tables
	Candidates            store.CandidateReader
	Eod                   store.OptionEodReader
	Catalog               store.OptionEodCatalog
	Names                 store.InstrumentNameReader
	Terms                 store.OptionTermsReader
	Sessions              store.SessionDayReader
	Tracker               trackapi.OptionTracker // nil is an error of Build
	Replay                compute.OptionLedger   // nil is an error of Build
	Marks                 mark.Reader            // nil means no position marks
	Params                trackapi.TrackParams
	CalendarID            string
}

// NewPage is the option_tracking_page page for c.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "option_tracking_page", Schema: "page.option_tracking_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}
