package reading

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

// upsertReadingSQL writes one reading by its key (docs/design/readings.md Decision 4). A stored row is replaced only
// when one of the nine compared columns differs and the new row is not a missing_input over a stored ok; recorded_at
// and job_run_id are not compared, or every run would differ. RETURNING 1 yields a row only for a written reading.
// Arguments as insertReadingSQL.
const upsertReadingSQL = `INSERT INTO readings.reading AS r (name, version, subject_kind, subject, day, rev, value, unit, ` +
	`currency, state, reason_code, origin, cutoff, job_run_id, recorded_at, rows_sha256) ` +
	`VALUES ($1, $2, $3, $4, $5, $6, $7::text::numeric, $8::text::readings.reading_unit, $9, $10, $11, $12, $13, ` +
	`$14::text::uuid, $15, $16) ON CONFLICT (name, version, subject_kind, subject, day) DO UPDATE SET ` +
	`value = EXCLUDED.value, unit = EXCLUDED.unit, currency = EXCLUDED.currency, state = EXCLUDED.state, ` +
	`reason_code = EXCLUDED.reason_code, origin = EXCLUDED.origin, cutoff = EXCLUDED.cutoff, ` +
	`rows_sha256 = EXCLUDED.rows_sha256, rev = EXCLUDED.rev, recorded_at = EXCLUDED.recorded_at, ` +
	`job_run_id = EXCLUDED.job_run_id ` +
	`WHERE (r.value, r.unit, r.currency, r.state, ` +
	`r.reason_code, r.origin, r.cutoff, r.rows_sha256, ` +
	`r.rev) IS DISTINCT FROM (EXCLUDED.value, EXCLUDED.unit, EXCLUDED.currency, EXCLUDED.state, ` +
	`EXCLUDED.reason_code, EXCLUDED.origin, EXCLUDED.cutoff, EXCLUDED.rows_sha256, EXCLUDED.rev) ` +
	`AND NOT (r.state = 'ok' AND EXCLUDED.state = 'missing_input') RETURNING 1`

// The deletes of the children of a replaced reading, children first. Arguments: name, version, subject_kind, subject, day.
const (
	deleteInputKeySQL = `DELETE FROM readings.reading_input_key WHERE name = $1 AND version = $2 AND ` +
		`subject_kind = $3 AND subject = $4 AND day = $5`
	deleteInputSQL = `DELETE FROM readings.reading_input WHERE name = $1 AND version = $2 AND ` +
		`subject_kind = $3 AND subject = $4 AND day = $5`
	deleteParameterSQL = `DELETE FROM readings.reading_parameter WHERE name = $1 AND version = $2 AND ` +
		`subject_kind = $3 AND subject = $4 AND day = $5`
)

// readingKey is the key of a reading; the day is the calendar date, so one date in two zones is one key.
type readingKey struct {
	name        string
	version     int
	subjectKind string
	subject     string
	day         string
}

var _ readingkey.ReadingUpserter = (*readingStore)(nil)

// UpsertReadings writes each reading by the key (name, version, subject_kind, subject, day) in one transaction: a
// differing reading replaces the stored one and its children are deleted and written again, an equal one or a
// missing_input over a stored ok is skipped. A batch with one key twice is a Permanent error before any statement; an
// empty list makes no call.
func (s *readingStore) UpsertReadings(ctx context.Context, rows []contracts.ReadingRecord) (readingkey.WriteCount, error) {
	if len(rows) == 0 {
		return readingkey.WriteCount{}, nil
	}
	seen := make(map[readingKey]bool, len(rows))
	for _, r := range rows {
		k := readingKey{r.Name, r.Version, r.SubjectKind, r.Subject, r.Day.Format(time.DateOnly)}
		if seen[k] {
			return readingkey.WriteCount{}, permanent(fmt.Errorf("reading: upsert: key (%s, %d, %s, %s, %s) twice in one call",
				r.Name, r.Version, r.SubjectKind, r.Subject, k.day))
		}
		seen[k] = true
	}
	var out readingkey.WriteCount
	err := s.db.InTx(ctx, func(tx execer) error {
		out = readingkey.WriteCount{}
		for _, r := range rows {
			written, err := upsertOne(ctx, tx, r)
			if err != nil {
				return err
			}
			if written {
				out.Written++
			} else {
				out.Skipped++
			}
		}
		return nil
	})
	if err != nil {
		return readingkey.WriteCount{}, retryable(fmt.Errorf("reading: upsert: %w", err))
	}
	return out, nil
}

// upsertOne writes one reading and, when it was written, replaces its children.
func upsertOne(ctx context.Context, tx execer, r contracts.ReadingRecord) (bool, error) {
	day := r.Day.Time
	var value any
	if r.Value != nil {
		value = *r.Value
	}
	n, err := tx.Exec(ctx, upsertReadingSQL, r.Name, r.Version, r.SubjectKind, r.Subject, day, r.Rev, value,
		string(r.Unit), r.Currency, string(r.State), r.ReasonCode, string(r.Origin), r.Cutoff, r.JobRunID,
		r.RecordedAt.UTC(), r.RowsSha256)
	if err != nil || n == 0 {
		return false, err
	}
	key := []any{r.Name, r.Version, r.SubjectKind, r.Subject, day}
	for _, sql := range []string{deleteInputKeySQL, deleteInputSQL, deleteParameterSQL} {
		if _, err := tx.Exec(ctx, sql, key...); err != nil {
			return false, err
		}
	}
	// The children carry rev, a plain column equal to the parent's.
	child := append(key[:5:5], r.Rev)
	for i, in := range r.Inputs {
		if _, err := tx.Exec(ctx, insertInputSQL, append(child[:6:6], i, in.InputTable, in.CodeVersion, in.ConventionVersion)...); err != nil {
			return false, err
		}
		for j, k := range in.Keys {
			if _, err := tx.Exec(ctx, insertInputKeySQL, append(child[:6:6], i, j, k)...); err != nil {
				return false, err
			}
		}
	}
	for _, p := range r.Parameters {
		if _, err := tx.Exec(ctx, insertParameterSQL, append(child[:6:6], p.Param, p.ValueNum, p.ValueTag)...); err != nil {
			return false, err
		}
	}
	return true, nil
}
