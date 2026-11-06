// Package migrate applies goose SQL migrations to a database.
// It takes the migration files as an fs.FS and embeds none, so the caller decides where they come from.
package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
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

// UpConnector is Up for a driver.Connector: it opens a database on c, migrates it and closes it.
func UpConnector(ctx context.Context, c driver.Connector, fsys fs.FS) (err error) {
	db := sql.OpenDB(c)
	defer func() {
		if cerr := db.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("migrate: close: %w", cerr)
		}
	}()
	return Up(ctx, db, fsys)
}
