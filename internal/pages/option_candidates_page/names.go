package option_candidates_page

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/store"
)

// withDisplayNames returns a copy of rows whose Issuer holds the ticker of the row's underlying, read
// with one InstrumentNames call for the distinct underlying ids. A nil names, an id with no name or an
// empty name keeps the stored issuer text; a reader error is returned. The copy is display-only: it is
// never stored or compared, and rows is not changed.
func withDisplayNames(ctx context.Context, names store.InstrumentNameReader, rows []store.Candidate) ([]store.Candidate, error) {
	if names == nil || len(rows) == 0 {
		return rows, nil
	}
	seen := map[int]bool{}
	var ids []int64
	for _, r := range rows {
		if r.UnderlyingID > 0 && !seen[r.UnderlyingID] {
			seen[r.UnderlyingID] = true
			ids = append(ids, int64(r.UnderlyingID))
		}
	}
	if len(ids) == 0 {
		return rows, nil
	}
	got, err := names.InstrumentNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("option_candidates_page: read names: %w", err)
	}
	byID := make(map[int]string, len(got))
	for _, n := range got {
		if n.Name != "" {
			byID[int(n.InstrumentID)] = n.Name
		}
	}
	out := make([]store.Candidate, len(rows))
	copy(out, rows)
	for i := range out {
		if n, ok := byID[out[i].UnderlyingID]; ok {
			out[i].Issuer = n
		}
	}
	return out, nil
}
