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

// Meta names a page. Schema is informational: no code reads it, and it is not
// the JSON field "schema" of a view model.
type Meta struct {
	Name   string
	Schema string
}

// Builder builds the view model of one page. The generated_at of the page is
// generatedAt in UTC, RFC 3339; a builder never reads a clock.
type Builder func(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error)

// Page is a page builder with its name.
type Page struct {
	Meta  Meta
	Build Builder
}
