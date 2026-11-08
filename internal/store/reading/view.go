package reading

import (
	"context"
	"fmt"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenReadingViewFunc = OpenView

// Both statements read views only, so a connection of ot_ro_pages (or ot_w_readings) needs no grant on a table.
const (
	// selectWinnerSQL reads the winner of each (day, version), the higher version of a day first; the reader keeps
	// the first row of each day. Inside a version the view rd.reading_r1 has chosen the winner (highest ok rev, else
	// highest rev).
	selectWinnerSQL = `SELECT name, version, subject_kind, subject, day, rev, value::text, unit::text, ` +
		`currency, state, reason_code, origin, cutoff, job_run_id::text, recorded_at, rows_sha256 ` +
		`FROM rd.reading_r1 ` +
		`WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5 ` +
		`ORDER BY day, version DESC`
	selectViewParametersSQL = `SELECT version, day, rev, param, value_num, value_tag FROM rd.reading_parameter_r1 ` +
		`WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5 ` +
		`ORDER BY day, version, rev, param`
)

type viewStore struct{ db database }

// OpenView connects to the database named by dsn and returns the winner reader over the views rd.reading_r1 and
// rd.reading_parameter_r1. The caller chooses the DSN (OT_PG_DSN_PAGES for the page process); nothing is read from
// the environment here. It applies no migration.
func OpenView(ctx context.Context, dsn string) (store.ReadingViewReader, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("reading: open view: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("reading: ping view: %w", err))
	}
	return &viewStore{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *viewStore) Close() error { return s.db.Close() }

// WinnerReadings returns one row per day from..to (inclusive), ascending by day: the highest version of the day,
// then its highest-rev ok row, else its highest-rev row. Rows carry the parameters, never the inputs. A day with no
// reading is absent; none gives an empty slice and no error. from after to is a Permanent error. Instants are UTC.
func (s *viewStore) WinnerReadings(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	if f.After(t) {
		return nil, permanent(fmt.Errorf("reading: range %s..%s is reversed", from, to))
	}
	args := []any{name, subjectKind, subject, f, t}
	out := []contracts.ReadingRecord{}
	at := map[rkey]int{}
	err = s.db.Select(ctx, selectWinnerSQL, args, func(sc scanner) error {
		var r contracts.ReadingRecord
		var day time.Time
		var unit, state, origin string
		if err := sc.Scan(&r.Name, &r.Version, &r.SubjectKind, &r.Subject, &day, &r.Rev, &r.Value, &unit, &r.Currency,
			&state, &r.ReasonCode, &origin, &r.Cutoff, &r.JobRunID, &r.RecordedAt, &r.RowsSha256); err != nil {
			return err
		}
		if n := len(out); n > 0 && out[n-1].Day.Equal(day) {
			return nil // a lower version of a day already taken
		}
		r.Day = types.SerializableDate{Time: day}
		setText(&r.Unit, unit)
		setText(&r.State, state)
		setText(&r.Origin, origin)
		r.RecordedAt = r.RecordedAt.UTC()
		r.Inputs, r.Parameters = nonNil(r.Inputs), nonNil(r.Parameters)
		at[rkey{r.Version, day, r.Rev}] = len(out)
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("reading: select winners: %w", err))
	}
	if len(out) == 0 {
		return out, nil
	}
	err = s.db.Select(ctx, selectViewParametersSQL, args, func(sc scanner) error {
		var k rkey
		var param string
		var num *float64
		var tag *string
		if err := sc.Scan(&k.version, &k.day, &k.rev, &param, &num, &tag); err != nil {
			return err
		}
		if i, ok := at[k]; ok {
			p := zero(out[i].Parameters)
			p.Param, p.ValueNum, p.ValueTag = param, num, tag
			out[i].Parameters = append(out[i].Parameters, p)
		}
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("reading: select parameters: %w", err))
	}
	return out, nil
}
