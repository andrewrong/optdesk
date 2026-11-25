package setup

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Errors of MigrateMissing have a fixed text that names the stage. They never wrap the error of pgx, whose text
// can hold pieces of the DSN.
var (
	errCheckConnect = errors.New("setup: migrate check: connect: the server refused or could not be reached")
	errCheckDenied  = errors.New("setup: migrate check: read meta.goose_db_version: permission denied")
	errCheckAbsent  = errors.New("setup: migrate check: read meta.goose_db_version: the table is absent")
	errCheckQuery   = errors.New("setup: migrate check: read meta.goose_db_version failed")
)

const versionsQuery = `SELECT version_id, is_applied FROM meta.goose_db_version ORDER BY id`

// versionRows is the part of a result set the check reads; pgx.Rows satisfies it.
type versionRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// versionQuerier runs the one SELECT of the check.
type versionQuerier func(ctx context.Context, sql string) (versionRows, error)

// MigrateMissing reads meta.goose_db_version with one SELECT and no write, and returns the versions of want that
// are not applied (missing) and the applied versions of the table that are not in want (extra), both ascending.
// A version is applied when its latest row (highest id) has is_applied true; versions are compared as numbers, so
// one applied out of order counts. The error of a connect, permission or absent-table failure has fixed text and
// holds no DSN.
func MigrateMissing(ctx context.Context, dsn string, want []int64) (missing, extra []int64, err error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, nil, errCheckConnect
	}
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, nil, errCheckConnect
	}
	defer func() { _ = conn.Close(ctx) }()
	return missingFrom(ctx, func(ctx context.Context, q string) (versionRows, error) {
		return conn.Query(ctx, q)
	}, want)
}

func missingFrom(ctx context.Context, query versionQuerier, want []int64) (missing, extra []int64, err error) {
	rows, err := query(ctx, versionsQuery)
	if err != nil {
		return nil, nil, classifyCheck(err)
	}
	defer rows.Close()
	applied := map[int64]bool{} // the last row read of a version wins: rows come ordered by id
	for rows.Next() {
		var v int64
		var ok bool
		if err := rows.Scan(&v, &ok); err != nil {
			return nil, nil, errCheckQuery
		}
		applied[v] = ok
	}
	if err := rows.Err(); err != nil {
		return nil, nil, classifyCheck(err)
	}
	wanted := make(map[int64]bool, len(want))
	for _, v := range want {
		wanted[v] = true
		if !applied[v] {
			missing = append(missing, v)
		}
	}
	for v, ok := range applied {
		if ok && !wanted[v] {
			extra = append(extra, v)
		}
	}
	slices.Sort(missing)
	missing = slices.Compact(missing)
	slices.Sort(extra)
	return missing, extra, nil
}

func classifyCheck(err error) error {
	if pe, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pe.Code {
		case "42501":
			return errCheckDenied
		case "42P01", "3F000":
			return errCheckAbsent
		}
	}
	return errCheckQuery
}
