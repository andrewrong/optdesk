package ownersetpg

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// The reads go through the views rd.*_r1 and nothing else: a page role and the runner have no grant on a table.
// The newest row per key (or instrument) not after $1: DISTINCT ON over the key, newest first, ties by id.
const (
	paramsSQL = `SELECT DISTINCT ON (param_key) param_key, value, set_at FROM rd.rule_param_r1 ` +
		`WHERE set_at <= $1 ORDER BY param_key, set_at DESC, id DESC`
	exclusionsSQL = `SELECT DISTINCT ON (instrument_id) instrument_id, excluded, set_at FROM rd.option_exclusion_r1 ` +
		`WHERE set_at <= $1 ORDER BY instrument_id, set_at DESC, id DESC`

	// The values in force now, read by Apply in its transaction.
	currentParamsSQL = `SELECT DISTINCT ON (param_key) param_key, value FROM rd.rule_param_r1 ` +
		`ORDER BY param_key, set_at DESC, id DESC`
	currentExclusionsSQL = `SELECT DISTINCT ON (instrument_id) instrument_id, excluded FROM rd.option_exclusion_r1 ` +
		`ORDER BY instrument_id, set_at DESC, id DESC`

	// The newest $1 rows of both views: kind, key, instrument id, value, excluded, set_at.
	historySQL = `SELECT kind, param_key, instrument_id, value, excluded, set_at FROM (` +
		`SELECT 'param' AS kind, param_key, 0::bigint AS instrument_id, value, false AS excluded, set_at, id FROM rd.rule_param_r1 ` +
		`UNION ALL ` +
		`SELECT 'exclusion', '', instrument_id, '', excluded, set_at, id FROM rd.option_exclusion_r1) h ` +
		`ORDER BY set_at DESC, id DESC, kind LIMIT $1`

	// The newest row id of both views not after $1, 0 when there is none.
	versionSQL = `SELECT GREATEST(` +
		`COALESCE((SELECT max(id) FROM rd.rule_param_r1 WHERE set_at <= $1), 0), ` +
		`COALESCE((SELECT max(id) FROM rd.option_exclusion_r1 WHERE set_at <= $1), 0))`
)

// Params returns the newest row per key not after at, sorted by key; none gives an empty slice.
func (s *Store) Params(ctx context.Context, at time.Time) ([]ownerset.Param, error) {
	out := []ownerset.Param{}
	err := s.db.Select(ctx, paramsSQL, []any{at}, func(sc scanner) error {
		var p ownerset.Param
		if err := sc.Scan(&p.Key, &p.Value, &p.SetAt); err != nil {
			return err
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("ownersetpg: params: %w", err))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Exclusions returns the newest row per instrument not after at, sorted by id; none gives an empty slice.
func (s *Store) Exclusions(ctx context.Context, at time.Time) ([]ownerset.Exclusion, error) {
	out := []ownerset.Exclusion{}
	err := s.db.Select(ctx, exclusionsSQL, []any{at}, func(sc scanner) error {
		var e ownerset.Exclusion
		if err := sc.Scan(&e.InstrumentID, &e.Excluded, &e.SetAt); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("ownersetpg: exclusions: %w", err))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstrumentID < out[j].InstrumentID })
	return out, nil
}

// History returns the newest n rows of both views, newest first; n below 1 gives an empty slice.
func (s *Store) History(ctx context.Context, n int) ([]ownerset.HistoryRow, error) {
	out := []ownerset.HistoryRow{}
	if n < 1 {
		return out, nil
	}
	err := s.db.Select(ctx, historySQL, []any{n}, func(sc scanner) error {
		var h ownerset.HistoryRow
		if err := sc.Scan(&h.Kind, &h.Key, &h.InstrumentID, &h.Value, &h.Excluded, &h.SetAt); err != nil {
			return err
		}
		out = append(out, h)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("ownersetpg: history: %w", err))
	}
	return out, nil
}

// Version returns the newest row id of both views not after at, 0 when there is none.
func (s *Store) Version(ctx context.Context, at time.Time) (int64, error) {
	var v int64
	err := s.db.Select(ctx, versionSQL, []any{at}, func(sc scanner) error { return sc.Scan(&v) })
	if err != nil {
		return 0, retryable(fmt.Errorf("ownersetpg: version: %w", err))
	}
	return v, nil
}

// readCurrentParams returns the value in force of every key that has a row.
func readCurrentParams(ctx context.Context, q querier) (map[string]string, error) {
	m := map[string]string{}
	err := q.Select(ctx, currentParamsSQL, nil, func(sc scanner) error {
		var k, v string
		if err := sc.Scan(&k, &v); err != nil {
			return err
		}
		m[k] = v
		return nil
	})
	return m, err
}

// readCurrentExclusions returns the flag in force of every instrument that has a row.
func readCurrentExclusions(ctx context.Context, q querier) (map[int64]bool, error) {
	m := map[int64]bool{}
	err := q.Select(ctx, currentExclusionsSQL, nil, func(sc scanner) error {
		var id int64
		var x bool
		if err := sc.Scan(&id, &x); err != nil {
			return err
		}
		m[id] = x
		return nil
	})
	return m, err
}
