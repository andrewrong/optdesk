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

// Cursor is the day up to which the runner has reconciled.
type Cursor struct {
	Day contracts.Day
}

// Lease is the run lease: its owner and its expiry.
type Lease struct {
	Owner     string
	ExpiresAt time.Time
}

// Window is the dataset window: the open and close instants.
type Window struct {
	OpenAt  time.Time
	CloseAt time.Time
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
	// PutWindow stores the dataset window; Window reads it back.
	PutWindow(ctx context.Context, w Window) error
	Window(ctx context.Context) (Window, error)
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
