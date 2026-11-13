// Package strategyresult declares the store of strategy results: the generic table strategy.result r1, insert-only,
// written and read by the role ot_w_candidate (docs/design/strategy-framework.md, Interface and Decisions 4 and 5).
// It holds declarations only; the implementation is a later issue. The package imports nothing of internal/strategy.
package strategyresult

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Row is one row of strategy.result r1. Legs, Checks and Facts are JSON text (array, array, object).
type Row struct {
	SessionDay                                   contracts.Day
	RunID, Strategy, RuleVersion, ParamHash, Key string
	UnderlyingID                                 int
	Status                                       string
	Rank, RankInUnderlying                       *int
	Legs, Checks, Facts                          string
	ComputedAt                                   time.Time
	SettingsVersion                              *int64
}

// Writer inserts rows into strategy.result.
type Writer interface {
	// InsertResults inserts the rows, insert-only; the same key again inserts nothing and is no error. It returns
	// the count inserted.
	InsertResults(ctx context.Context, rows []Row) (int, error)
}

// Reader reads the view rd.strategy_result_r1.
type Reader interface {
	// Results returns the rows of one strategy of one day written by the newest run of that day, ranked rows first.
	Results(ctx context.Context, day contracts.Day, strategy string) ([]Row, error)
}

// Store is the Writer and the Reader.
type Store interface {
	Writer
	Reader
	Close() error
}

// OpenFunc opens a Store from a connection string.
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
