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
	// OptionEodLatest returns one entry per underlying_id with a row of session_day on or after since, ordered by
	// UnderlyingID. since is a session_day date (YYYY-MM-DD); an empty since means no lower bound. No row gives
	// an empty list, not an error.
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
	// OptionTerms returns the version with valid_to IS NULL of each id, ordered by InstrumentID; an id without a
	// current version is absent from the answer; an empty list gives an empty answer; more than 500 ids is a
	// Permanent error.
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
