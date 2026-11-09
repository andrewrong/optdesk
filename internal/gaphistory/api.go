// Package gaphistory reads the rows of the view rd.job_gap_r1 for the morning report.
// It declares one read interface and its own row type; opening the database is the
// store's work.
package gaphistory

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Gap is one row of rd.job_gap_r1 (the columns of store.Gap). A zero time is NULL.
type Gap struct {
	JobID, DatasetID string
	Day              contracts.Day
	Reason           string
	FirstSeenAt      time.Time
	ReportedAt       time.Time
	ClearedAt        time.Time
}

// Reader reads the gap history.
type Reader interface {
	// GapsSince returns the rows with first_seen_at >= since or cleared_at >= since,
	// ascending by FirstSeenAt, never nil.
	GapsSince(ctx context.Context, since time.Time) ([]Gap, error)
	// Close releases the connection.
	Close() error
}

// OpenFunc opens a Reader on the database named by dsn.
type OpenFunc func(ctx context.Context, dsn string) (Reader, error)
