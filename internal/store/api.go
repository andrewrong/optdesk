// Package store owns the database; only its implementation packages open it.
package store

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// SessionDayWriter upserts session days.
type SessionDayWriter interface {
	UpsertSessionDays(ctx context.Context, rows []contracts.SessionDay) error
}

// SessionDayReader reads session days of a calendar between two days, inclusive.
type SessionDayReader interface {
	SessionDays(ctx context.Context, calendarID string, from, to contracts.Day) ([]contracts.SessionDay, error)
}

// SeriesWriter inserts points idempotently.
type SeriesWriter interface {
	InsertSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint) error
}

// SeriesReader reads the points of a series with observation date between two days, inclusive.
type SeriesReader interface {
	SeriesPoints(ctx context.Context, seriesID string, from, to contracts.Day) ([]contracts.SeriesPoint, error)
}

// OptionEodWriter appends option end-of-day rows; a row whose key exists is left as it is.
type OptionEodWriter interface {
	InsertOptionEod(ctx context.Context, rows []contracts.OptionEod) error
}

// OptionEodReader reads the rows of an underlying with session_day between two days, inclusive,
// ordered by session_day, instrument_id and capture_id.
type OptionEodReader interface {
	OptionEod(ctx context.Context, underlyingID int64, from, to contracts.Day) ([]contracts.OptionEod, error)
}

// QuoteSnapshotWriter appends quote snapshots idempotently.
type QuoteSnapshotWriter interface {
	InsertQuoteSnapshots(ctx context.Context, rows []contracts.QuoteSnapshot) error
}

// QuoteSnapshotReader reads the snapshots of an instrument with session_day between two days, inclusive,
// ordered by session_day, slot and capture_id.
type QuoteSnapshotReader interface {
	QuoteSnapshots(ctx context.Context, instrumentID int64, from, to contracts.Day) ([]contracts.QuoteSnapshot, error)
}

// Identifier is one name of an instrument. Schemes: ibkr_conid, osi, ticker_exchange, jpx_option_code, isin.
type Identifier struct {
	Scheme, Value string
}

// InstrumentSpec describes an instrument. Type is stock, index or option;
// PriceIncrement and LotSize are decimal text.
type InstrumentSpec struct {
	Type, Name, Currency, MIC, CalendarID, Status, PriceIncrement, LotSize string
}

// Instrument is a master.instrument row with the identifiers that are valid now.
type Instrument struct {
	ID          int64
	Spec        InstrumentSpec
	Identifiers []Identifier
}

// InstrumentResolver gives the id of an instrument.
type InstrumentResolver interface {
	// ResolveInstrument returns the id of the instrument that key names now, creating it with spec and the
	// identifiers key and extra when none exists. The same key always gives the same id, under concurrent calls too.
	ResolveInstrument(ctx context.Context, key Identifier, spec InstrumentSpec, extra []Identifier) (int64, error)
}

// InstrumentReader reads instruments by id.
type InstrumentReader interface {
	// Instruments returns the instruments of ids ordered by id; an unknown id is absent from the answer.
	Instruments(ctx context.Context, ids []int64) ([]Instrument, error)
}

// InstrumentStore is the resolver and the reader of master.instrument.
type InstrumentStore interface {
	InstrumentResolver
	InstrumentReader
	Close() error
}

// OptionEodLatest is an underlying that has at least one option_eod row and the greatest session_day it has.
type OptionEodLatest struct {
	UnderlyingID int64
	SessionDay   contracts.Day
}

// OptionEodCatalog lists the underlyings with rows of option_eod.
type OptionEodCatalog interface {
	// OptionEodLatest returns one entry per underlying_id with a row of session_day on or after since, ordered by UnderlyingID.
	// since is a session_day date (YYYY-MM-DD); an empty since means no lower bound. No row gives an empty list, not an error.
	OptionEodLatest(ctx context.Context, since contracts.Day) ([]OptionEodLatest, error)
}

// OptionEodCatalogStore is the catalog of option_eod and Close.
type OptionEodCatalogStore interface {
	OptionEodCatalog
	Close() error
}

// OptionTerms is the current version of the terms of one option. Right is C or P; Strike and Multiplier are decimal text.
type OptionTerms struct {
	TermsVersionID, InstrumentID       int64
	Right                              string
	Strike, StrikeCurrency, Multiplier string
	ExpirationDate                     contracts.Day
	ExerciseStyle, SettlementType      string
	ValidFrom                          time.Time
}

