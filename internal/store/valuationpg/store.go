// Package valuationpg implements valuation.Reader and valuation.Writer on PostgreSQL: the five insert-only tables of
// schema valuation (docs/design/valuation-structure.md, Decisions 8 and 9). The writer inserts into the tables as the
// role ot_w_settings; the reader reads the views rd.valuation_*_r1 and nothing else. Only cmd/ot imports it.
package valuationpg

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the database/sql driver "pgx"

	"github.com/andrewrong/optdesk/internal/store/valuation"
)

var (
	_ valuation.OpenFunc = Open
	_ valuation.Reader   = (*Store)(nil)
	_ valuation.Writer   = (*Store)(nil)
)

// Store is the valuation store: a valuation.Reader and a valuation.Writer over one pool.
type Store struct{ db *sql.DB }

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("valuationpg: open: %w", err)
	}
	return db, nil
}

// Open connects to the database named by dsn and returns the Reader, the Writer and the closer of the one pool. It
// applies no migration: the schema is made by goose with the role ot_owner. What the connection may do is what its
// role may do: the writer needs ot_w_settings.
func Open(dsn string) (valuation.Reader, valuation.Writer, io.Closer, error) {
	db, err := openDB(dsn)
	if err != nil {
		return nil, nil, nil, err
	}
	s := &Store{db: db}
	return s, s, db, nil
}

// OpenReader connects to the database named by dsn (the pages line, as OpenGapReader does) and returns the Reader,
// which reads the views rd.valuation_*_r1 only, and the closer of its pool. The connection is made on the first query;
// Ping at open reports a wrong dsn at once.
func OpenReader(ctx context.Context, dsn string) (valuation.Reader, io.Closer, error) {
	db, err := openDB(dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("valuationpg: ping: %w", err)
	}
	return &Store{db: db}, db, nil
}
