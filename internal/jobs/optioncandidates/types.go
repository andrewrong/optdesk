package optioncandidates

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

// CandidateStores is the stores the option_candidates job uses. Earnings, Accounts and Series may be nil: the
// gates that need them answer unknown. Catalog lists the underlyings that have a chain and the newest chain day
// of each (the job takes the id of a symbol from Names); Candidates is the one writer.
type CandidateStores struct {
	OptionEod store.OptionEodReader
	Catalog   store.OptionEodCatalog
	Terms     store.OptionTermsReader
	Names     store.InstrumentNameReader
	Series    store.SeriesReader
	Earnings  store.EarningsDateReader
	Accounts  store.AccountReader
	// Aliases lists the account labels whose newest capture the job reads, in this order; a nil list reads no account.
	Aliases     []string
	Candidates  store.CandidateWriter
	SessionDays store.SessionDayReader
}

// NewOptionCandidatesJobFunc is the constructor of the option_candidates job (internal/jobs/optioncandidates.New),
// anchored at session_close plus 30 minutes on XNYS. For each US stock entry of the list it evaluates
// compute.KindShortPut and writes the rows under a new run id (one UUID text per Run); a JP entry gives no row
// and the job names it through its Skipped method. The job holds no dividend reader and sends nothing.
type NewOptionCandidatesJobFunc func(CandidateStores, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error)

// NewOptionCandidatesCallsFunc is the constructor of the option_candidates job with the dividend reader
// (internal/jobs/optioncandidates.NewWithCalls). It adds the covered-call and held-call-guard rows to the run: the
// same job, schedule, plan and Skipped as NewOptionCandidatesJobFunc, which is unchanged, and in one run the
// short_put rows, then the covered_call and held_call_guard rows of the same run id, with the next ex-date and
// the amount of the newest dividend row. A nil dividend reader is an error; a read error of it fails the run.
type NewOptionCandidatesCallsFunc func(CandidateStores, dividend.Reader, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error)

// SettingsVersionFunc answers the version of the owner settings in force at an instant (the shape of
// ownerset.Reader.Version, so this package imports nothing of the store; docs/design/settings-form.md Decision 13).
// The option_candidates job (internal/jobs/optioncandidates.WithSettingsVersion) calls it once per run with the
// run's instant and writes the value on every row of the run; with no function or on an error the rows carry nil.
type SettingsVersionFunc func(ctx context.Context, at time.Time) (int64, error)