// OptionTermsReader reads the current terms of options by instrument id.
type OptionTermsReader interface {
	// OptionTerms returns the version with valid_to IS NULL of each id, ordered by InstrumentID; an id without a current
	// version is absent from the answer; an empty list gives an empty answer; more than 500 ids is a Permanent error.
	OptionTerms(ctx context.Context, instrumentIDs []int64) ([]OptionTerms, error)
}

// OptionTermsStore is the reader of the option terms and Close.
type OptionTermsStore interface {
	OptionTermsReader
	Close() error
}

// InstrumentName is the name of one instrument; for a stock or index it is the symbol.
type InstrumentName struct {
	InstrumentID int64
	Type, Name   string
}

// InstrumentNameReader reads names by instrument id.
type InstrumentNameReader interface {
	// InstrumentNames returns the names of ids ordered by InstrumentID; an unknown id is absent; an empty list gives
	// an empty answer and no database call; more than 500 ids or an id of 0 is Permanent.
	InstrumentNames(ctx context.Context, ids []int64) ([]InstrumentName, error)
}

// InstrumentNameStore is the reader of the names and Close.
type InstrumentNameStore interface {
	InstrumentNameReader
	Close() error
}

// ReadingWriter inserts readings, each with its inputs and parameters, idempotently on the key
// (name, version, subject_kind, subject, day, rev). It returns the count of readings inserted; a key that exists adds none.
type ReadingWriter interface {
	InsertReadings(ctx context.Context, rows []contracts.ReadingRecord) (inserted int, err error)
}

// ReadingReader reads the readings of one figure and subject.
type ReadingReader interface {
	// LatestReadings returns the newest rev of each day in the range from to to, inclusive, any state, ascending by day.
	LatestReadings(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)
}

// SessionDayStore is the Reader and the Writer of master.session_day.
type SessionDayStore interface {
	SessionDayReader
	SessionDayWriter
	Close() error
}

// ReadingStore is the Reader and the Writer of readings.reading.
type ReadingStore interface {
	ReadingReader
	ReadingWriter
	Close() error
}

// SeriesStore is the Reader and the Writer of obs.series_point.
type SeriesStore interface {
	SeriesReader
	SeriesWriter
	Close() error
}

// OptionEodStore is the Reader and the Writer of obs.option_eod.
type OptionEodStore interface {
	OptionEodReader
	OptionEodWriter
	Close() error
}

// QuoteSnapshotStore is the Reader and the Writer of obs.quote_snapshot.
type QuoteSnapshotStore interface {
	QuoteSnapshotReader
	QuoteSnapshotWriter
	Close() error
}

// SetupConfig holds the arguments of SetupFunc.
type SetupConfig struct {
	// AdminDSN is the administrative database connection string.
	AdminDSN string
	// Home is the path of $OT_HOME.
	Home string
}

// PresentStatus is the status of a present key.
type PresentStatus string

// The statuses of a present key.
const (
	StatusPresent PresentStatus = "present"
	StatusNoValue PresentStatus = "no_value"
)

// The names of the lines `Setup` writes to $OT_HOME/.env and `cmd/ot` reads, one per role.
const (
	// DSNEnvRunner names the line that holds the DSN of the role ot_runner.
	DSNEnvRunner = "OT_PG_DSN_RUNNER"
	// DSNEnvSeriesFred names the line that holds the DSN of the role ot_w_series_fred.
	DSNEnvSeriesFred = "OT_PG_DSN_SERIES_FRED"
	// DSNEnvPages names the line that holds the DSN of the role ot_ro_pages.
	DSNEnvPages = "OT_PG_DSN_PAGES"
	// DSNEnvLoader names the line of $OT_HOME/.env that holds the DSN of the role ot_loader.
	DSNEnvLoader = "OT_PG_DSN_LOADER"
	// DSNEnvOptionEod names the line that holds the DSN of the role ot_w_option_eod.
	DSNEnvOptionEod = "OT_PG_DSN_OPTION_EOD"
	// DSNEnvQuoteSnapshot names the line that holds the DSN of the role ot_w_quote_snapshot.
	DSNEnvQuoteSnapshot = "OT_PG_DSN_QUOTE_SNAPSHOT"
	// DSNEnvMaster names the line that holds the DSN of the role ot_w_master.
	DSNEnvMaster = "OT_PG_DSN_MASTER"
	// DSNEnvReadings names the line that holds the DSN of the role ot_w_readings.
	DSNEnvReadings = "OT_PG_DSN_READINGS"
	// DSNEnvAlerts names the line that holds the DSN of the role ot_w_alerts.
	DSNEnvAlerts = "OT_PG_DSN_ALERTS"
	// DSNEnvEvents names the line that holds the DSN of the role ot_w_events.
	DSNEnvEvents = "OT_PG_DSN_EVENTS"
	// DSNEnvFundamental names the line that holds the DSN of the role ot_w_fundamental.
	DSNEnvFundamental = "OT_PG_DSN_FUNDAMENTAL"
	// DSNEnvCandidate names the line that holds the DSN of the role ot_w_candidate.
	DSNEnvCandidate = "OT_PG_DSN_CANDIDATE"
	// DSNEnvOptionLedger names the line that holds the DSN of the role ot_w_optionledger.
	DSNEnvOptionLedger = "OT_PG_DSN_OPTION_LEDGER"
	// DSNEnvAccount names the line that holds the DSN of the role ot_w_account.
	DSNEnvAccount = "OT_PG_DSN_ACCOUNT"
	// AdminDSNEnv names the process environment variable that carries SetupConfig.AdminDSN;
	// it is read from the process environment only, never from .env and never from a flag.
	AdminDSNEnv = "OT_PG_ADMIN_DSN"
)

