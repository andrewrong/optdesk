package datacheck

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

// CheckWindowSessions is the number of session days of a market a rule looks at.
const CheckWindowSessions = 5

// GapGrace is how old an open job gap must be before gap_unclosed reports it.
const GapGrace = 24 * time.Hour

// ExpectedDataset is a dataset that has a plan on every session day of its calendar; cmd/ot builds the list.
type ExpectedDataset struct{ Dataset, Calendar string }

// Rule is one check. A rule that returns an error is stored as one finding of severity unknown.
type Rule struct {
	ID  string
	Run func(ctx context.Context, in Input) ([]finding.Finding, error)
}

// Input is what a rule reads.
type Input struct {
	CheckDay contracts.Day
	Now      time.Time
	Windows  map[string][]contracts.Day // calendar id to its session days of the window, ascending, check day excluded
	Cov      coverage.Reader
	Probe    finding.Probe
	Expected []ExpectedDataset
}
