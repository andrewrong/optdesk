// Package flexsection declares the store of the Flex sections of option-ledger: the rows of
// the extra sections of a Flex statement and one status per section and fetch. It holds
// declarations only; the implementation is internal/store/flexsectionpg.
package flexsection

import (
	"context"
	"time"
)

// Row is one row of a Flex section. Attributes is the JSON object of the attributes as
// text; AttributesSHA256 is the hex digest of that text.
type Row struct {
	Alias            string
	Section          string
	RowKey           string
	Revision         int
	FetchID          string
	RowIndex         int
	Attributes       string
	AttributesSHA256 string
	RecordedAt       time.Time
}

// Status says whether a fetch held a section: State is "present" or "absent".
type Status struct {
	FetchID     string
	Alias       string
	Section     string
	State       string
	RowCount    int
	RowsRefused int
	RecordedAt  time.Time
}

// Insert counts what RecordSections did with the rows: New keys written at revision 1,
// Revised keys written at a higher revision, Unchanged rows that matched the newest
// revision and were not written again.
type Insert struct {
	New       int
	Revised   int
	Unchanged int
}

// Writer records sections.
type Writer interface {
	// RecordSections writes rows and statuses of the fetch fetchID in one transaction.
	// The revision rule works on (Alias, Section, RowKey): a new key is revision 1, a key
	// whose attributes differ from its newest revision is written at that revision plus
	// one, and an identical one is counted Unchanged. The Revision a Row carries is
	// ignored. A row whose Attributes hold accountId or acctAlias is an error and nothing
	// is written. With no rows and no statuses no database call is made.
	RecordSections(ctx context.Context, fetchID string, rows []Row, statuses []Status) (Insert, error)
}

// Reader reads sections. Both methods read the views, never a base table.
type Reader interface {
	// Rows returns the highest revision of every key of the section of the alias,
	// sorted by RowKey.
	Rows(ctx context.Context, alias, section string) ([]Row, error)
	// Statuses returns the newest status per section of the alias.
	Statuses(ctx context.Context, alias string) ([]Status, error)
}

// Store is the writer and the reader of one database.
type Store interface {
	Writer
	Reader
	Close() error
}

// OpenFunc opens a Store on a DSN.
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
