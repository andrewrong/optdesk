package findingpg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

const (
	// latestRunSQL: the highest check_day, then the highest run_id, so a forced run of an old day does not
	// replace the newest day.
	latestRunSQL = `SELECT run_id, to_char(check_day, 'YYYY-MM-DD'), started_at, finished_at, rules_total, rules_unknown
FROM rd.data_check_run_r1 ORDER BY check_day DESC, run_id DESC LIMIT 1`
	runFindingsSQL = `SELECT rule, dataset, to_char(day, 'YYYY-MM-DD'), subject, severity, evidence, is_new
FROM rd.data_finding_r1 WHERE run_id = $1
ORDER BY CASE severity WHEN 'error' THEN 0 WHEN 'warn' THEN 1 ELSE 2 END, dataset, day, rule, subject`
)

type pgReader struct{ pool *pgxpool.Pool }

var _ finding.Reader = (*pgReader)(nil)

// OpenReader connects to the database named by dsn (the line of the page role). It applies no migration.
func OpenReader(ctx context.Context, dsn string) (finding.Reader, error) {
	pool, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &pgReader{pool: pool}, nil
}

var _ finding.OpenReaderFunc = OpenReader

func (r *pgReader) Close() error {
	r.pool.Close()
	return nil
}

// Latest returns the newest run and its findings; with no run a zero Run with an empty Findings.
func (r *pgReader) Latest(ctx context.Context) (finding.Run, error) {
	run := finding.Run{Findings: []finding.Finding{}}
	var day string
	err := r.pool.QueryRow(ctx, latestRunSQL).
		Scan(&run.RunID, &day, &run.StartedAt, &run.FinishedAt, &run.RulesTotal, &run.RulesUnknown)
	if errors.Is(err, pgx.ErrNoRows) {
		return finding.Run{Findings: []finding.Finding{}}, nil
	}
	if err != nil {
		return finding.Run{}, fmt.Errorf("findingpg: latest run: %w", err)
	}
	run.CheckDay = contracts.Day(day)
	rows, err := r.pool.Query(ctx, runFindingsSQL, run.RunID)
	if err != nil {
		return finding.Run{}, fmt.Errorf("findingpg: latest findings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var f finding.Finding
		var fday, sev, raw string
		if err := rows.Scan(&f.Rule, &f.Dataset, &fday, &f.Subject, &sev, &raw, &f.IsNew); err != nil {
			return finding.Run{}, fmt.Errorf("findingpg: latest findings: %w", err)
		}
		f.Day, f.Severity = contracts.Day(fday), finding.Severity(sev)
		var ev []evidenceJSON
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			return finding.Run{}, fmt.Errorf("findingpg: evidence of run %d: %w", run.RunID, err)
		}
		f.Evidence = make([]finding.Count, len(ev))
		for i, c := range ev {
			f.Evidence[i] = finding.Count{Name: c.Name, N: c.N}
		}
		run.Findings = append(run.Findings, f)
	}
	if err := rows.Err(); err != nil {
		return finding.Run{}, fmt.Errorf("findingpg: latest findings: %w", err)
	}
	return run, nil
}
