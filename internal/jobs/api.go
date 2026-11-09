// Package jobs owns the jobs the binary runs.
package jobs

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

// Registry is the jobs the binary runs.
type Registry []contracts.Job

// NewFredSeriesFunc is the constructor of the FRED series job (internal/jobs/fredseries.New).
type NewFredSeriesFunc func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error)

// CaptureID identifies one capture: one per key, deterministic,
// uuid.NewSHA1(uuid.NameSpaceOID, []byte(day + "|" + dataset + "|" + subject)).
type CaptureID = uuid.UUID

// NewOptionEodFunc is the constructor of the option_eod job (internal/jobs/optioneod.New).
type NewOptionEodFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error)

// NewQuoteSnapshotFunc is the constructor of the quote_snapshot job (internal/jobs/quotesnapshot.New).
type NewQuoteSnapshotFunc func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error)

// ComputeStores is the seven store interfaces the compute_figures job reads and writes.
type ComputeStores struct {
	Catalog     store.OptionEodCatalog
	OptionEod   store.OptionEodReader
	Terms       store.OptionTermsReader
	Names       store.InstrumentNameReader
	SessionDays store.SessionDayReader
	Series      store.SeriesReader
	Readings    store.ReadingStore
}

// NewComputeFiguresFunc is the constructor of the compute_figures job (internal/jobs/computefigures.New).
type NewComputeFiguresFunc func(ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, clock.Clock) (contracts.Job, error)

// NewComputeFiguresEaseFunc is the constructor of the compute_figures job that also stores the ease figure
// (internal/jobs/computefigures.NewWithEase); the Ease comes after the SeriesOps and
// before the clock. With a nil compute.Ease it builds the same job as
// NewComputeFiguresFunc; with one the job plans and stores the key macro.ease|market|WALCL.
type NewComputeFiguresEaseFunc func(
	ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
) (contracts.Job, error)

// SeriesLoaded is the count of points fetched for one series.
type SeriesLoaded struct {
	SeriesID string
	Fetched  int
}

// SeriesHistoryLoader loads the history of series once, outside the daily job. Load fetches plan.From..plan.To
// once per series and writes each series once through the idempotent store.SeriesWriter. It refuses an empty
// series list, From after To and a range over 5 years (Permanent), and returns contracts.TypedError values only.
type SeriesHistoryLoader interface {
	Load(ctx context.Context, plan sources.FetchPlan) ([]SeriesLoaded, error)
}

// NewSeriesHistoryFunc is the constructor of the loader (internal/jobs/serieshistory.New).
type NewSeriesHistoryFunc func(sources.Source, store.SeriesWriter) (SeriesHistoryLoader, error)

// CalendarObserver is called once per changed day of the calendar_refresh job, before the job writes; the
// wiring logs one line per call. It must not block and its return value is not used.
type CalendarObserver func(clock.SessionDayChange)

// NewCalendarRefreshFunc is the constructor of the calendar_refresh job (internal/jobs/calendarrefresh.New).
type NewCalendarRefreshFunc func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, CalendarObserver) (contracts.Job, error)

// WatchEntry is one [[underlying]] of $OT_HOME/watch.toml (docs/design/event-sources.md Decision 1), one
// field per key. Issuer is cik:<10 digits> for a US company and edinet:<code> for a Japanese one; Market is
// US or JP; Contract is the IBKR contract of the company.
type WatchEntry struct {
	Issuer string
	Market string
	// ReportDateOverride is the owner's own report date, YYYY-MM-DD; empty is unset. The owner's word wins over
	// any automatic value: the job writes only an owner row for such an entry and no vendor row.
	ReportDateOverride string
	// BuyPrice is the price the owner would buy at: decimal text in the currency of the contract, positive,
	// at most 4 places; empty is unset. The owner's word wins over any automatic value.
	BuyPrice string
	// SalePrice is the price the owner would sell at: decimal text in the currency of the contract, positive,
	// at most 4 places, not below BuyPrice; empty is unset. The owner's word wins over any automatic value.
	SalePrice string
	Contract  broker.Contract
}

