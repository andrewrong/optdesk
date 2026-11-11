package underlyings

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Page is the underlyings page: its Meta and Build as a pages.Builder. The page reads no series: the Builder takes
// generatedAt and the day and builds the read-only page of no rows in UTC; the route of cmd/ot calls Build with a
// full Input instead.
func Page() pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "underlyings", Schema: "page.underlyings.v1"},
		Build: func(_ context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(Input{Now: at, SessionDay: string(day)})
		},
	}
}
