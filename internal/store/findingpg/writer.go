// Package findingpg is the Postgres implementation of finding.Writer and finding.Reader over pgx
// (docs/design/data-check.md, entry 4). The writer is the role ot_w_datacheck and inserts into schema datacheck;
// the reader is the page role ot_ro_pages and reads the views rd.data_check_run_r1 and rd.data_finding_r1 only.
package findingpg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/store/finding"
)

// maxTextBytes is the longest Subject, Dataset or Rule: an id or a name, never free text.
const maxTextBytes = 120

// evidenceNames is the fixed vocabulary of Count.Name (internal/store/finding).
var evidenceNames = map[string]bool{
	"ok": true, "same": true, "rows": true, "late": true, "hours_behind": true, "keys": true,
	"missing": true, "expired": true, "cells": true, "age_hours": true, "read_failed": true,
}

const (
	insertRunSQL = `INSERT INTO datacheck.run (check_day, started_at, finished_at, rules_total, rules_unknown)
VALUES ($1::date, $2, $3, $4, $5) RETURNING run_id`
	// insertFindingSQL computes is_new in the insert: no finding of the same key in an earlier run that has a
	// notice row (Decision 8). Arguments: run_id, rule, dataset, day, subject, severity, evidence.
	insertFindingSQL = `INSERT INTO datacheck.finding (run_id, rule, dataset, day, subject, severity, evidence, is_new)
SELECT $1, $2, $3, $4::date, $5, $6, $7, NOT EXISTS (
    SELECT 1 FROM datacheck.finding f JOIN datacheck.notice n ON n.run_id = f.run_id
    WHERE f.run_id < $1 AND f.rule = $2 AND f.dataset = $3 AND f.day = $4::date AND f.subject = $5)
RETURNING is_new`
	markNoticedSQL = `INSERT INTO datacheck.notice (run_id, noticed_at) VALUES ($1, now()) ON CONFLICT (run_id) DO NOTHING`
)

type pgWriter struct{ pool *pgxpool.Pool }

var _ finding.Writer = (*pgWriter)(nil)

// OpenWriter connects to the database named by dsn (the line finding.DSNEnvDataCheck). It applies no migration.
func OpenWriter(ctx context.Context, dsn string) (finding.Writer, error) {
	pool, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &pgWriter{pool: pool}, nil
}

var _ finding.OpenWriterFunc = OpenWriter

func openPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("findingpg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("findingpg: ping: %w", err)
	}
	return pool, nil
}

func (w *pgWriter) Close() error {
	w.pool.Close()
	return nil
}

// validate refuses what could carry free text or an id: an evidence name outside the vocabulary, a severity
// outside the three, a Subject, Dataset or Rule longer than maxTextBytes.
func validate(r finding.Run) error {
	for i, f := range r.Findings {
		for _, s := range []struct{ name, v string }{{"rule", f.Rule}, {"dataset", f.Dataset}, {"subject", f.Subject}} {
			if len(s.v) > maxTextBytes {
				return fmt.Errorf("findingpg: finding %d: %s is %d bytes, over %d", i, s.name, len(s.v), maxTextBytes)
			}
		}
		switch f.Severity {
		case finding.SeverityError, finding.SeverityWarn, finding.SeverityUnknown:
		default:
			return fmt.Errorf("findingpg: finding %d: severity %q", i, f.Severity)
		}
		for _, c := range f.Evidence {
			if !evidenceNames[c.Name] {
				return fmt.Errorf("findingpg: finding %d: evidence name %q is not in the vocabulary", i, c.Name)
			}
		}
	}
	return nil
}

type evidenceJSON struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

// InsertRun stores r and its findings in one transaction and returns r with RunID and every IsNew set.
func (w *pgWriter) InsertRun(ctx context.Context, r finding.Run) (finding.Run, error) {
	if err := validate(r); err != nil {
		return finding.Run{}, err
	}
	out := r
	out.Findings = make([]finding.Finding, len(r.Findings))
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return finding.Run{}, fmt.Errorf("findingpg: insert run: %w", err)
	}
	fail := func(err error) (finding.Run, error) {
		if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, rerr)
		}
		return finding.Run{}, fmt.Errorf("findingpg: insert run: %w", err)
	}
	if err := tx.QueryRow(ctx, insertRunSQL, string(r.CheckDay), r.StartedAt, r.FinishedAt, r.RulesTotal, r.RulesUnknown).
		Scan(&out.RunID); err != nil {
		return fail(err)
	}
	for i, f := range r.Findings {
		ev := make([]evidenceJSON, len(f.Evidence))
		for j, c := range f.Evidence {
			ev[j] = evidenceJSON{Name: c.Name, N: c.N}
		}
		raw, err := json.Marshal(ev)
		if err != nil {
			return fail(err)
		}
		if err := tx.QueryRow(ctx, insertFindingSQL, out.RunID, f.Rule, f.Dataset, string(f.Day), f.Subject,
			string(f.Severity), string(raw)).Scan(&f.IsNew); err != nil {
			return fail(err)
		}
		out.Findings[i] = f
	}
	if err := tx.Commit(ctx); err != nil {
		return fail(err)
	}
	return out, nil
}

// MarkNoticed records that the message of runID was delivered; a second call for the same run writes nothing.
func (w *pgWriter) MarkNoticed(ctx context.Context, runID int64) error {
	if _, err := w.pool.Exec(ctx, markNoticedSQL, runID); err != nil {
		return fmt.Errorf("findingpg: mark noticed %d: %w", runID, err)
	}
	return nil
}