// NewEarningsRefreshFunc is the constructor of the earnings_refresh job (internal/jobs/earningsrefresh.New).
// The map holds one EarningsSource per market, keyed US or JP; a market with no entry (or a nil source) gets
// unsupported rows with source none. The job runs once per XNYS session day after the close, makes one Fetch
// per source per run and never retries: a source that has a row of the day, error included, is not called
// again, and a second run of the day writes no row for an entry that has one and returns the same Permanent
// error when a row of the day is error. The reader is how the job sees the rows of the day.
type NewEarningsRefreshFunc func(
	map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []WatchEntry, clock.Clock,
) (contracts.Job, error)

// NewOptionEodWatchFunc is the constructor of the option_eod job that takes its underlyings from the watch
// list (internal/jobs/optioneod.NewWithWatch): the US stock entries, each with a key per symbol. Expiries are
// those with DTE in min_dte..max_dte of job.toml; the strikes of an entry with a BuyPrice run from
// 0.80 x BuyPrice to max(1.05 x BuyPrice, spot), at most max_strikes, the nearest BuyPrice kept; an entry
// without a BuyPrice keeps the strike_band. The list comes before the clock.
type NewOptionEodWatchFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []WatchEntry, clock.Clock) (contracts.Job, error)

// NewOptionEodHeldFunc is the constructor of the option_eod job that also captures the contracts of the short
// calls the account holds, whatever their DTE (declaration only; its implementation is a later issue). The
// watch list comes after the writer, the reader after the list and the clock last.
type NewOptionEodHeldFunc func(
	broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []WatchEntry, store.AccountReader, clock.Clock,
) (contracts.Job, error)

// OptionLedgerStores is the store interfaces the option ledger job uses.
type OptionLedgerStores struct {
	Ledger store.OptionLedgerWriter
	Read   store.OptionLedgerReader
	// Sections records the Flex section rows; nil switches the section step off.
	Sections flexsection.Writer
}

// FlexRun is one Flex login of the option ledger job. A login whose token variable is empty reaches the job
// as a FlexRun with a nil Source: the constructor accepts it and never fails for a missing token; Run
// returns a failed result when the login is Required, else skips it with the degraded log record
// (job ledger-09-job).
type FlexRun struct {
	Name     string
	QueryIDs []string
	Required bool
	Source   sources.FlexSource
}

// OptionLedgerConfig is the configuration of the option_ledger job. Accounts maps account number to alias;
// the wiring builds it from the environment and it is never printed. HistoryFrom is the first day fetched.
type OptionLedgerConfig struct {
	Logins      []FlexRun
	Accounts    map[string]string
	HistoryFrom string
}

// NewOptionLedgerJobFunc is the constructor of the option_ledger job (internal/jobs/optionledger).
type NewOptionLedgerJobFunc func(OptionLedgerStores, OptionLedgerConfig, clock.Clock) (contracts.Job, error)

// ImportResult counts the lines read and the rows inserted, the same and the conflicting.
type ImportResult struct {
	Lines, Inserted, Same, Conflict int
}

// ImportOptionTraderFunc imports the JSON Lines export of option-trader's stored Flex rows
// (internal/jobs/optionledger); aliasMap maps the alias of the file to the alias of the ledger and now is the
// recording instant.
type ImportOptionTraderFunc func(ctx context.Context, r io.Reader, w store.OptionLedgerWriter, aliasMap map[string]string, now time.Time) (ImportResult, error)

// FlexAttributesSHA256Func is the hash of the attributes of one Flex row (internal/jobs/optionledger.AttributesSHA256).
// The recipe: remove the keys accountId and acctAlias; encode the rest as one JSON object, keys ascending by
// byte order, no whitespace, strings in UTF-8 with only ", \ and control characters below 0x20 escaped and no
// HTML escaping; the hash is the lower-case hex SHA-256 of that text.
type FlexAttributesSHA256Func func(attrs map[string]string) (string, error)

// VerifyResult counts the lines read, the lines with no stored row of their key (Missing) and the lines whose
// key has a stored row with another hash (Different).
type VerifyResult struct {
	Lines, Missing, Different int
}

