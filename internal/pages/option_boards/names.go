package option_boards

import (
	"context"
	"fmt"
	"sort"

	"github.com/andrewrong/optdesk/internal/store"
)

// displayNames maps an underlying instrument id to its display name, the ticker. Only non-empty names are kept.
type displayNames map[int]string

// loadNames makes one InstrumentNames call for the distinct underlying ids of rows. A nil reader or no rows
// gives an empty map and no call; a reader error is an error.
func loadNames(ctx context.Context, r store.InstrumentNameReader, rows []store.Candidate) (displayNames, error) {
	out := displayNames{}
	if r == nil {
		return out, nil
	}
	seen := map[int]bool{}
	ids := []int64{}
	for _, c := range rows {
		if !seen[c.UnderlyingID] {
			seen[c.UnderlyingID] = true
			ids = append(ids, int64(c.UnderlyingID))
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	names, err := r.InstrumentNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("option_boards: names of %d underlyings: %w", len(ids), err)
	}
	for _, n := range names {
		if n.Name != "" && seen[int(n.InstrumentID)] {
			out[int(n.InstrumentID)] = n.Name
		}
	}
	return out, nil
}

// of is the display name of the row's company: the ticker, else the stored issuer text.
func (n displayNames) of(c store.Candidate) string {
	if name, ok := n[c.UnderlyingID]; ok {
		return name
	}
	return c.Issuer
}

// ticker is the stored name of an underlying, or nil when there is none.
func (n displayNames) ticker(id int) *string {
	if name, ok := n[id]; ok {
		return strPtr(name)
	}
	return nil
}
