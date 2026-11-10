// Package mark declares the store of position marks (account.position_mark r1):
// one stored mark per open position of each account capture. It holds
// declarations only; the implementation is the package markpg under internal/store.
package mark

import (
	"context"
	"time"
)

// DSNEnvPositionMark names the line that holds the DSN of the role ot_w_posmark.
const DSNEnvPositionMark = "OT_PG_DSN_POSMARK"

// The basis a mark price comes from.
const (
	BasisMid   = "mid"
	BasisLast  = "last"
	BasisClose = "close"
	BasisNone  = "none"
)

// PositionMark is one row of account.position_mark: the mark of one position of
// one capture. Decimals are text and empty when absent; Price is empty when Basis
// is BasisNone. SessionDay is YYYY-MM-DD, the day of the capture. Attempt starts
// at 1.
type PositionMark struct {
	CaptureID, Alias       string
	MarkedAt               time.Time
	SessionDay             string
	ConID                  int64
	Attempt                int
	Currency, Price, Basis string
	Bid, Ask, Last, Close  string
	MarketDataType         int
	Status                 string
}

// Writer inserts position marks. The table is insert-only.
type Writer interface {
	// WriteMarks writes the rows in one transaction (all rows or none). A row
	// whose key (CaptureID, ConID, Attempt) exists is left as it is; a correction
	// is a row with a higher Attempt.
	WriteMarks(ctx context.Context, rows []PositionMark) error
}

// Reader reads position marks.
type Reader interface {
	// Marks returns the rows of the newest capture of the session day (YYYY-MM-DD)
	// that has marks for the alias: the highest Attempt of each ConID, sorted by
	// ConID. No rows gives an empty slice.
	Marks(ctx context.Context, alias, day string) ([]PositionMark, error)
}

// Store is the writer and the reader, with Close.
type Store interface {
	Writer
	Reader
	Close() error
}

// OpenFunc opens the store from a DSN (markpg.Open, package markpg under internal/store).
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