// Cursor is the day up to which the runner has reconciled.
type Cursor struct {
	Day contracts.Day
}

// Lease is the run lease: its owner and its expiry.
type Lease struct {
	Owner     string
	ExpiresAt time.Time
}

// Window is the window of one dataset on one day: its key and the open and close instants.
// A zero time.Time in OpenAt or CloseAt is stored as NULL and read back as the zero value.
type Window struct {
	DatasetID string
	Day       contracts.Day
	OpenAt    time.Time
	CloseAt   time.Time
}

// RunStatus is the status of a run.
type RunStatus string

// The statuses of a run. A crashed run is a running run whose LeaseUntil is before the caller's now.
const (
	RunRunning RunStatus = "running"
	RunDone    RunStatus = "done"
)

// Run is one attempt of the runner on a day and an expected key.
// A zero time.Time in LeaseUntil, HeartbeatAt or FinishedAt is stored as NULL and read back as the zero value.
// ErrorCode holds a contracts.ErrorKind string and ReasonCode a reason string of the runner.
type Run struct {
	// ID is minted by the runner: a deterministic string from the day, the expected key and the attempt number.
	ID                                  string
	Day                                 contracts.Day
	Key                                 contracts.ExpectedKey
	Status                              RunStatus
	LeaseUntil, HeartbeatAt, FinishedAt time.Time
	ReturnedN, UsableN                  int
	ErrorCode, ReasonCode               string
}

// RunCloser is the optional method of the OpsLedger value the scheduler is given (asserted like runner.ConsoleClaimer).
// CloseRunning closes every running run as done with ReasonCode reason and FinishedAt now; it returns the count.
type RunCloser interface {
	CloseRunning(ctx context.Context, now time.Time, reason string) (closed int, err error)
}

// OpsLedger is the only way the runner touches the ops tables.
type OpsLedger interface {
	// PutExpected records the expected keys of day under planHash, replacing an earlier plan.
	PutExpected(ctx context.Context, day contracts.Day, planHash string, keys []contracts.ExpectedKey) error
	// Expected returns the expected keys of day and their plan hash; no plan gives an empty hash.
	Expected(ctx context.Context, day contracts.Day) (keys []contracts.ExpectedKey, planHash string, err error)
	// MarkPresent records that key is present for day with status.
	MarkPresent(ctx context.Context, day contracts.Day, key contracts.ExpectedKey, status PresentStatus) error
	// Present returns the present keys of day with their status.
	Present(ctx context.Context, day contracts.Day) (map[contracts.ExpectedKey]PresentStatus, error)
	// PutWindow stores the window under its DatasetID and Day, replacing an earlier one with the same key.
	PutWindow(ctx context.Context, w Window) error
	// Window reads the window of datasetID on day; with none it returns a contracts.TypedError of kind NoValue.
	Window(ctx context.Context, datasetID string, day contracts.Day) (Window, error)
	// RecordRun inserts r, or updates the run with the same ID.
	RecordRun(ctx context.Context, r Run) error
	// Runs returns the runs of day.
	Runs(ctx context.Context, day contracts.Day) ([]Run, error)
	// SetCursor stores the reconcile cursor; Cursor reads it back.
	SetCursor(ctx context.Context, c Cursor) error
	Cursor(ctx context.Context) (Cursor, error)
	// Acquire takes the run lease for owner until now plus ttl; it fails with a Retryable error while another owner holds it.
	Acquire(ctx context.Context, owner string, ttl time.Duration) (Lease, error)
	Release(ctx context.Context, l Lease) error
	Close() error
}

