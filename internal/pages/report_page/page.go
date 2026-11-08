package report_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// NewPage is the report_page page for c; its Build ignores the series reader it is given.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "report_page", Schema: "page.report_page.v2"},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
			return Build(ctx, c, day, generatedAt)
		},
	}
}
