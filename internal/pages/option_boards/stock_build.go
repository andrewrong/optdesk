package option_boards

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// BuildStock builds page.stock_option_page.v1. r is unused; at is given in UTC and written in c.Zone as generated_at.
// Stub: it answers an error until the stock page builder replaces it.
func BuildStock(_ context.Context, _ StockConfig, _ store.SeriesReader, _ contracts.Day, _ time.Time) ([]byte, error) {
	return nil, errNotBuilt
}