// ReadEnvFunc reads the KEY=VALUE lines of the env file at path (setup.ReadEnv). It does not touch the process
// environment. A missing file is an error for which errors.Is(err, fs.ErrNotExist) holds; no error carries a value.
type ReadEnvFunc func(path string) (map[string]string, error)

// OpenSessionDayFunc opens the session day store (internal/store/sessionday.Open).
type OpenSessionDayFunc func(ctx context.Context, dsn string) (SessionDayStore, error)

// OpenSeriesFunc opens the series store (internal/store/series.Open).
type OpenSeriesFunc func(ctx context.Context, dsn string) (SeriesStore, error)

// OpenReadingFunc opens the readings store (internal/store/reading.Open).
type OpenReadingFunc func(ctx context.Context, dsn string) (ReadingStore, error)

// OpenOptionEodFunc opens the option end-of-day store (internal/store/snapshot.OpenOptionEod).
type OpenOptionEodFunc func(ctx context.Context, dsn string) (OptionEodStore, error)

// OpenOptionEodCatalogFunc opens the option end-of-day catalog (internal/store/snapshot.OpenOptionEodCatalog).
type OpenOptionEodCatalogFunc func(ctx context.Context, dsn string) (OptionEodCatalogStore, error)

// OpenOptionTermsFunc opens the option terms store (internal/store/instrument.OpenOptionTerms).
type OpenOptionTermsFunc func(ctx context.Context, dsn string) (OptionTermsStore, error)

// OpenInstrumentNameFunc opens the name reader with the line DSNEnvPages (instrument.OpenInstrumentNames).
type OpenInstrumentNameFunc func(ctx context.Context, dsn string) (InstrumentNameStore, error)

// OpenQuoteSnapshotFunc opens the quote snapshot store (internal/store/snapshot.OpenQuoteSnapshot).
type OpenQuoteSnapshotFunc func(ctx context.Context, dsn string) (QuoteSnapshotStore, error)

// OpenInstrumentFunc opens the instrument store (internal/store/instrument.Open).
type OpenInstrumentFunc func(ctx context.Context, dsn string) (InstrumentStore, error)

// OpenLedgerFunc opens the ops ledger (internal/store/ops.Open).
type OpenLedgerFunc func(ctx context.Context, dsn string) (OpsLedger, error)

// SetupFunc creates the database objects (internal/store/setup.Setup).
type SetupFunc func(ctx context.Context, cfg SetupConfig) error

// The statuses a scheduler run may also take (additive; the column is text).
const (
	// RunPartial is a run cut at its deadline or at a stop after it returned rows (Returned > 0). It is a counted
	// verdict but does not mark the key present: the key stays missing and the tick runs it again.
	RunPartial RunStatus = "partial"
	// RunAbandoned is a run cut by a graceful stop with Returned = 0 (reason abandoned_stop); it is not counted.
	// A crashed run is not this status: it stays running, its lease lapses, and the reconciler marks it with reason crashed.
	RunAbandoned RunStatus = "abandoned"
)

// Fence is a held instance fence: one scheduler per database.
type Fence interface {
	// Lost returns a channel that is closed when the fence is no longer held (its connection died).
	Lost() <-chan struct{}
	// Close releases the fence; it is safe to call more than once.
	Close() error
}

// InstanceFence takes the one-scheduler fence. Only cmd/ot calls it.
type InstanceFence interface {
	// TryLock takes the fence for build without waiting; it fails with a Retryable error while another holds it.
	// The fence holds one dedicated connection until Close.
	TryLock(ctx context.Context, build string) (Fence, error)
}

// Gap is one record of ops.job_gap. A key gap (expired, partial, crashed_repeatedly) carries DatasetID, Day and no JobID;
// a preflight_failed gap carries JobID and no DatasetID. A zero time.Time in ReportedAt or ClearedAt is NULL.
// ReportedAt has one owner, the notify module; no page sets it.
type Gap struct {
	JobID, DatasetID string
	Day              contracts.Day
	Reason           string
	FirstSeenAt      time.Time
	ReportedAt       time.Time
	ClearedAt        time.Time
}

// JobState is one row of ops.scheduler_job: the state of a job as the ticker last saw it.
// A zero time.Time is NULL; an empty DisabledReason means the job is enabled.
type JobState struct {
	JobID            string
	NextStart        time.Time
	WindowEnd        time.Time
	RestartSensitive bool
	DisabledReason   string
}

