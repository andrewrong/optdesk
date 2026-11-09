package optiontrack

import (
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func reconKey(underlying string, expiry contracts.Day, strike, right string) string {
	return underlying + "|" + string(expiry) + "|" + strikeKey(strike) + "|" + side(right)
}

// reconcile compares the open short positions of the replay with the short options the
// account holds (Decision 3). It never changes a row.
func reconcile(open []compute.LedgerPosition, held []trackapi.TrackHeld) []trackapi.TrackRecon {
	inFlex := map[string]bool{}
	for _, p := range open {
		inFlex[reconKey(p.Underlying, contracts.Day(p.Expiry), p.Strike, p.Right)] = true
	}
	inAccount := map[string]bool{}
	var out []trackapi.TrackRecon
	for _, h := range held {
		q, ok := dec(h.Quantity)
		if !ok || q.Sign() >= 0 {
			continue
		}
		k := reconKey(h.Underlying, h.Expiry, h.Strike, h.Right)
		inAccount[k] = true
		if !inFlex[k] {
			out = append(out, trackapi.TrackRecon{
				Underlying: h.Underlying, Right: h.Right, Strike: h.Strike, Quantity: h.Quantity,
				Expiry: h.Expiry, Code: "not_in_flex",
			})
		}
	}
	for _, p := range open {
		if !inAccount[reconKey(p.Underlying, contracts.Day(p.Expiry), p.Strike, p.Right)] {
			out = append(out, trackapi.TrackRecon{
				Underlying: p.Underlying, Right: p.Right, Strike: p.Strike, Quantity: p.Quantity,
				Expiry: contracts.Day(p.Expiry), Code: "not_in_account",
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Underlying != b.Underlying {
			return a.Underlying < b.Underlying
		}
		if a.Expiry != b.Expiry {
			return a.Expiry < b.Expiry
		}
		if c := cmpStrike(a.Strike, b.Strike); c != 0 {
			return c < 0
		}
		if a.Right != b.Right {
			return a.Right < b.Right
		}
		return a.Code < b.Code
	})
	return out
}
