// Package option_candidates_page builds the view model of the option candidates page.
package option_candidates_page

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// Config is what the option_candidates_page builder needs besides the series reader.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location        // generated_at is written in it; nil is an error of Build
	Candidates            store.CandidateReader // nil gives the gap no_reader
}

// NewPage is the option_candidates_page page for c.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "option_candidates_page", Schema: "page.option_candidates_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}

// Build is the pages.Builder shape; r is unused and at is generated_at in UTC. It is not built yet.
func Build(_ context.Context, _ Config, _ store.SeriesReader, _ contracts.Day, _ time.Time) ([]byte, error) {
	return nil, errors.New("option_candidates_page: not built")
}
