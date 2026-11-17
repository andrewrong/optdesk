package option_pnl_page

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// Build is a stub until the real builder lands; it always returns an error.
func Build(_ context.Context, _ Config, _ store.SeriesReader, _ contracts.Day, _ time.Time) ([]byte, error) {
	return nil, errors.New("option_pnl_page: not built")
}
