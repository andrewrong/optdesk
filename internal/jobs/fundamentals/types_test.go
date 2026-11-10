package fundamentals_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/fundamentals"
	"github.com/andrewrong/optdesk/internal/sources"
)

type factSourceFake struct{}

func (factSourceFake) Fetch(context.Context, sources.FactPlan) ([]contracts.FundamentalFact, error) {
	return nil, nil
}

type factWriterFake struct{}

func (factWriterFake) InsertFacts(context.Context, []contracts.FundamentalFact) error { return nil }

type zeroClock struct{}

func (zeroClock) Now() time.Time { return time.Unix(0, 0) }

func TestNewFundamentalsFuncContract(t *testing.T) {
	t.Parallel()
	var newJob fundamentals.NewFundamentalsFunc = fundamentals.New
	job, err := newJob(factSourceFake{}, factWriterFake{}, []jobs.WatchEntry{{Market: "US"}}, zeroClock{})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID() != "fundamentals" {
		t.Fatalf("id %q", job.ID())
	}
	sk, ok := job.(fundamentals.FundamentalsSkips)
	if !ok {
		t.Fatal("job does not implement FundamentalsSkips")
	}
	if got := sk.Skipped(); len(got) != 1 || got[0].Reason != "no_cik" {
		t.Fatalf("skipped %v", got)
	}
}
