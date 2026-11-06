// Package pages builds and serves the page view models.
package pages

import (
	"context"
	"net/http"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// BuildMacroRatesFunc builds the page.macro_rates.v1 view model (internal/pages/macro_rates.Build).
// The generated_at of the page is generatedAt in UTC, RFC 3339; the builder never reads a clock.
type BuildMacroRatesFunc func(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error)

// NewHandlerFunc serves a view model as HTTP (internal/pages/embed.Handler).
type NewHandlerFunc func(viewModel []byte) http.Handler
