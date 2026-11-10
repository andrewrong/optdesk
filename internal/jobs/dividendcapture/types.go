package dividendcapture

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

// NewDividendCaptureFunc is the constructor of the dividend_capture job (internal/jobs/dividendcapture.New;
// docs/design/event-sources.md Decisions 2, 5, 6 and 7). Once per XNYS session day after the close it resolves
// each entry to its instrument_id through the resolver (scheme ibkr_conid; a failed resolve is an error row
// resolve_failed for that stock only), asks ReferenceClient.Dividends for the entries of the markets named in the
// passed_markets key of job.toml and writes one obs.dividend_expectation row per stock through the writer:
// present, no_value, or error with an error_code. An entry of any other market gets an unsupported row with
// source none. One stock failing does not stop the others. The reader is how the job sees the rows of the day: a
// stock that has a row of the day gets no second one.
type NewDividendCaptureFunc func(
	broker.ReferenceClient, store.InstrumentResolver, dividend.Writer, dividend.Reader, []jobs.WatchEntry, clock.Clock,
) (contracts.Job, error)
