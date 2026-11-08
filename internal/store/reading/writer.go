package reading

import (
	"context"
	"fmt"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// insertReadingSQL inserts one reading; a key that exists is a conflict and adds no row. Text casts carry the decimal,
// the uuid and the enum, so the driver sends plain text. Arguments: name, version, subject_kind, subject, day, rev,
// value, unit, currency, state, reason_code, origin, cutoff, job_run_id, recorded_at, rows_sha256.
const insertReadingSQL = `INSERT INTO readings.reading (name, version, subject_kind, subject, day, rev, value, unit, ` +
	`currency, state, reason_code, origin, cutoff, job_run_id, recorded_at, rows_sha256) ` +
	`VALUES ($1, $2, $3, $4, $5, $6, $7::text::numeric, $8::text::readings.reading_unit, $9, $10, $11, $12, $13, ` +
	`$14::text::uuid, $15, $16) ON CONFLICT DO NOTHING`

// insertInputSQL arguments: the six key columns, ord, input_table, code_version, convention_version.
const insertInputSQL = `INSERT INTO readings.reading_input (name, version, subject_kind, subject, day, rev, ord, ` +
	`input_table, code_version, convention_version) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`

// insertInputKeySQL arguments: the six key columns, ord, key_ord, input_key.
const insertInputKeySQL = `INSERT INTO readings.reading_input_key (name, version, subject_kind, subject, day, rev, ord, ` +
	`key_ord, input_key) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT DO NOTHING`

// insertParameterSQL arguments: the six key columns, param, value_num, value_tag.
const insertParameterSQL = `INSERT INTO readings.reading_parameter (name, version, subject_kind, subject, day, rev, ` +
	`param, value_num, value_tag) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT DO NOTHING`

// InsertReadings inserts each reading with its inputs, input keys and parameters, all in one transaction. A reading
// whose key (name, version, subject_kind, subject, day, rev) exists is left as it is, children included, and is not
// counted. An empty input makes no database call.
func (s *readingStore) InsertReadings(ctx context.Context, rows []contracts.ReadingRecord) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	inserted := 0
	err := s.db.InTx(ctx, func(tx execer) error {
		inserted = 0 // the closure may not rerun, but a count of a rolled-back try must never leak
		for _, r := range rows {
			n, err := insertOne(ctx, tx, r)
			if err != nil {
				return err
			}
			inserted += n
		}
		return nil
	})
	if err != nil {
		return 0, retryable(fmt.Errorf("reading: insert: %w", err))
	}
	return inserted, nil
}

// insertOne inserts one reading and, when it was new, its children; it returns 1 for a new reading, else 0.
func insertOne(ctx context.Context, tx execer, r contracts.ReadingRecord) (int, error) {
	day := r.Day.Time
	var value any
	if r.Value != nil {
		value = *r.Value
	}
	n, err := tx.Exec(ctx, insertReadingSQL, r.Name, r.Version, r.SubjectKind, r.Subject, day, r.Rev, value,
		string(r.Unit), r.Currency, string(r.State), r.ReasonCode, string(r.Origin), r.Cutoff, r.JobRunID,
		r.RecordedAt.UTC(), r.RowsSha256)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	key := []any{r.Name, r.Version, r.SubjectKind, r.Subject, day, r.Rev}
	for i, in := range r.Inputs {
		if _, err := tx.Exec(ctx, insertInputSQL, append(key[:6:6], i, in.InputTable, in.CodeVersion, in.ConventionVersion)...); err != nil {
			return 0, err
		}
		for j, k := range in.Keys {
			if _, err := tx.Exec(ctx, insertInputKeySQL, append(key[:6:6], i, j, k)...); err != nil {
				return 0, err
			}
		}
	}
	for _, p := range r.Parameters {
		if _, err := tx.Exec(ctx, insertParameterSQL, append(key[:6:6], p.Param, p.ValueNum, p.ValueTag)...); err != nil {
			return 0, err
		}
	}
	return 1, nil
}
