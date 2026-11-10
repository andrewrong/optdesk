package fundamentals

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// NewFundamentalsFunc is the constructor of the fundamentals job (internal/jobs/fundamentals.New): the SEC facts
// of the cik: entries of Market US of the watch list, fetched through one FactSource.Fetch per run and written
// through one FactWriter.InsertFacts call. The job id is fundamentals; docs/design/valuation.md#fundamentals-job.
type NewFundamentalsFunc func(sources.FactSource, store.FactWriter, []jobs.WatchEntry, clock.Clock) (contracts.Job, error)

// FundamentalsSkip names one entry of Market US the fundamentals job does not fetch. Subject is the Issuer text
// or, when it is empty, Contract.Symbol; Reason is no_cik (an empty or non-cik: issuer) or edinet_not_fetched.
type FundamentalsSkip struct {
	Subject, Reason string
}

// FundamentalsSkips is implemented by the job that fundamentals.New returns: the skipped entries in
// watch-list order.
type FundamentalsSkips interface {
	Skipped() []FundamentalsSkip
}
