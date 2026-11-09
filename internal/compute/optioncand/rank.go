package optioncand

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
)

// scored carries a row with the exact numbers its order needs.
type scored struct {
	row    compute.Candidate
	yield  *big.Rat
	strike *big.Rat
}

// order sorts the rows: ok rows first by yield descending, DTE, strike and instrument id
// ascending, then the others by expiry, strike and instrument id; it ranks the ok rows.
func order(rows []scored) []compute.Candidate {
	sort.SliceStable(rows, func(i, j int) bool { return less(rows[i], rows[j]) })
	out := make([]compute.Candidate, len(rows))
	rank := 0
	for i, s := range rows {
		if s.row.Status == statusOK {
			rank++
			s.row.Rank, s.row.RankInUnderlying = rank, rank
		}
		out[i] = s.row
	}
	return out
}

func less(a, b scored) bool {
	aok, bok := a.row.Status == statusOK, b.row.Status == statusOK
	if aok != bok {
		return aok
	}
	if aok {
		if c := a.yield.Cmp(b.yield); c != 0 {
			return c > 0
		}
		if a.row.DTE != b.row.DTE {
			return a.row.DTE < b.row.DTE
		}
	} else if a.row.Expiry != b.row.Expiry {
		return a.row.Expiry < b.row.Expiry
	}
	if c := a.strike.Cmp(b.strike); c != 0 {
		return c < 0
	}
	return a.row.InstrumentID < b.row.InstrumentID
}
