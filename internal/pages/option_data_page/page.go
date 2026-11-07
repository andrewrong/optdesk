package option_data_page

import (
	"context"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// NewPage is the option_data_page page for c. It keeps a copy of c, with its own Notes slice; its Build ignores the
// series reader it is given and reads the readers of c.
func NewPage(c Config) pages.Page {
	c.Notes = slices.Clone(c.Notes)
	return pages.Page{
		Meta: pages.Meta{Name: "option_data_page", Schema: "page.option_data_page.v1"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
			return Build(ctx, c, day, generatedAt)
		},
	}
}
