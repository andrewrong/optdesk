package option_data_page

import (
	"context"
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var rightText = map[string]string{"C": "call", "P": "put"}

// applyTerms asks the reader once for the terms of the kept rows (ids in row order) and fills
// expiry, right and strike of the rows that have a record; the others keep their null values.
func applyTerms(ctx context.Context, r store.OptionTermsReader, kept []contracts.OptionEod, rows []chainRow) error {
	if r == nil || len(kept) == 0 {
		return nil
	}
	ids := make([]int64, len(kept))
	for i, k := range kept {
		ids[i] = int64(k.InstrumentID)
	}
	terms, err := r.OptionTerms(ctx, ids)
	if err != nil {
		return fmt.Errorf("option_data_page: read option terms: %w", err)
	}
	byID := make(map[int64]store.OptionTerms, len(terms))
	for _, t := range terms {
		byID[t.InstrumentID] = t
	}
	for i := range rows {
		t, ok := byID[rows[i].InstrumentID]
		if !ok {
			continue
		}
		right, ok := rightText[t.Right]
		if !ok {
			return fmt.Errorf("option_data_page: instrument %d has right %q", t.InstrumentID, t.Right)
		}
		strike, err := strconv.ParseFloat(t.Strike, 64)
		if err != nil {
			return fmt.Errorf("option_data_page: instrument %d has strike %q: %w", t.InstrumentID, t.Strike, err)
		}
		expiry := string(t.ExpirationDate)
		rows[i].Expiry, rows[i].Right, rows[i].Strike = &expiry, &right, priceCell(&strike)
	}
	return nil
}
