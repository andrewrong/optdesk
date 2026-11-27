package findingpg

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

const (
	// earningsChecksSQL: one row per (market, day) that has check rows; the day is the observed_on of the check.
	earningsChecksSQL = `SELECT market, to_char(observed_on, 'YYYY-MM-DD'), count(*)::int,
    (count(*) FILTER (WHERE status = 'listed'))::int, (count(*) FILTER (WHERE status = 'error'))::int
FROM rd.earnings_date_r1 WHERE observed_on BETWEEN $1::date AND $2::date
GROUP BY market, observed_on ORDER BY observed_on, market`

	// barShapesSQL: the columns are decimal text, compared as numbers.
	barShapesSQL = `SELECT to_char(session_day, 'YYYY-MM-DD'), count(*)::int,
    (count(*) FILTER (WHERE low::numeric > high::numeric OR open::numeric < low::numeric OR open::numeric > high::numeric
        OR close::numeric < low::numeric OR close::numeric > high::numeric))::int
FROM rd.price_bar_r1 WHERE basis = 'trades' AND session_day BETWEEN $1::date AND $2::date
GROUP BY session_day ORDER BY session_day`
)

func (p *pgProbe) EarningsChecks(ctx context.Context, from, to contracts.Day) ([]finding.EarningsCheck, error) {
	out := []finding.EarningsCheck{}
	rows, err := p.pool.Query(ctx, earningsChecksSQL, string(from), string(to))
	if err != nil {
		return nil, fmt.Errorf("findingpg: earnings checks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e finding.EarningsCheck
		var day string
		if err := rows.Scan(&e.Market, &day, &e.Names, &e.Listed, &e.Errors); err != nil {
			return nil, fmt.Errorf("findingpg: earnings checks: %w", err)
		}
		e.Day = contracts.Day(day)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("findingpg: earnings checks: %w", err)
	}
	return out, nil
}

func (p *pgProbe) BarShapes(ctx context.Context, from, to contracts.Day) ([]finding.BarShape, error) {
	out := []finding.BarShape{}
	rows, err := p.pool.Query(ctx, barShapesSQL, string(from), string(to))
	if err != nil {
		return nil, fmt.Errorf("findingpg: bar shapes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var b finding.BarShape
		var day string
		if err := rows.Scan(&day, &b.Rows, &b.Bad); err != nil {
			return nil, fmt.Errorf("findingpg: bar shapes: %w", err)
		}
		b.Day = contracts.Day(day)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("findingpg: bar shapes: %w", err)
	}
	return out, nil
}
