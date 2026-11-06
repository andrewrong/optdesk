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

// The stages of MigrateUp that can fail before goose runs. Their text is fixed: an error of pgx, net/url or
// database/sql for these steps can hold the host, the user or pieces of the DSN, so none is wrapped or formatted.
var (
	errParse   = errors.New("setup: migrate: parse failed")
	errConnect = errors.New("setup: migrate: connect failed")
	errRole    = errors.New("setup: migrate: set role failed")
	errSchema  = errors.New("setup: migrate: schema failed")
)

// execer is the part of a connection the session setup needs; *pgx.Conn satisfies it.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// afterConnect runs once on every new connection, before the connection is used.
type afterConnect func(ctx context.Context, conn execer) error

// MigrateUp applies the *.sql files of fsys to the database optdesk of the server of adminDSN, as the role
// ot_owner: the first statement of every session switches to that role, so every object belongs to ot_owner.
// The schema meta is made there and holds goose's version table. A role the server refuses stops the run before
// any other statement. Errors name a stage and never hold the DSN; only the error of the goose run is wrapped.
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
	open := func(_ context.Context, after afterConnect) (*sql.DB, error) {
		db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(ctx context.Context, c *pgx.Conn) error {
			return after(ctx, c)
		}))
		db.SetMaxOpenConns(1)
		return db, nil
	}
	return migrateWith(ctx, open, gooseUp, fsys, log)
}

// migrateWith opens the database with ownerSession as the after-connect step, makes the schema meta, then calls up.
func migrateWith(ctx context.Context, open func(context.Context, afterConnect) (*sql.DB, error),
	up func(context.Context, *sql.DB, fs.FS) error, fsys fs.FS, log *slog.Logger) error {
	db, err := open(ctx, func(ctx context.Context, c execer) error { return ownerSession(ctx, c, log) })
	if err != nil {
		return errConnect
	}
	defer func() { _ = db.Close() }()
	// The first use of the pool makes the one connection and so runs the session setup.
	if err := db.PingContext(ctx); err != nil {
		if errors.Is(err, errRole) {
			return errRole
		}
		return errConnect
	}
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS meta"); err != nil {
		return errSchema
	}
	log.Info("schema ready", "schema", "meta")
	return up(ctx, db, fsys)
}

// ownerSession is the session setup: switch to the owner role first, then point search_path at meta, so goose
// creates and finds its version table there. When the role is refused nothing else runs.
func ownerSession(ctx context.Context, conn execer, log *slog.Logger) error {
	if _, err := conn.Exec(ctx, "SET ROLE ot_owner"); err != nil {
		return errRole
	}
	if _, err := conn.Exec(ctx, "SET search_path TO meta"); err != nil {
		return errSchema
	}
	log.Debug("session ready", "role", roleOwner)
	return nil
}

// gooseUp applies the migrations of fsys to db; the version table is goose's default, found through search_path.
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

// OwnerDSN returns the DSN of the database optdesk on the server of adminDSN, with the startup option that sets
// the role ot_owner. A server that refuses the role refuses the login, so nothing runs as the admin role.
// Host, port, user, password and sslmode are kept. The value holds the password: do not log it.
func OwnerDSN(adminDSN string) (string, error) {
	u, err := parseURL(adminDSN)
	if err != nil {
		return "", errParse
	}
	q := url.Values{"options": {"-c role=" + roleOwner}}
	if m := u.Query().Get("sslmode"); m != "" {
		q.Set("sslmode", m)
	}
	out := url.URL{Scheme: "postgres", User: u.User, Host: u.Host, Path: "/" + Database, RawQuery: q.Encode()}
	return out.String(), nil
}
