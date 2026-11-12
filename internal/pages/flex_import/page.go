package flex_import

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Page is the Flex import page: its Meta and Build as a pages.Builder. The Builder reads no series and builds the
// read-only page (form null, one no_reader gap, no files, no recent records); the route of cmd/ot calls Build with a
// full Input instead.
func Page() pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "flex_import", Schema: "page.flex_import.v1"},
		Build: func(_ context.Context, _ store.SeriesReader, _ contracts.Day, _ time.Time) ([]byte, error) {
			return Build(Input{Gaps: []Gap{{Kind: gapKind, Reason: gapNoReader, Text: readOnlyGapText}}})
		},
	}
}
