package price_lines_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Page is the price_lines_page page: its Meta and Build as a pages.Builder. The page reads no series: the Builder
// builds the read-only page of no reader; the route of cmd/ot calls Build with a full Input instead.
func Page() pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "price_lines_page", Schema: "page.price_lines_page.v1"},
		Build: func(_ context.Context, _ store.SeriesReader, _ contracts.Day, at time.Time) ([]byte, error) {
			return Build(Input{RunID: "page", BuildID: "page", Scope: "price_lines", Now: at})
		},
	}
}