// SchedulerStatus is what `ot scheduler status` shows, read from ops.* only.
type SchedulerStatus struct {
	// Build is the build the running scheduler recorded with SetState.
	Build string
	// Runs are the non-terminal runs, with HeartbeatAt.
	Runs []Run
	Jobs []JobState
	// Gaps are the open gaps.
	Gaps []Gap
}

// SchedulerLedger is the writer of the scheduler tables and the reader of their status; the role is ot_runner.
// Every lease or gap time comes from the database's now(), never from the caller's clock.
type SchedulerLedger interface {
	// RenewLease extends l until the database's now() plus ttl and returns the lease; it fails with a Retryable
	// error when another owner holds the lease.
	RenewLease(ctx context.Context, l Lease, ttl time.Duration) (Lease, error)
	// PutGap inserts g, or updates the open gap of the same key; FirstSeenAt is the database's now() on insert.
	PutGap(ctx context.Context, g Gap) error
	// ClearGap sets ClearedAt of the open gap with the key of g to the database's now(); no open gap is no error.
	ClearGap(ctx context.Context, g Gap) error
	// MarkGapReported sets ReportedAt of the gap with the key of g to the database's now(); only notify calls it.
	MarkGapReported(ctx context.Context, g Gap) error
	// SetState records build in the one row of ops.scheduler_state.
	SetState(ctx context.Context, build string) error
	// PutJobStates replaces the rows of ops.scheduler_job with states.
	PutJobStates(ctx context.Context, states []JobState) error
	// Status reads ops.* only.
	Status(ctx context.Context) (SchedulerStatus, error)
	Close() error
}

// Expectation is one row of rd.expectation_r1. Closed true: the day has no session, so CLOSED;
// Closed false with a zero ExpectedDay: no expectation, so UNKNOWN.
type Expectation struct {
	DatasetID               string
	ExpectedDay, PresentDay contracts.Day
	DueBy                   time.Time
	Gap                     string
	Closed                  bool
}

// GapReader reads the views rd.job_gap_r1 and rd.expectation_r1 as the role ot_ro_pages, which has no grant on an ops table.
type GapReader interface {
	// OpenGaps returns the gaps with no ClearedAt, shown whether or not ReportedAt is set.
	OpenGaps(ctx context.Context) ([]Gap, error)
	// Expectations returns the expectations with ExpectedDay between from and to, inclusive.
	Expectations(ctx context.Context, from, to contracts.Day) ([]Expectation, error)
	Close() error
}

// OpenFenceFunc opens the instance fence (internal/store/ops.OpenFence).
type OpenFenceFunc func(ctx context.Context, dsn string) (InstanceFence, error)

// OpenSchedulerLedgerFunc opens the scheduler ledger (internal/store/ops.OpenSchedulerLedger).
type OpenSchedulerLedgerFunc func(ctx context.Context, dsn string) (SchedulerLedger, error)

// OpenGapReaderFunc opens the gap reader with the line DSNEnvPages (internal/store/ops.OpenGapReader).
type OpenGapReaderFunc func(ctx context.Context, dsn string) (GapReader, error)

// SentMark is one row of alerts.sent: the alert rule that was said on a data day.
type SentMark struct {
	RuleID string
	Day    contracts.Day
}

// SentStore reads and writes the table alerts.sent, key (rule_id, day).
type SentStore interface {
	// SentSince returns the marks with day >= from, ascending by day then rule id.
	SentSince(ctx context.Context, from contracts.Day) ([]SentMark, error)
	// MarkSent inserts the mark ON CONFLICT DO NOTHING, sent_at from the database's now().
	// The caller marks after the send, never before.
	MarkSent(ctx context.Context, ruleID string, day contracts.Day) error
	Close() error
}

// OpenSentFunc opens the sent-mark store with the line DSNEnvAlerts (internal/store/alert.Open).
type OpenSentFunc func(ctx context.Context, dsn string) (SentStore, error)

// ReadingViewReader reads the winner rows of rd.reading_r1 as the role ot_ro_pages (the page process) or ot_w_readings (the notify pass of risk-alerts).
// This reader needs no grant on a readings table: it reads only the views rd.reading_r1 and rd.reading_parameter_r1.
// Rows carry the parameters (data_day among them) read from the view rd.reading_parameter_r1, and no inputs.
// One row per day: the highest version of the day, then in that version its highest-rev ok row, else its highest-rev row.
type ReadingViewReader interface {
	// WinnerReadings returns one row per day with day between from and to, inclusive, ascending by day,
	// for one name, subject kind and subject; no rows gives an empty slice.
	WinnerReadings(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)
	Close() error
}

