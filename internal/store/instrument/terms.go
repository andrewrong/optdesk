package instrument

import (
	"context"
	"fmt"
	"slices"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenOptionTermsFunc = OpenOptionTerms

// maxTermsIDs is the most ids one OptionTerms call accepts.
const maxTermsIDs = 500

// termsSQL reads the current versions through the view; decimals come back as text and the date as YYYY-MM-DD.
const termsSQL = `SELECT terms_version_id, instrument_id, "right", strike::text, strike_currency, multiplier::text,
       to_char(expiration_date, 'YYYY-MM-DD'), exercise_style, settlement_type, valid_from
FROM rd.option_terms_r1
WHERE instrument_id = ANY($1)
ORDER BY instrument_id`

// optionTermsStore is the store.OptionTermsStore over a database seam.
type optionTermsStore struct{ db database }

// OpenOptionTerms connects to the database named by dsn (the line store.DSNEnvPages, role ot_ro_pages) and returns
// the option terms store. It applies no migration.
func OpenOptionTerms(ctx context.Context, dsn string) (store.OptionTermsStore, error) {
	db, err := openDB(ctx, "option terms", dsn)
	if err != nil {
		return nil, err
	}
	return &optionTermsStore{db: db}, nil
}

// Close releases the connections.
func (s *optionTermsStore) Close() error { return s.db.Close() }

// OptionTerms returns the current version of each id, ordered by InstrumentID; an id without one is absent. An empty
// ids makes no database call; more than 500 ids or an id of 0 is a Permanent error.
func (s *optionTermsStore) OptionTerms(ctx context.Context, ids []int64) ([]store.OptionTerms, error) {
	if len(ids) > maxTermsIDs {
		return nil, permanent(fmt.Errorf("instrument: option terms: %d ids, at most %d", len(ids), maxTermsIDs))
	}
	if slices.Contains(ids, 0) {
		return nil, permanent(fmt.Errorf("instrument: option terms: instrument id 0"))
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var out []store.OptionTerms
	err := s.db.Select(ctx, termsSQL, []any{ids}, func(r scanner) error {
		var o store.OptionTerms
		var day string
		if err := r.Scan(&o.TermsVersionID, &o.InstrumentID, &o.Right, &o.Strike, &o.StrikeCurrency, &o.Multiplier,
			&day, &o.ExerciseStyle, &o.SettlementType, &o.ValidFrom); err != nil {
			return err
		}
		o.ExpirationDate = contracts.Day(day)
		o.ValidFrom = o.ValidFrom.UTC()
		out = append(out, o)
		return nil
	})
	if err != nil {
		return nil, asTyped(err, "option terms")
	}
	slices.SortFunc(out, func(a, b store.OptionTerms) int {
		switch {
		case a.InstrumentID < b.InstrumentID:
			return -1
		case a.InstrumentID > b.InstrumentID:
			return 1
		}
		return 0
	})
	return out, nil
}
