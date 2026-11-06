// Package sources owns the clients of external data sources.
package sources

import (
	"context"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// FREDConfig holds the arguments of NewFREDFunc.
type FREDConfig struct {
	BaseURL string
	// APIKey is passed in by the caller; this package reads no environment.
	APIKey string
	Clock  clock.Clock
}

// FetchPlan names the series to fetch and the observation date range, inclusive.
type FetchPlan struct {
	SeriesIDs []string
	From      contracts.Day
	To        contracts.Day
}

// Source returns the points of a plan with provenance; it returns contracts.TypedError values only.
type Source interface {
	Fetch(ctx context.Context, plan FetchPlan) ([]contracts.SeriesPoint, error)
}

// NewFREDFunc is the constructor of the FRED source (internal/sources/fred.New).
type NewFREDFunc func(FREDConfig) (Source, error)