// OpenReadingViewFunc opens the view reader from a DSN of the role ot_ro_pages (the page process)
// or ot_w_readings (the notify pass of risk-alerts); both hold SELECT on the views (internal/store/reading.OpenView).
type OpenReadingViewFunc func(ctx context.Context, dsn string) (ReadingViewReader, error)

// FactWriter appends the filed facts of obs.fundamental_fact.
type FactWriter interface {
	// InsertFacts inserts the rows ON CONFLICT DO NOTHING on the eight-column key (source, subject, doc_id, taxonomy, concept, unit,
	// period_start_key, period_end), so a rerun adds 0 rows. An absent PeriodStart is stored as period_start_key 0001-01-01 and read back as nil.
	InsertFacts(ctx context.Context, rows []contracts.FundamentalFact) error
}

// FactReader reads the facts through the view rd.fundamental_fact_r1.
type FactReader interface {
	// FactsFiledBy returns every row of the source and subject with filed on or before cutoff, no winner picked;
	// none gives an empty slice. The order is unspecified.
	FactsFiledBy(ctx context.Context, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error)
}

// FactStore is one store for both sides: the figures job opens it with the line DSNEnvFundamental (role ot_w_fundamental,
// INSERT on the table and SELECT on the view), the page with DSNEnvPages (role ot_ro_pages, SELECT on the view only; its InsertFacts fails).
type FactStore interface {
	FactWriter
	FactReader
	Close() error
}

// OpenFactStoreFunc opens the fact store (internal/store/fundamental.Open).
type OpenFactStoreFunc func(ctx context.Context, dsn string) (FactStore, error)

// EarningsDateWriter appends the rows of obs.earnings_date (docs/design/event-sources.md Decision 7).
type EarningsDateWriter interface {
	// InsertEarningsDate inserts one row ON CONFLICT DO NOTHING on the key (observed_on, issuer, attempt):
	// a row whose key exists stays as it is and the call returns no error. A row with no issuer or an
	// attempt below 1 is Permanent, a database error Retryable.
	InsertEarningsDate(ctx context.Context, row contracts.EarningsDate) error
}

// EarningsDateReader reads the rows through the view rd.earnings_date_r1.
type EarningsDateReader interface {
	// EarningsDates returns the rows of one issuer with observed_on from from to to, inclusive, ascending by
	// day, only the highest attempt of each day; none gives an empty slice. A bad day is Permanent.
	EarningsDates(ctx context.Context, issuer string, from, to contracts.Day) ([]contracts.EarningsDate, error)
}

// EarningsDateStore is one store for both sides: the earnings job opens it with the line DSNEnvEvents
// (role ot_w_events, INSERT on the table and SELECT on the view), a page with DSNEnvPages (role ot_ro_pages,
// SELECT on the view only; its InsertEarningsDate fails).
type EarningsDateStore interface {
	EarningsDateWriter
	EarningsDateReader
	Close() error
}

// OpenEarningsDateStoreFunc opens the earnings store (internal/store/events.OpenEarningsDate).
type OpenEarningsDateStoreFunc func(ctx context.Context, dsn string) (EarningsDateStore, error)

// Candidate is one row of candidates.option_candidate r1 as plain text and numbers (design
// docs/design/option-candidates.md): money is decimal text, an optional column is a nil pointer, Gates is the
// ordered JSON array text of the gate results, Marks the JSON array text of the marks and Fits the JSON object
// text from alias to fit, no_fit or unknown. The key is (SessionDay, RunID, Kind, InstrumentID, Alias); Alias is empty for puts.
type Candidate struct {
	SessionDay       contracts.Day
	RunID            string
	Kind             string // short_put, covered_call or held_call_guard
	InstrumentID     int
	Alias            string
	RuleVersion      string
	Issuer           string
	UnderlyingID     int
	Expiry           contracts.Day
	DTE              int
	Right            string // put or call
	Strike           string
	Multiplier       int
	BidEod           string
	Ask              *string
	Spot             *string
	BuyPrice         *string
	SalePrice        *string
	BreakEven        string
	AnnualizedYield  *string
	Delta            *string
	Iv               *string
	Status           string // ok, blocked or unknown
	Gates            string
	Marks            string
	Fits             string
	NextEarnings     *contracts.Day
	EarningsState    string
	NextExDate       *contracts.Day
	ExAmount         *string
	Extrinsic        *string
	Rank             *int
	RankInUnderlying *int
	ComputedAt       time.Time
	SettingsVersion  *int64
}

