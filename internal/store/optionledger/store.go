// Package optionledger holds the store of the option ledger: Open connects with the role ot_w_optionledger
// (INSERT and SELECT only) and returns a Store that appends the Flex trade rows and the fetch records to schema
// optionledger (store.OptionLedgerWriter) and reads them through the views rd.flex_trade_row_r1 and
// rd.flex_fetch_r1 (store.OptionLedgerReader). Nothing is updated or deleted. Design:
// docs/design/option-ledger.md Decisions 5 and 6.
package optionledger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OptionLedgerStore = (*Store)(nil)
var _ store.OpenOptionLedgerFunc = func(ctx context.Context, dsn string) (store.OptionLedgerStore, error) {
	return Open(ctx, dsn)
}

const (
	sourceFlex   = "flex"
	sourceImport = "ot_import"
)

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// tx is one open transaction.
type tx interface {
	// Exec runs a statement and returns the number of rows it wrote.
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	// Select runs a query and calls scan once per row.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(tx) error) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

// Store is the option ledger store.
type Store struct{ db database }

// Open connects to the database named by dsn (the line store.DSNEnvOptionLedger, passed in by the caller). It
// applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("optionledger: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("optionledger: ping: %w", err))
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// RecordFlexFetch writes the rows and the fetch record in one transaction (see store.OptionLedgerWriter). The
// counts the record carries for new, revised and unchanged rows are replaced by what this call did.
func (s *Store) RecordFlexFetch(ctx context.Context, rec store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	for i, r := range rows {
		if err := checkRow(r); err != nil {
			return store.FlexInsert{}, fmt.Errorf("optionledger: row %d: %w", i, err)
		}
	}
	first, last, err := recordDays(rec)
	if err != nil {
		return store.FlexInsert{}, err
	}
	var out store.FlexInsert
	err = s.db.InTx(ctx, func(t tx) error {
		out = store.FlexInsert{}
		for _, r := range rows {
			var have []revision
			if err := t.Select(ctx, latestRevisionSQL, key(r), scanRevision(&have)); err != nil {
				return err
			}
			next := 1
			switch {
			case len(have) == 0:
				out.New++
			case have[0].hash == r.AttributesSHA256:
				out.Unchanged++
				continue
			default:
				next = have[0].n + 1
				out.Revised++
			}
			n, err := t.Exec(ctx, insertRowSQL, rowArgs(r, next, sourceFlex, nil)...)
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("revision %d of %s/%s/%s exists: a concurrent writer", next, r.Alias, r.LevelOfDetail, r.TransactionID)
			}
		}
		_, err := t.Exec(ctx, insertFetchSQL, fetchArgs(rec, first, last, out)...)
		return err
	})
	if err != nil {
		return store.FlexInsert{}, wrap("record fetch", err)
	}
	return out, nil
}

// ImportFlexTradeRows applies Decision 6 to the rows in one transaction (see store.OptionLedgerWriter).
func (s *Store) ImportFlexTradeRows(ctx context.Context, rows []store.FlexTradeRow) (store.FlexImportResult, error) {
	if len(rows) == 0 {
		return store.FlexImportResult{}, nil
	}
	for i, r := range rows {
		if err := checkRow(r); err != nil {
			return store.FlexImportResult{}, fmt.Errorf("optionledger: row %d: %w", i, err)
		}
		if r.Revision < 1 {
			return store.FlexImportResult{}, permanent(fmt.Errorf("optionledger: row %d: revision %d", i, r.Revision))
		}
	}
	var out store.FlexImportResult
	err := s.db.InTx(ctx, func(t tx) error {
		out = store.FlexImportResult{}
		for _, r := range rows {
			var have []revision
			if err := t.Select(ctx, allRevisionsSQL, key(r), scanRevision(&have)); err != nil {
				return err
			}
			same, clash := false, false
			for _, h := range have {
				same = same || h.hash == r.AttributesSHA256
				clash = clash || h.n == r.Revision
			}
			switch {
			case same:
				out.Same++
				continue
			case clash:
				out.Conflict++
				continue
			}
			var at *time.Time
			if !r.RecordedAt.IsZero() {
				at = &r.RecordedAt
			}
			n, err := t.Exec(ctx, insertRowSQL, rowArgs(r, r.Revision, sourceImport, at)...)
			if err != nil {
				return err
			}
			if n != 1 {
				out.Conflict++
				continue
			}
			out.Inserted++
		}
		return nil
	})
	if err != nil {
		return store.FlexImportResult{}, wrap("import", err)
	}
	return out, nil
}

