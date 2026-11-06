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

// SessionDayStore is the Reader and the Writer of master.session_day.
type SessionDayStore interface {
	SessionDayReader
	SessionDayWriter
	Close() error
}

// SeriesStore is the Reader and the Writer of obs.series_point.
type SeriesStore interface {
	SeriesReader
	SeriesWriter
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

// OpenSessionDayFunc opens the session day store (internal/store/sessionday.Open).
type OpenSessionDayFunc func(ctx context.Context, dsn string) (SessionDayStore, error)

// OpenSeriesFunc opens the series store (internal/store/series.Open).
type OpenSeriesFunc func(ctx context.Context, dsn string) (SeriesStore, error)

// OpenLedgerFunc opens the ops ledger (internal/store/ops.Open).
type OpenLedgerFunc func(ctx context.Context, dsn string) (OpsLedger, error)

// SetupFunc creates the database objects (internal/store/setup.Setup).
type SetupFunc func(ctx context.Context, cfg SetupConfig) error
