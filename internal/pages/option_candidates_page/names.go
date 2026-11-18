package option_candidates_page

import (
	"github.com/andrewrong/optdesk/internal/store"
)

// namedRows is a copy of rows whose Issuer holds the ticker of the row's underlying where tickers has one.
func namedRows(rows []store.Candidate, tickers map[int]string) []store.Candidate {
	out := make([]store.Candidate, len(rows))
	copy(out, rows)
	for i := range out {
		if n := tickers[out[i].UnderlyingID]; n != "" {
			out[i].Issuer = n
		}
	}
	return out
}
