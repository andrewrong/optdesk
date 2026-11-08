package reading

import (
	"context"
	"fmt"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The three statements share their arguments: name, subject_kind, subject, from, to.
const (
	// selectLatestSQL takes the newest rev of each day, whatever its state; of two versions the higher wins.
	selectLatestSQL = `SELECT DISTINCT ON (day) name, version, subject_kind, subject, day, rev, value::text, unit::text, ` +
		`currency, state, reason_code, origin, cutoff, job_run_id::text, recorded_at, rows_sha256 ` +
		`FROM readings.reading ` +
		`WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5 ` +
		`ORDER BY day, version DESC, rev DESC`
	selectInputsSQL = `SELECT version, day, rev, ord, input_table, code_version, convention_version ` +
		`FROM readings.reading_input ` +
		`WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5 ` +
		`ORDER BY day, version, rev, ord`
	selectInputKeysSQL = `SELECT version, day, rev, ord, input_key FROM readings.reading_input_key ` +
		`WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5 ` +
		`ORDER BY day, version, rev, ord, key_ord`
	selectParametersSQL = `SELECT version, day, rev, param, value_num, value_tag FROM readings.reading_parameter ` +
		`WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5 ` +
		`ORDER BY day, version, rev, param`
)

// rkey names one reading of the subject asked for.
type rkey struct {
	version int
	day     time.Time
	rev     int
}

// LatestReadings returns the newest rev of each day from..to (inclusive) of one figure and subject, whatever its
// state, ascending by day, each with its inputs and parameters. A day with no reading is absent; none gives an empty
// slice and no error. Instants are returned in UTC.
func (s *readingStore) LatestReadings(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	args := []any{name, subjectKind, subject, f, t}
	out := []contracts.ReadingRecord{}
	at := map[rkey]int{}
	err = s.db.Select(ctx, selectLatestSQL, args, func(sc scanner) error {
		var r contracts.ReadingRecord
		var day time.Time
		var unit, state, origin string
		if err := sc.Scan(&r.Name, &r.Version, &r.SubjectKind, &r.Subject, &day, &r.Rev, &r.Value, &unit, &r.Currency,
			&state, &r.ReasonCode, &origin, &r.Cutoff, &r.JobRunID, &r.RecordedAt, &r.RowsSha256); err != nil {
			return err
		}
		r.Day = types.SerializableDate{Time: day}
		setText(&r.Unit, unit)
		setText(&r.State, state)
		setText(&r.Origin, origin)
		r.RecordedAt = r.RecordedAt.UTC()
		at[rkey{r.Version, day, r.Rev}] = len(out)
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("reading: select: %w", err))
	}
	if len(out) == 0 {
		return out, nil
	}
	if err := s.loadChildren(ctx, args, out, at); err != nil {
		return nil, retryable(fmt.Errorf("reading: select children: %w", err))
	}
	for i := range out {
		out[i].Inputs = nonNil(out[i].Inputs)
		out[i].Parameters = nonNil(out[i].Parameters)
	}
	return out, nil
}

// loadChildren attaches the inputs, their keys and the parameters of the readings named by at.
func (s *readingStore) loadChildren(ctx context.Context, args []any, out []contracts.ReadingRecord, at map[rkey]int) error {
	err := s.db.Select(ctx, selectInputsSQL, args, func(sc scanner) error {
		var k rkey
		var ord int
		var table, code, conv string
		if err := sc.Scan(&k.version, &k.day, &k.rev, &ord, &table, &code, &conv); err != nil {
			return err
		}
		if i, ok := at[k]; ok {
			in := zero(out[i].Inputs)
			in.InputTable, in.CodeVersion, in.ConventionVersion, in.Keys = table, code, conv, []string{}
			out[i].Inputs = append(out[i].Inputs, in) // ord ascending: the index of the input is ord
		}
		return nil
	})
	if err == nil {
		err = s.db.Select(ctx, selectInputKeysSQL, args, func(sc scanner) error {
			var k rkey
			var ord int
			var key string
			if err := sc.Scan(&k.version, &k.day, &k.rev, &ord, &key); err != nil {
				return err
			}
			if i, ok := at[k]; ok && ord < len(out[i].Inputs) {
				out[i].Inputs[ord].Keys = append(out[i].Inputs[ord].Keys, key)
			}
			return nil
		})
	}
	if err == nil {
		err = s.db.Select(ctx, selectParametersSQL, args, func(sc scanner) error {
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
	}
	return err
}

// parseDay turns a contracts.Day into the date it names.
func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return time.Time{}, permanent(fmt.Errorf("reading: bad day %q: %w", d, err))
	}
	return t, nil
}

// setText stores s in a string-based enum of the record; the generated enum types are not exported by contracts.
func setText[T ~string](dst *T, s string) { *dst = T(s) }

// zero returns the zero element of a slice of a generated type that contracts does not name.
func zero[T any](_ []T) T {
	var z T
	return z
}

// nonNil returns s, or an empty slice for nil, so the record marshals to [] and not null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