// CandidateWriter appends the rows of candidates.option_candidate (role ot_w_candidate, line DSNEnvCandidate).
type CandidateWriter interface {
	// InsertCandidates inserts the rows, insert-only: a rerun carries a new RunID and adds rows, none is
	// replaced. It returns the number of rows inserted; no rows inserts none and makes no database call.
	InsertCandidates(ctx context.Context, rows []Candidate) (int, error)
}

// CandidateReader reads the rows through the view rd.option_candidate_r1.
type CandidateReader interface {
	// Candidates returns the rows of one kind of one day written by the newest run of that day (the latest
	// ComputedAt), ranked rows first in ascending Rank, then the rest; a day or kind with no rows gives an
	// empty slice, never nil. A zero day or an unknown kind is an error.
	Candidates(ctx context.Context, day contracts.Day, kind string) ([]Candidate, error)
}

// FlexTradeRow is one stored <Trade> row of schema optionledger (design docs/design/option-ledger.md,
// Decision 5) as plain text: Attributes is the JSON object text of every attribute as sent, without accountId
// and acctAlias, and AttributesSHA256 the lower-case hex hash of it. The key is (Alias, LevelOfDetail,
// TransactionID, Revision); Alias is the only identity and no account number is held. Source is flex or
// ot_import. Revision and RecordedAt are set by the store on RecordFlexFetch and read back by the reader.
type FlexTradeRow struct {
	Alias            string
	LevelOfDetail    string
	TransactionID    string
	Revision         int
	Attributes       string
	AttributesSHA256 string
	Source           string
	RecordedAt       time.Time
}

// FlexFetchRecord is the record of one request pair of one report (Decision 5): the login name and query id,
// the first and last day asked, the start and finish times, the status (ok or a failure), the error kind and
// code (empty when ok), the hash of the raw report, the parser version and the counts. No token, account
// number or row content is held.
type FlexFetchRecord struct {
	Login             string
	QueryID           string
	FirstDay          contracts.Day
	LastDay           contracts.Day
	StartedAt         time.Time
	FinishedAt        time.Time
	Status            string
	ErrorKind         string
	ErrorCode         string
	RawSHA256         string
	ParserVersion     string
	Statements        int
	UnmappedStatement int
	RowsParsed        int
	RowsNew           int
	RowsRevised       int
	RowsUnchanged     int
}

// FlexInsert counts what RecordFlexFetch did with the rows of one report.
type FlexInsert struct {
	New, Revised, Unchanged int
}

// FlexImportResult counts what ImportFlexTradeRows did with the rows given.
type FlexImportResult struct {
	Inserted, Same, Conflict int
}

// OptionLedgerWriter appends to schema optionledger (role ot_w_optionledger, line DSNEnvOptionLedger):
// insert-only, nothing is updated or deleted.
type OptionLedgerWriter interface {
	// RecordFlexFetch writes the fetch record and its rows in one transaction, so a failure leaves neither.
	// The revision rule of Decision 5 is applied per row on the key (Alias, LevelOfDetail, TransactionID):
	// a new key is written as revision 1 (New), a row whose AttributesSHA256 equals that of the highest stored
	// revision is not written (Unchanged), a changed hash is written as the next revision (Revised). The
	// Revision of a row given is ignored. A row whose Attributes hold the key accountId or acctAlias is an error
	// and nothing is written. No rows still writes the record.
	RecordFlexFetch(ctx context.Context, rec FlexFetchRecord, rows []FlexTradeRow) (FlexInsert, error)
	// ImportFlexTradeRows writes imported rows (Source ot_import, Revision as given) in one transaction under
	// Decision 6: a row is inserted only when its key (Alias, LevelOfDetail, TransactionID, Revision) is absent
	// and no stored row of that transaction has the same AttributesSHA256; a row whose hash equals a stored one
	// is Same and not written; a key that exists with another hash is a Conflict, counted and not written.
	// No rows makes no database call.
	ImportFlexTradeRows(ctx context.Context, rows []FlexTradeRow) (FlexImportResult, error)
}

