// Package option_notices_page builds the view model of the option notices page.
package option_notices_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Config is what the option_notices_page builder needs besides the series reader.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location // generated_at is written in it; nil is an error of Build
	Main                  string         // the alias of the main account
	Aliases               []string       // the aliases to show, in order
	Account               store.AccountReader
	Candidates            store.CandidateReader    // nil gives the gap no_reader
	Earnings              store.EarningsDateReader // nil gives the gap no_reader
	Rule                  compute.OptionNotices    // nil is an error of Build
	Params                compute.NoticeParams
}

// NewPage is the option_notices_page page for c.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "option_notices_page", Schema: "page.option_notices_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}
