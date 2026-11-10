// Package advice_page builds the page.advice_page.v1 view model. No store holds earlier advice yet, so
// the page has no rows and one gap, not_built. It computes no figure, reads no clock and makes no request.
package advice_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Config is what the builder needs. Zone is the owner's zone (nil is an error of Build). Sessions may be
// nil: the page needs no reader to be correct, so a nil reader is never the gap no_reader.
type Config struct {
	RunID, BuildID string
	Zone           *time.Location
	Sessions       store.SessionDayReader
}

// NewPage is the advice_page page.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "advice_page", Schema: "page.advice_page.v1"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, day, at)
		},
	}
}
