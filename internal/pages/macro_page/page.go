package macro_page

import (
	"context"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// NewPage is the macro_page page for c. It keeps a copy of c, so changing the
// Indicators slice of c afterwards does not change the page.
func NewPage(c Config) pages.Page {
	c.Indicators = slices.Clone(c.Indicators)
	return pages.Page{
		Meta: pages.Meta{Name: "macro_page", Schema: "page.macro_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
			return Build(ctx, r, c, day, generatedAt)
		},
	}
}
