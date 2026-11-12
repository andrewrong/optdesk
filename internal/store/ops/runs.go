package ops

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

var (
	_ runview.Reader       = (*runs)(nil)
	_ runview.ReasonWriter = (*runs)(nil)
)

const (
	selectRunViewSQL = `SELECT dataset_id, subject_key, status, finished_at, returned_n, usable_n, error_code, reason_code ` +
		`FROM rd.capture_run_r1 WHERE session_day = $1 ORDER BY dataset_id, subject_key, finished_at NULLS LAST`
	selectReasonViewSQL = `SELECT dataset_id, subject_key, scope, code, detail, at FROM rd.option_run_reason_r1 ` +
		`WHERE session_day = $1 ORDER BY dataset_id, subject_key, scope, code`
	selectNotWiredSQL = `SELECT dataset_id, subject_key, scope, code, detail, at FROM rd.option_run_reason_r1 ` +
		`WHERE session_day = $1 AND dataset_id = $2 ORDER BY scope`
	deleteReasonsSQL = `DELETE FROM ops.option_run_reason WHERE session_day = $1 AND dataset_id = $2 AND subject_key = $3`
	insertReasonSQL  = `INSERT INTO ops.option_run_reason (session_day, dataset_id, subject_key, scope, code, detail, at) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7) ` +
		`ON CONFLICT (session_day, dataset_id, subject_key, scope) DO UPDATE SET code = EXCLUDED.code, ` +
		`detail = EXCLUDED.detail, at = EXCLUDED.at`
)

// RunStore reads the runs and the reasons of a session day and writes the reasons of a run.
type RunStore interface {
	runview.Reader
	runview.ReasonWriter
	Close() error
}

type runs struct{ db database }

// OpenRuns connects to the database named by dsn and returns the run store. It reads the two rd views; it writes
// ops.option_run_reason, which the role ot_runner (the role of the ledger) may do.
func OpenRuns(ctx context.Context, dsn string) (RunStore, error) {
	db, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &runs{db: db}, nil
}

func (r *runs) Close() error { return r.db.Close() }

// RunsOn returns the runs of the session day; see runview.Reader.
func (r *runs) RunsOn(ctx context.Context, day contracts.Day) ([]runview.Row, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	out := []runview.Row{}
	err = r.db.Select(ctx, selectRunViewSQL, []any{d}, func(s scanner) error {
		row := runview.Row{SessionDay: day}
		var fin *time.Time
		if err := s.Scan(&row.Dataset, &row.Key, &row.Status, &fin, &row.Returned, &row.Usable, &row.ErrorCode, &row.ReasonCode); err != nil {
			return err
		}
		row.FinishedAt = fromNullable(fin)
		out = append(out, row)
		return nil
	})
	if err != nil {
		return nil, wrap("runs on", err)
	}
	return out, nil
}

// ReasonsOn returns the reasons of the session day; see runview.Reader.
func (r *runs) ReasonsOn(ctx context.Context, day contracts.Day) ([]runview.Reason, error) {
	return r.reasons(ctx, "reasons on", day, selectReasonViewSQL, nil)
}

// NotWired returns the reasons stored under runview.NotWiredDay; see runview.Reader.
func (r *runs) NotWired(ctx context.Context) ([]runview.Reason, error) {
	return r.reasons(ctx, "not wired", runview.NotWiredDay, selectNotWiredSQL, []any{runview.NotWiredDataset})
}

func (r *runs) reasons(ctx context.Context, what string, day contracts.Day, sql string, extra []any) ([]runview.Reason, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	out := []runview.Reason{}
	err = r.db.Select(ctx, sql, append([]any{d}, extra...), func(s scanner) error {
		x := runview.Reason{Day: day}
		var at time.Time
		if err := s.Scan(&x.Dataset, &x.Subject, &x.Scope, &x.Code, &x.Detail, &at); err != nil {
			return err
		}
		x.At = at.UTC()
		out = append(out, x)
		return nil
	})
	if err != nil {
		return nil, wrap(what, err)
	}
	return out, nil
}

// ReplaceReasons replaces the set of the day, dataset and subject in one transaction; see runview.ReasonWriter.
// The day, dataset and subject of each reason are the arguments, not the fields of the reason. Two reasons of one
// scope leave the later one.
func (r *runs) ReplaceReasons(ctx context.Context, day contracts.Day, dataset, subject string, reasons []runview.Reason) error {
	d, err := parseDay(day)
	if err != nil {
		return err
	}
	err = r.db.InTx(ctx, func(q querier) error {
		if err := q.Exec(ctx, deleteReasonsSQL, d, dataset, subject); err != nil {
			return err
		}
		for _, x := range reasons {
			if err := q.Exec(ctx, insertReasonSQL, d, dataset, subject, x.Scope, x.Code, x.Detail, x.At.UTC()); err != nil {
				return err
			}
		}
		return nil
	})
	return wrap("replace reasons", err)
}
