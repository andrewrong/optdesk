package earningsrefresh

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// NewEarningsRefreshFunc is the constructor of the earnings_refresh job (internal/jobs/earningsrefresh.New).
// The map holds one EarningsSource per market, keyed US or JP; a market with no entry (or a nil source) gets
// unsupported rows with source none. The job runs once per XNYS session day after the close, makes one Fetch
// per source per run and never retries: a source that has a row of the day, error included, is not called
// again, and a second run of the day writes no row for an entry that has one and returns the same Permanent
// error when a row of the day is error. The reader is how the job sees the rows of the day.
type NewEarningsRefreshFunc func(
	map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock,
) (contracts.Job, error)

// NewEarningsRefreshSymbolsFunc is NewEarningsRefreshFunc with a symbol.Translator as the last argument. A nil
// translator means no translation: the job behaves as it does without one, including its own matching of
// share-class separators.
type NewEarningsRefreshSymbolsFunc func(
	map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock, symbol.Translator,
) (contracts.Job, error)