// OptionLedgerReader reads schema optionledger through the views rd.flex_trade_row_r1 and rd.flex_fetch_r1.
type OptionLedgerReader interface {
	// FlexTradeRows returns the highest revision of every transaction of the alias, sorted by TransactionID;
	// an alias with no rows gives an empty slice, never nil.
	FlexTradeRows(ctx context.Context, alias string) ([]FlexTradeRow, error)
	// LatestOkFetch returns the record of the newest fetch of the login and query id whose Status is ok, and
	// true; false when there is none.
	LatestOkFetch(ctx context.Context, login, queryID string) (FlexFetchRecord, bool, error)
	// StoredAliases returns every alias that has a stored row, ascending, each once; none gives an empty slice.
	StoredAliases(ctx context.Context) ([]string, error)
}

// OptionLedgerStore is one store for both sides: the job opens it with the line DSNEnvOptionLedger (role
// ot_w_optionledger, INSERT and SELECT), a page with DSNEnvPages for the reader only.
type OptionLedgerStore interface {
	OptionLedgerWriter
	OptionLedgerReader
	// Close releases the connection.
	Close() error
}

// OpenOptionLedgerFunc opens the option ledger store (internal/store/optionledger.Open).
type OpenOptionLedgerFunc func(ctx context.Context, dsn string) (OptionLedgerStore, error)

// Position is one row of account.position r1, decimals as text. The first four fields repeat the Capture: the
// Reader fills them, WriteCapture ignores them.
type Position struct {
	CaptureID  string
	Alias      string
	CapturedAt time.Time
	SessionDay string // YYYY-MM-DD
	Symbol     string
	SecType    string
	Exchange   string
	Currency   string
	Expiry     string // YYYY-MM-DD, empty when the instrument has none
	Strike     string // decimal text, empty when the instrument has none
	Multiplier string // decimal text
	Right      string // put or call, empty when the instrument has none
	ConID      int64
	Quantity   string // decimal text
	AvgCost    string // decimal text
}

// AccountValue is one row of account.value r1 (a summary tag), decimals as text; the first four fields as in Position.
type AccountValue struct {
	CaptureID  string
	Alias      string
	CapturedAt time.Time
	SessionDay string // YYYY-MM-DD
	Tag        string
	Value      string // decimal text
	Currency   string
}

// Execution is one row of account.execution r1, decimals as text, key (Alias, ExecID); WriteExecutions takes the alias
// from its argument.
type Execution struct {
	Alias              string
	ExecID             string
	PermID             int64
	ExecutedAt         time.Time
	SessionDay         string // YYYY-MM-DD
	Symbol             string
	SecType            string
	Expiry             string // YYYY-MM-DD, empty when the instrument has none
	Strike             string // decimal text, empty when the instrument has none
	Right              string // put or call, empty when the instrument has none
	Multiplier         string // decimal text
	Currency           string
	Side               string
	Quantity           string // decimal text
	Price              string // decimal text
	Commission         string // decimal text
	CommissionCurrency string
}

// Capture is a row of account.capture r1 with its children. ID is <alias>:<session_day>:<run key>, so a retry of a
// run carries the same ID. A kind the gateway does not collect is nil; a collected kind with no row is empty.
type Capture struct {
	ID         string
	Alias      string
	At         time.Time
	SessionDay string // YYYY-MM-DD
	Status     string
	Values     []AccountValue
	Positions  []Position
}

// AccountWriter writes the account datasets (role ot_w_account, line DSNEnvAccount), insert-only.
type AccountWriter interface {
	// WriteCapture writes the capture and its children in one transaction. A capture whose ID exists writes
	// nothing, children included, and returns no error. A database error is Retryable.
	WriteCapture(ctx context.Context, c Capture) error
	// WriteExecutions inserts the rows under alias, ON CONFLICT DO NOTHING on (alias, exec_id).
	WriteExecutions(ctx context.Context, alias string, rows []Execution) error
}

// AccountReader reads the account datasets through the views rd.account_*_r1, never the base tables. Every
// method returns an empty slice, never nil, when there is no row.
type AccountReader interface {
	// Positions returns the position rows of alias on session day day, sorted by key.
	Positions(ctx context.Context, alias, day string) ([]Position, error)
	// Values returns the value rows of alias on session day day, sorted by key.
	Values(ctx context.Context, alias, day string) ([]AccountValue, error)
	// Executions returns the executions of alias with session day from to to, inclusive, sorted by key.
	Executions(ctx context.Context, alias, from, to string) ([]Execution, error)
}

// AccountStore is one store for both sides.
type AccountStore interface {
	AccountWriter
	AccountReader
	// Close releases the connection.
	Close() error
}

// OpenAccountFunc opens the account store with the line DSNEnvAccount (internal/store/account.Open).
type OpenAccountFunc func(ctx context.Context, dsn string) (AccountStore, error)
