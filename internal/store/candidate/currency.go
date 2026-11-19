package candidate

import (
	"context"
	"fmt"
	"regexp"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
)

var (
	_ candidatecur.Reader = (*Store)(nil)
	_ candidatecur.Writer = (*Store)(nil)
)

// insertInSQL is insertSQL with the currency as the last column.
var insertInSQL = "INSERT INTO candidates.option_candidate (" + columns + ", currency) VALUES (" +
	placeholders(columnCount+1) + ") ON CONFLICT DO NOTHING"

const (
	selectInSQL = `SELECT ` + columns + ` FROM rd.option_candidate_r1 WHERE session_day = $1 AND kind = $2 AND ` +
		`currency = $3 ORDER BY rank ASC NULLS LAST, instrument_id, alias, run_id`
	// selectUSDInSQL also keeps the rows with a null currency, which read as USD.
	selectUSDInSQL = `SELECT ` + columns + ` FROM rd.option_candidate_r1 WHERE session_day = $1 AND kind = $2 AND ` +
		`(currency IS NULL OR currency = $3) ORDER BY rank ASC NULLS LAST, instrument_id, alias, run_id`
)

var currencyRE = regexp.MustCompile(`^[A-Z]{3}$`)

// CandidatesIn returns the rows of one kind of one day written by the newest run of that currency; see
// candidatecur.Reader. The query keeps one currency, so the newest run is picked among the rows of that currency.
func (s *Store) CandidatesIn(ctx context.Context, day contracts.Day, kind, currency string) ([]candidatecur.Row, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	if !validKind(kind) {
		return nil, permanent(fmt.Errorf("candidate: unknown kind %q", kind))
	}
	if !currencyRE.MatchString(currency) {
		return nil, permanent(fmt.Errorf("candidate: bad currency %q", currency))
	}
	q := selectInSQL
	if currency == "USD" {
		q = selectUSDInSQL
	}
	var all []store.Candidate
	err = s.db.Select(ctx, q, []any{d, kind, currency}, func(sc scanner) error {
		c, err := scanRow(sc)
		if err != nil {
			return err
		}
		all = append(all, c)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("candidate: select: %w", err))
	}
	newest := newestRun(all)
	out := make([]candidatecur.Row, 0, len(newest))
	for _, c := range newest {
		out = append(out, candidatecur.Row{Candidate: c, Currency: currency})
	}
	return out, nil
}

// InsertCandidatesIn inserts the rows with their currency; see candidatecur.Writer.
func (s *Store) InsertCandidatesIn(ctx context.Context, rows []candidatecur.Row) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	args := make([][]any, 0, len(rows))
	for i, r := range rows {
		if !currencyRE.MatchString(r.Currency) {
			return 0, permanent(fmt.Errorf("candidate: row %d: bad currency %q", i, r.Currency))
		}
		a, err := rowArgs(r.Candidate)
		if err != nil {
			return 0, fmt.Errorf("candidate: row %d: %w", i, err)
		}
		args = append(args, append(a, r.Currency))
	}
	n, err := s.db.InsertAll(ctx, insertInSQL, args)
	if err != nil {
		err = fmt.Errorf("candidate: insert: %w", err)
		if isConstraintViolation(err) {
			return 0, permanent(err)
		}
		return 0, retryable(err)
	}
	return n, nil
}
