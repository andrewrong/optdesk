package optioneod

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ jobs.NewOptionEodHeldFunc = NewWithHeld

// heldAliases are the two IBKR accounts of the owner (docs/design/option-candidates.md, alias A and B).
var heldAliases = []string{"A", "B"}

// heldLookbackDays is how many calendar days before the run day the newest position capture is searched.
const heldLookbackDays = 7

// NewWithHeld is NewWithWatch that also adds the expiry and strike of every open short call of a watched
// underlying, from the newest account.position capture of each alias, to the day's contract list whatever its
// DTE (Decisions 10 and 12), so the guard verdict is not unknown by construction. A nil reader behaves as
// NewWithWatch.
func NewWithHeld(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, watch []jobs.WatchEntry,
	acct store.AccountReader, clk clock.Clock) (contracts.Job, error) {
	j, err := NewWithWatch(b, res, w, watch, clk)
	if err != nil {
		return nil, err
	}
	if acct == nil {
		return j, nil
	}
	jb, ok := j.(*job)
	if !ok {
		return nil, errors.New("optioneod: unexpected job type")
	}
	jb.acct = acct
	return jb, nil
}

type heldCall struct {
	expiry string // YYYYMMDD
	strike string // as the position gave it
}

// heldCalls lists the open short calls of symbol: per alias the positions of the newest day that has any,
// searching back from day.
func (j *job) heldCalls(ctx context.Context, symbol string, day time.Time) ([]heldCall, error) {
	var out []heldCall
	for _, alias := range heldAliases {
		for back := 0; back <= heldLookbackDays; back++ {
			d := day.AddDate(0, 0, -back).Format(time.DateOnly)
			rows, err := j.acct.Positions(ctx, alias, d)
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 {
				continue
			}
			for _, p := range rows {
				if p.SecType != "OPT" || p.Symbol != symbol || !strings.EqualFold(p.Right, "call") || p.Strike == "" || p.Expiry == "" {
					continue
				}
				if q, ok := new(big.Rat).SetString(p.Quantity); !ok || q.Sign() >= 0 {
					continue
				}
				out = append(out, heldCall{expiry: strings.ReplaceAll(p.Expiry, "-", ""), strike: p.Strike})
			}
			break
		}
	}
	return out, nil
}

// addHeld appends to pairs the call of every held entry that pairs does not hold yet; a strike compares by value.
func addHeld(pairs []broker.Contract, chain broker.Chain, u underlying, held []heldCall) []broker.Contract {
	seen := map[string]bool{}
	for _, p := range pairs {
		seen[pairKey(p.Expiry, p.Strike, p.Right)] = true
	}
	for _, h := range held {
		k := pairKey(h.expiry, h.strike, "C")
		if seen[k] {
			continue
		}
		seen[k] = true
		pairs = append(pairs, broker.Contract{SecType: "OPT", Symbol: u.Symbol, Exchange: chain.Exchange, Currency: u.Currency,
			Expiry: h.expiry, Strike: h.strike, Right: "C", Multiplier: chain.Multiplier, TradingClass: chain.TradingClass})
	}
	return pairs
}

func pairKey(expiry, strike, right string) string {
	if r, ok := new(big.Rat).SetString(strings.TrimSpace(strike)); ok {
		strike = r.RatString()
	}
	return expiry + "|" + strike + "|" + right
}
