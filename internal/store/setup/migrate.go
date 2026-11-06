package setup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Errors of the connection and role steps have a fixed text that names the stage. They never wrap the error of
// pgx, net/url or database/sql, whose text can hold pieces of the DSN.
var (
	errParse   = errors.New("setup: migrate: parse: the admin DSN must be a postgres:// URL")
	errConnect = errors.New("setup: migrate: connect: the server refused or could not be reached")
	errRole    = errors.New("setup: migrate: set role ot_owner failed")
	errSchema  = errors.New("setup: migrate: schema: create schema meta failed")
)

// execer is the part of a connection the session setup uses; *pgx.Conn satisfies it.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// MigrateUp applies the *.sql files of fsys to the database optdesk of the server of adminDSN as ot_owner.
// The first statement of every session is the role switch; then the schema meta is created, which holds the
// goose version table. The error of a connection, parse or role step is of fixed text and holds no DSN.
func MigrateUp(ctx context.Context, adminDSN string, fsys fs.FS) error {
	return migrateUp(ctx, adminDSN, fsys, slog.Default())
}

func migrateUp(ctx context.Context, adminDSN string, fsys fs.FS, log *slog.Logger) error {
	dsn, err := withDatabase(adminDSN)
	if err != nil {
		return errParse
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return errParse
	}
	open := func(ctx context.Context, after func(context.Context, execer) error) (*sql.DB, error) {
		db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(ctx context.Context, c *pgx.Conn) error {
			return after(ctx, c)
		}))
		db.SetMaxOpenConns(1)
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, errConnect
		}
		return db, nil
	}
	return migrateWith(ctx, open, gooseUp, fsys, log)
}

// migrateWith opens the database with the session setup as the after-connect step, then runs up on it.
// When the role switch or the schema step fails, up is not called.
func migrateWith(
	ctx context.Context,
	open func(context.Context, func(context.Context, execer) error) (*sql.DB, error),
	up func(context.Context, *sql.DB, fs.FS) error,
	fsys fs.FS,
	log *slog.Logger,
) error {
	var stage error
	after := func(ctx context.Context, c execer) error {
		if err := ownerSession(ctx, c, log); err != nil {
			stage = err
			return err
		}
		if _, err := c.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS meta"); err != nil {
			stage = errSchema
			return errSchema
		}
		return nil
	}
	db, err := open(ctx, after)
	if err != nil {
		if stage != nil {
			return stage
		}
		return errConnect
	}
	if db != nil {
		defer func() { _ = db.Close() }()
	}
	return up(ctx, db, fsys)
}

// ownerSession is the setup of every session: the role first, so that nothing runs as the admin role, then the
// search path, so that goose creates and finds its version table in meta.
func ownerSession(ctx context.Context, c execer, log *slog.Logger) error {
	if _, err := c.Exec(ctx, "SET ROLE ot_owner"); err != nil {
		return errRole
	}
	if _, err := c.Exec(ctx, "SET search_path TO meta"); err != nil {
		return errSchema
	}
	log.Debug("session ready", "role", roleOwner)
	return nil
}

// gooseUp applies the migrations of fsys with goose's default version table, which lands in meta by search_path.
func gooseUp(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err == nil {
		_, err = p.Up(ctx)
	}
	if err != nil {
		return fmt.Errorf("setup: migrate: goose: %w", err)
	}
	return nil
}

// OwnerDSN returns the DSN of the database optdesk on the server of adminDSN with the startup option that sets
// the role ot_owner; a server that refuses the role refuses the login. host, port, user, password and sslmode
// are kept. The value holds the password: the caller does not log it.
func OwnerDSN(adminDSN string) (string, error) {
	u, err := url.Parse(adminDSN)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return "", errParse
	}
	q := url.Values{"options": {"-c role=" + roleOwner}}
	if m := u.Query().Get("sslmode"); m != "" {
		q.Set("sslmode", m)
	}
	out := url.URL{Scheme: "postgres", User: u.User, Host: u.Host, Path: "/" + Database, RawQuery: q.Encode()}
	return out.String(), nil
}
