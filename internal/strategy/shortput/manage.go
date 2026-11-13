package shortput

import (
	"math/big"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
)

const dayLayout = "2006-01-02"

// Manage holds to expiry in close_mode expiry. In early it closes with reason early_keep when the share of the
// credit kept, (OpenPrice - ask) / OpenPrice, is at least early_keep at the ask of the position's contract in the
// snapshot quotes, else with early_dte when the days from RunDay to Expiry are at most early_dte, else holds.
func (s *shortPut) Manage(p strategy.Position, snap strategy.Snapshot) strategy.Action {
	hold := strategy.Action{Kind: "hold"}
	if s.closeMode != CloseEarly {
		return hold
	}
	if open, ok := parseDecimal(p.OpenPrice); ok && open.Sign() > 0 {
		for _, q := range snap.Quotes {
			if q.InstrumentID != p.Contract {
				continue
			}
			if ask, ok := parseDecimal(q.Ask); ok {
				kept := new(big.Rat).Sub(open, ask)
				kept.Quo(kept, open)
				if kept.Cmp(s.earlyKeep) >= 0 {
					return strategy.Action{Kind: "close", Reason: "early_keep"}
				}
			}
			break
		}
	}
	if days, ok := dayDiff(snap.RunDay, p.Expiry); ok && days <= s.earlyDTE {
		return strategy.Action{Kind: "close", Reason: "early_dte"}
	}
	return hold
}

func dayDiff(from, to contracts.Day) (int, bool) {
	a, err1 := time.Parse(dayLayout, string(from))
	b, err2 := time.Parse(dayLayout, string(to))
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return int(b.Sub(a).Hours() / 24), true
}
