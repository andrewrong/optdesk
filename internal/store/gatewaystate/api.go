// Package gatewaystate declares the store of the gateway connection state: one row per gateway alias, saying whether
// the gateway has no connection and since when (docs/design/gateway-down-since.md, entry 2, Decisions 1 and 2). It
// holds declarations only; the implementation is internal/store/gatewaystatepg.
package gatewaystate

import (
	"context"
	"time"
)

// State is one row. Alias is a lower-case gateway alias; Down is true while the gateway has no connection; Since is
// the time the present value of Down began; ObservedAt is the last observation that saw it.
type State struct {
	Alias      string
	Down       bool
	Since      time.Time
	ObservedAt time.Time
}

// Reader reads the rows of the view rd.gateway_state_r1.
type Reader interface {
	// States returns every row, ordered by alias. No rows gives an empty non-nil slice.
	States(ctx context.Context) ([]State, error)
	Close() error
}

// Writer writes the rows of ops.gateway_state.
type Writer interface {
	// Observe records one observation of one gateway, in one statement. A missing row is inserted with Since and
	// ObservedAt equal to at; a row with the same down sets only ObservedAt to at; a row with another down sets
	// Down, Since and ObservedAt to at. An at earlier than the stored ObservedAt changes nothing. The store reads no
	// wall clock: every stored time is the at of the caller.
	Observe(ctx context.Context, alias string, down bool, at time.Time) error
	Close() error
}

// OpenReaderFunc opens a Reader on the PostgreSQL DSN. An empty dsn is an error.
type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, error)

// OpenWriterFunc opens a Writer on the PostgreSQL DSN. An empty dsn is an error.
type OpenWriterFunc func(ctx context.Context, dsn string) (Writer, error)
