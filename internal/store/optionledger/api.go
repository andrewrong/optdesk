// Interface file of the import log: declarations only. The implementation is in a later file of this package.
// Design: docs/design/flex-xml-import.md#3 import-log-api.
package optionledger

import (
	"context"

	"github.com/andrewrong/optdesk/internal/store"
)

// ImportLog is the read of past imports. RecentFetches returns the records of login whose query_id starts with
// "file:", newest first by finished_at and then fetch_id (both descending), at most limit of them, from
// rd.flex_fetch_r1. The filter on the query form keeps a flex.toml login that is itself named import out of
// the list: its daily records have digit query ids.
type ImportLog interface {
	RecentFetches(ctx context.Context, login string, limit int) ([]store.FlexFetchRecord, error)
}
