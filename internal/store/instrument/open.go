package instrument

import (
	"context"
	"log/slog"

	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenInstrumentFunc = Open

// instrumentStore is the store.InstrumentStore over a database seam.
type instrumentStore struct {
	db database
	// log receives the line of a rolled-back terms insert; Open leaves it nil and logger() then reads slog.Default.
	log *slog.Logger
}

// Open connects to the database named by dsn (the line store.DSNEnvMaster, role ot_w_master) and returns the
// instrument store. It applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (store.InstrumentStore, error) {
	db, err := openDB(ctx, "instrument", dsn)
	if err != nil {
		return nil, err
	}
	return &instrumentStore{db: db}, nil
}

// Close releases the connections.
func (s *instrumentStore) Close() error { return s.db.Close() }
