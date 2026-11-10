package option_boards

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var errNotBuilt = errors.New("option_boards: not built")

// BuildOption builds page.option_page.v1. r is unused; at is given in UTC and written in c.Zone as generated_at.
// Stub: it answers an error until the option page builder replaces it.
func BuildOption(_ context.Context, _ OptionConfig, _ store.SeriesReader, _ contracts.Day, _ time.Time) ([]byte, error) {
	return nil, errNotBuilt
}