// ImportAccountFunc imports the account.jsonl of the cut-over export (internal/jobs/cutover.ImportAccount):
// one store.Capture per line, insert-only, the capture id <alias>:<day>:import. aliasMap maps the alias of the
// file to the alias of the store (nil keeps every alias; a non-nil map refuses an alias it does not name
// before any write). A line after through is refused. Errors name the line number, never its content.
type ImportAccountFunc func(ctx context.Context, r io.Reader, st store.AccountStore, aliasMap map[string]string, through contracts.Day) (ImportResult, error)

// VerifyAccountFunc checks the same file against the store and writes nothing
// (internal/jobs/cutover.VerifyAccount); aliasMap and through are as in ImportAccountFunc.
type VerifyAccountFunc func(ctx context.Context, r io.Reader, rd store.AccountReader, aliasMap map[string]string, through contracts.Day) (VerifyResult, error)

// FigureStatus is the outcome of the comparison of one figure.
type FigureStatus string

// The five statuses of a compared figure.
const (
	FigureChecked     FigureStatus = "checked"
	FigureExplained   FigureStatus = "explained"
	FigureUnexplained FigureStatus = "unexplained"
	FigureNew         FigureStatus = "new"
	FigureUnchecked   FigureStatus = "unchecked"
)

// FigureResult is the status of one figure of the register and, for unchecked, the reason.
type FigureResult struct {
	ID      string
	Version int
	Status  FigureStatus
	Reason  string
}

// FigureReport is the result of a comparison; OK is true only when no figure is unexplained.
type FigureReport struct {
	Results []FigureResult
	OK      bool
}

// CompareFiguresFunc compares the old figure file with optdesk's readings (internal/jobs/cutover.CompareFigures):
// old is figure.jsonl, register the figure register, classes the class file and explained the owner's
// explained entries.
type CompareFiguresFunc func(ctx context.Context, old, register, classes, explained io.Reader, rd store.ReadingReader) (FigureReport, error)

// CandidateStores is the stores the option_candidates job uses. Earnings, Accounts and Series may be nil: the
// gates that need them answer unknown. Catalog lists the underlyings that have a chain and the newest chain day
// of each (the job takes the id of a symbol from Names); Candidates is the one writer.
type CandidateStores struct {
	OptionEod   store.OptionEodReader
	Catalog     store.OptionEodCatalog
	Terms       store.OptionTermsReader
	Names       store.InstrumentNameReader
	Series      store.SeriesReader
	Earnings    store.EarningsDateReader
	Accounts    store.AccountReader
	Candidates  store.CandidateWriter
	SessionDays store.SessionDayReader
}

// NewOptionCandidatesJobFunc is the constructor of the option_candidates job (internal/jobs/optioncandidates.New),
// anchored at session_close plus 30 minutes on XNYS. For each US stock entry of the list it evaluates
// compute.KindShortPut and writes the rows under a new run id (one UUID text per Run); a JP entry gives no row
// and the job names it through its Skipped method. The job holds no dividend reader and sends nothing.
type NewOptionCandidatesJobFunc func(CandidateStores, compute.OptionCandidates, []WatchEntry, clock.Clock) (contracts.Job, error)

// OptionEodWindow is the window the owner settings give the option_eod job: min_dte, max_dte and max_strikes.
// A zero field keeps the value of the embedded job.toml; a non-zero field replaces it.
type OptionEodWindow struct {
	MinDTE, MaxDTE, MaxStrikes int
}

// NewOptionEodWatchWindowFunc is the constructor of the option_eod job that takes its underlyings from the
// watch list and its window from the owner settings (internal/jobs/optioneod.NewWithWatchWindow). It behaves
// as NewOptionEodWatchFunc, then replaces min_dte, max_dte and max_strikes of the manifest with each non-zero
// field of the window; a window with min_dte above max_dte is an error. The window comes after the list.
type NewOptionEodWatchWindowFunc func(
	broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []WatchEntry, OptionEodWindow, clock.Clock,
) (contracts.Job, error)
