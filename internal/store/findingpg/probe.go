package findingpg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

// readingLookbackDays is how far before the window a reading is searched for the "previous day that has one" of a
// subject; it covers a long weekend with a holiday on each side.
const readingLookbackDays = 14

const (
	// repeatedReadingsSQL: per (name, day) the rows of the highest version that has a row for the day, ok rows only,
	// each compared with the same subject's newest earlier ok row of that name (the highest version of its own day).
	repeatedReadingsSQL = `WITH chosen AS (
    SELECT name, subject_kind, subject, day, state, value
    FROM (SELECT r.*, max(r.version) OVER (PARTITION BY r.name, r.day) AS top
          FROM rd.reading_r1 r
          WHERE r.name = ANY($1) AND r.day BETWEEN $2::date - $4::int AND $3::date) ranked
    WHERE version = top
), ok AS (
    SELECT name, subject_kind, subject, day, value FROM chosen WHERE state = 'ok' AND value IS NOT NULL
)
SELECT c.name, to_char(c.day, 'YYYY-MM-DD'), count(*)::int, (count(*) FILTER (WHERE p.value = c.value))::int
FROM ok c
LEFT JOIN LATERAL (
    SELECT o.value FROM ok o
    WHERE o.name = c.name AND o.subject_kind = c.subject_kind AND o.subject = c.subject AND o.day < c.day
    ORDER BY o.day DESC LIMIT 1) p ON true
WHERE c.day >= $2::date
GROUP BY c.name, c.day ORDER BY c.name, c.day`

	// lateOpenQuotesSQL: a currency with no zone gives a NULL zone, so its rows count in rows and never in late.
	lateOpenQuotesSQL = `SELECT to_char(session_day, 'YYYY-MM-DD'), currency, count(*)::int,
    (count(*) FILTER (WHERE (received_at AT TIME ZONE CASE currency
        WHEN 'USD' THEN 'America/New_York' WHEN 'JPY' THEN 'Asia/Tokyo' END)::date > session_day))::int
FROM rd.option_open_r1 WHERE session_day BETWEEN $1::date AND $2::date
GROUP BY session_day, currency ORDER BY session_day, currency`

	newestCandidateRunsSQL = `SELECT DISTINCT ON (session_day, kind) to_char(session_day, 'YYYY-MM-DD'), kind, n, computed_at
FROM (SELECT session_day, kind, run_id, count(*)::int AS n, max(computed_at) AS computed_at
      FROM rd.option_candidate_r1 WHERE session_day BETWEEN $1::date AND $2::date
      GROUP BY session_day, kind, run_id) runs
ORDER BY session_day, kind, computed_at DESC, run_id DESC`

	settingsChangedAtSQL = `SELECT max(t) FROM (SELECT max(set_at) AS t FROM rd.rule_param_r1
UNION ALL SELECT max(set_at) FROM rd.option_exclusion_r1) s`

	openGapsSQL = `SELECT job_id, dataset_id, to_char(day, 'YYYY-MM-DD'), reason, first_seen_at
FROM rd.job_gap_r1 WHERE cleared_at IS NULL ORDER BY first_seen_at, job_id, dataset_id, day`
)

type pgProbe struct{ pool *pgxpool.Pool }

var _ finding.Probe = (*pgProbe)(nil)

// OpenProbe connects to the database named by dsn (the line of the role ot_w_datacheck). It reads the rd views only
// and applies no migration.
func OpenProbe(ctx context.Context, dsn string) (finding.Probe, error) {
	pool, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &pgProbe{pool: pool}, nil
}

var _ finding.OpenProbeFunc = OpenProbe

func (p *pgProbe) Close() error {
	p.pool.Close()
	return nil
}

func (p *pgProbe) RepeatedReadings(ctx context.Context, names []string, from, to contracts.Day) ([]finding.ReadingRepeat, error) {
	out := []finding.ReadingRepeat{}
	if len(names) == 0 {
		return out, nil
	}
	rows, err := p.pool.Query(ctx, repeatedReadingsSQL, names, string(from), string(to), readingLookbackDays)
	if err != nil {
		return nil, fmt.Errorf("findingpg: repeated readings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r finding.ReadingRepeat
		var day string
		if err := rows.Scan(&r.Name, &day, &r.OK, &r.Same); err != nil {
			return nil, fmt.Errorf("findingpg: repeated readings: %w", err)
		}
		r.Day = contracts.Day(day)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("findingpg: repeated readings: %w", err)
	}
	return out, nil
}

func (p *pgProbe) LateOpenQuotes(ctx context.Context, from, to contracts.Day) ([]finding.LateQuotes, error) {
	out := []finding.LateQuotes{}
	rows, err := p.pool.Query(ctx, lateOpenQuotesSQL, string(from), string(to))
	if err != nil {
		return nil, fmt.Errorf("findingpg: late open quotes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var q finding.LateQuotes
		var day string
		if err := rows.Scan(&day, &q.Currency, &q.Rows, &q.Late); err != nil {
			return nil, fmt.Errorf("findingpg: late open quotes: %w", err)
		}
		q.Day = contracts.Day(day)
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("findingpg: late open quotes: %w", err)
	}
	return out, nil
}

func (p *pgProbe) NewestCandidateRuns(ctx context.Context, from, to contracts.Day) ([]finding.CandidateRun, error) {
	out := []finding.CandidateRun{}
	rows, err := p.pool.Query(ctx, newestCandidateRunsSQL, string(from), string(to))
	if err != nil {
		return nil, fmt.Errorf("findingpg: candidate runs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c finding.CandidateRun
		var day string
		if err := rows.Scan(&day, &c.Kind, &c.Rows, &c.ComputedAt); err != nil {
			return nil, fmt.Errorf("findingpg: candidate runs: %w", err)
		}
		c.Day = contracts.Day(day)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("findingpg: candidate runs: %w", err)
	}
	return out, nil
}

func (p *pgProbe) SettingsChangedAt(ctx context.Context) (time.Time, bool, error) {
	var at *time.Time
	if err := p.pool.QueryRow(ctx, settingsChangedAtSQL).Scan(&at); err != nil {
		return time.Time{}, false, fmt.Errorf("findingpg: settings changed at: %w", err)
	}
	if at == nil {
		return time.Time{}, false, nil
	}
	return *at, true, nil
}

func (p *pgProbe) OpenGaps(ctx context.Context) ([]finding.OpenGap, error) {
	out := []finding.OpenGap{}
	rows, err := p.pool.Query(ctx, openGapsSQL)
	if err != nil {
		return nil, fmt.Errorf("findingpg: open gaps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var g finding.OpenGap
		var day string
		if err := rows.Scan(&g.JobID, &g.Dataset, &day, &g.Reason, &g.FirstSeenAt); err != nil {
			return nil, fmt.Errorf("findingpg: open gaps: %w", err)
		}
		g.Day = contracts.Day(day)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("findingpg: open gaps: %w", err)
	}
	return out, nil
}
