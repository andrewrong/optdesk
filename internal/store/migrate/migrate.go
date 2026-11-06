// Package migrate applies goose SQL migrations to a database.
// It takes the migration files as an fs.FS and embeds none, so the caller decides where they come from.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// Up applies every pending migration found in fsys (the *.sql files at its root) to db, in version order.
// A database already at the latest version is left unchanged.
func Up(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return fmt.Errorf("migrate: provider: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}
