package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// stmt is one statement of an upsert call with its arguments.
type stmt struct {
	sql  string
	args []any
}

// upsertDB is the transaction seam of the upsert methods: the statements of one call run in order in one
// transaction, and the rows each statement returned (its RETURNING rows) come back in the same order. The first
// failing statement rolls the transaction back and its error is returned. pgx is touched only by its pgDB method.
type upsertDB interface {
	Upsert(ctx context.Context, stmts []stmt) ([]int, error)
}

// Upsert runs the statements in one transaction and counts the rows each one returned.
func (d pgDB) Upsert(ctx context.Context, stmts []stmt) ([]int, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	counts := make([]int, 0, len(stmts))
	for _, s := range stmts {
		n, err := returned(ctx, tx, s)
		if err != nil {
			if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
				err = errors.Join(err, rerr)
			}
			return nil, err
		}
		counts = append(counts, n)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return counts, nil
}

func returned(ctx context.Context, tx pgx.Tx, s stmt) (int, error) {
	rows, err := tx.Query(ctx, s.sql, s.args...)
	if err != nil {
		return 0, err
	}
	n := 0
	for rows.Next() {
		n++
	}
	rows.Close()
	return n, rows.Err()
}

// upsertErr types an error of an upsert call: a constraint violation is Permanent, every other error Retryable.
func upsertErr(what string, err error) error {
	wrapped := fmt.Errorf("events: %s: %w", what, err)
	if isConstraintViolation(err) {
		return permanent(wrapped)
	}
	return retryable(wrapped)
}

// perRow folds the rows each statement returned into the counts of the input rows: owner[i] is the input row
// statement i belongs to; a row is written when any of its statements returned a row, else unchanged.
func perRow(rows int, owner []int, counts []int) (written int) {
	hit := make([]bool, rows)
	for i, n := range counts {
		if n > 0 {
			hit[owner[i]] = true
		}
	}
	for _, h := range hit {
		if h {
			written++
		}
	}
	return written
}
