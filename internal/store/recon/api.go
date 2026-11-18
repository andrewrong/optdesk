// Package recon declares the store of the reconciliation runs of the base data page: the writer of a run with its
// items and the reader of the newest run per pair and day (docs/design/base-data-page.md, Interface, entry 6).
// It holds declarations only: the implementation is internal/store/reconpg. Importers name it reconstore.
package recon

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	// DSNEnvRecon names the line that holds the DSN of the role ot_w_recon.
	DSNEnvRecon = "OT_PG_DSN_RECON"
	// LatestItemsMax is the most items per run that Reader.Latest returns.
	LatestItemsMax = 20
	// RunItemsMax is the most items per run that Writer.InsertRun accepts.
	RunItemsMax = 500
)

// Item is one finding of a run; a match is never stored. Company is the symbol.
type Item struct {
	Key, Company, Ours, Other, Diff, Kind, Reason string
}

// Run is one comparison of one pair for one session day. Status is ok, partial or failed; Reason is empty or one of
// rate_limited, network, timeout, source_changed. The count of each kind is stored on the run, and Items holds only
// the items that are not match, at most RunItemsMax per run.
type Run struct {
	PairID                                      string
	SessionDay                                  contracts.Day
	Status, Reason                              string
	StartedAt, FinishedAt                       time.Time
	N, Matched, Diff, MissingOurs, MissingOther int
	Skipped, PairsAsked, PairsCompared          int
	ToleranceText, SourceID                     string
	Items                                       []Item
}

// Writer inserts runs as role ot_w_recon, insert-only: a second run for the same pair and day is a new run.
// InsertRun returns the new run id and refuses more than RunItemsMax items and any match item.
type Writer interface {
	InsertRun(ctx context.Context, r Run) (int64, error)
	Close() error
}

// Reader reads the newest finished run per pair and day, each with at most LatestItemsMax items ordered by
// severity diff, missing_ours, missing_other, skipped, then company and key. The slices are non-nil.
type Reader interface {
	Latest(ctx context.Context) ([]Run, error)
	Close() error
}

// OpenWriterFunc opens a Writer on the database named by dsn.
type OpenWriterFunc func(ctx context.Context, dsn string) (Writer, error)

// OpenReaderFunc opens a Reader on the database named by dsn.
type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, error)