// FlexTradeRows returns the highest revision of every transaction of the alias, sorted by transaction id.
func (s *Store) FlexTradeRows(ctx context.Context, alias string) ([]store.FlexTradeRow, error) {
	out := []store.FlexTradeRow{}
	err := s.db.Select(ctx, selectRowsSQL, []any{alias}, func(sc scanner) error {
		var r store.FlexTradeRow
		if err := sc.Scan(&r.Alias, &r.LevelOfDetail, &r.TransactionID, &r.Revision, &r.Attributes,
			&r.AttributesSHA256, &r.Source, &r.RecordedAt); err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, wrap("read rows", err)
	}
	return out, nil
}

// LatestOkFetch returns the newest fetch of the login and query id whose status is ok.
func (s *Store) LatestOkFetch(ctx context.Context, login, queryID string) (store.FlexFetchRecord, bool, error) {
	var rec store.FlexFetchRecord
	found := false
	err := s.db.Select(ctx, selectLatestOkSQL, []any{login, queryID}, func(sc scanner) error {
		var first, last *time.Time
		if err := sc.Scan(&rec.Login, &rec.QueryID, &first, &last, &rec.StartedAt, &rec.FinishedAt, &rec.Status,
			&rec.ErrorKind, &rec.ErrorCode, &rec.RawSHA256, &rec.ParserVersion, &rec.Statements,
			&rec.UnmappedStatement, &rec.RowsParsed, &rec.RowsNew, &rec.RowsRevised, &rec.RowsUnchanged); err != nil {
			return err
		}
		rec.FirstDay, rec.LastDay = dayText(first), dayText(last)
		found = true
		return nil
	})
	if err != nil {
		return store.FlexFetchRecord{}, false, wrap("read fetch", err)
	}
	return rec, found, nil
}

// StoredAliases returns every alias that has a stored row, ascending.
func (s *Store) StoredAliases(ctx context.Context) ([]string, error) {
	out := []string{}
	err := s.db.Select(ctx, selectAliasesSQL, nil, func(sc scanner) error {
		var a string
		if err := sc.Scan(&a); err != nil {
			return err
		}
		out = append(out, a)
		return nil
	})
	if err != nil {
		return nil, wrap("read aliases", err)
	}
	return out, nil
}

type revision struct {
	n    int
	hash string
}

func scanRevision(into *[]revision) func(scanner) error {
	return func(sc scanner) error {
		var r revision
		if err := sc.Scan(&r.n, &r.hash); err != nil {
			return err
		}
		*into = append(*into, r)
		return nil
	}
}

func key(r store.FlexTradeRow) []any { return []any{r.Alias, r.LevelOfDetail, r.TransactionID} }

func rowArgs(r store.FlexTradeRow, rev int, source string, at *time.Time) []any {
	return []any{r.Alias, r.LevelOfDetail, r.TransactionID, rev, r.Attributes, r.AttributesSHA256, source, at}
}

func fetchArgs(rec store.FlexFetchRecord, first, last *time.Time, c store.FlexInsert) []any {
	return []any{rec.Login, rec.QueryID, first, last, rec.StartedAt, rec.FinishedAt, rec.Status, rec.ErrorKind,
		rec.ErrorCode, rec.RawSHA256, rec.ParserVersion, rec.Statements, rec.UnmappedStatement, rec.RowsParsed,
		c.New, c.Revised, c.Unchanged}
}

// checkRow refuses a row with an empty key part or hash, attributes that are not a JSON object, or attributes
// that hold the key accountId or acctAlias: alias is the only identity and no account number is stored.
func checkRow(r store.FlexTradeRow) error {
	if r.Alias == "" || r.LevelOfDetail == "" || r.TransactionID == "" || r.AttributesSHA256 == "" {
		return permanent(fmt.Errorf("empty alias, level of detail, transaction id or hash"))
	}
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil || attrs == nil {
		return permanent(fmt.Errorf("attributes are not a JSON object"))
	}
	for _, k := range []string{"accountId", "acctAlias"} {
		if _, ok := attrs[k]; ok {
			return permanent(fmt.Errorf("attributes hold the key %s", k))
		}
	}
	return nil
}

func recordDays(rec store.FlexFetchRecord) (first, last *time.Time, err error) {
	if first, err = parseDay(rec.FirstDay); err != nil {
		return nil, nil, err
	}
	if last, err = parseDay(rec.LastDay); err != nil {
		return nil, nil, err
	}
	return first, last, nil
}

// parseDay turns a day into a date; an empty day is NULL.
func parseDay(d contracts.Day) (*time.Time, error) {
	if d == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return nil, permanent(fmt.Errorf("optionledger: bad day %q: %w", d, err))
	}
	return &t, nil
}

func dayText(t *time.Time) contracts.Day {
	if t == nil {
		return ""
	}
	return contracts.Day(t.Format(time.DateOnly))
}

func wrap(what string, err error) error {
	return retryable(fmt.Errorf("optionledger: %s: %w", what, err))
}

// typedErr is a contracts.TypedError wrapping a cause.
type typedErr struct {
	kind  contracts.ErrorKind
	cause error
}

func (e *typedErr) Error() string             { return e.cause.Error() }
func (e *typedErr) Unwrap() error             { return e.cause }
func (e *typedErr) Kind() contracts.ErrorKind { return e.kind }

func retryable(err error) error { return &typedErr{kind: contracts.Retryable, cause: err} }
func permanent(err error) error { return &typedErr{kind: contracts.Permanent, cause: err} }

// pgDB is the pgx adapter of database.
type pgDB struct{ pool *pgxpool.Pool }

type pgTx struct{ tx pgx.Tx }

func (p pgTx) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := p.tx.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}

func (p pgTx) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	return selectRows(ctx, p.tx, sql, args, scan)
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func selectRows(ctx context.Context, q querier, sql string, args []any, scan func(scanner) error) error {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (d pgDB) InTx(ctx context.Context, fn func(tx) error) error {
	t, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = t.Rollback(ctx) }()
	if err := fn(pgTx{tx: t}); err != nil {
		return err
	}
	return t.Commit(ctx)
}

func (d pgDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	return selectRows(ctx, d.pool, sql, args, scan)
}

func (d pgDB) Close() error {
	d.pool.Close()
	return nil
}
