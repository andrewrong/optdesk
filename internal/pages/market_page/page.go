// Package market_page builds the view model of the market page.
package market_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Config is what the market_page builder needs besides the series reader.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location      // generated_at is written in it; nil is an error of Build
	Readings              compute.ReadingFunc // the method value WinnerReadings of the page-role reader; nil is an error of Build
}

// NewPage is the market_page page for c.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "market_page", Schema: "page.market_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}
