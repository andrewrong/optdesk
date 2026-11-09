package optionledger

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
)

type stockLot struct{ rem, price *big.Rat }

// cycle is one wheel cycle of an alias, underlying and currency (Decision 10).
type cycle struct {
	id, underlying, currency string
	startKey, endKey         string // endKey is empty while shares are held
	lots                     []*stockLot
	commissions, realized    *big.Rat
	boughtQty, boughtCost    *big.Rat
	deliveries               []event // stock buys of the cycle
	premium                  *big.Rat
}

// buildCycles walks the stock rows (in time order) first in, first out. A sale with no
// shares before it is returned as unresolved. Option pieces are then attributed: the
// position whose assignment gave shares, and every position opened while shares were held,
// belong to the cycle; the rest is premium outside cycles of the name.
func buildCycles(alias string, stocks []event, pieces []piece, ms markSet) ([]compute.LedgerCycle, []string) {
	var cycles []*cycle
	active := map[string]*cycle{}
	var unresolved []string
	for _, e := range stocks {
		key := e.underlying + "|" + e.currency
		c := active[key]
		if e.kind == kindStockBuy {
			if c == nil {
				c = newCycle(alias, e)
				cycles = append(cycles, c)
				active[key] = c
			}
			c.lots = append(c.lots, &stockLot{rem: new(big.Rat).Set(e.qty), price: e.price})
			c.boughtQty.Add(c.boughtQty, e.qty)
			c.boughtCost.Add(c.boughtCost, new(big.Rat).Mul(e.qty, e.price))
			c.commissions.Add(c.commissions, e.commission)
			c.deliveries = append(c.deliveries, e)
			continue
		}
		if c == nil {
			unresolved = append(unresolved, e.txID)
			continue
		}
		if !c.sell(e) {
			unresolved = append(unresolved, e.txID)
		}
		if len(c.lots) == 0 {
			c.endKey = e.timeKey
			delete(active, key)
		}
	}

	outside := map[string]*big.Rat{}
	for _, p := range pieces {
		key := p.underlying + "|" + p.currency
		if c := owner(cycles, p); c != nil {
			c.premium.Add(c.premium, p.premium)
			continue
		}
		if outside[key] == nil {
			outside[key] = new(big.Rat)
		}
		outside[key].Add(outside[key], p.premium)
	}

	out := make([]compute.LedgerCycle, 0, len(cycles))
	for _, c := range cycles {
		out = append(out, c.print(alias, outside[c.underlying+"|"+c.currency], ms))
	}
	return out, unresolved
}

func newCycle(alias string, first event) *cycle {
	return &cycle{
		id: alias + ":" + first.underlying + ":" + first.txID, underlying: first.underlying,
		currency: first.currency, startKey: first.timeKey,
		commissions: new(big.Rat), realized: new(big.Rat),
		boughtQty: new(big.Rat), boughtCost: new(big.Rat), premium: new(big.Rat),
	}
}

// owner returns the first cycle a piece belongs to, or nil.
func owner(cycles []*cycle, p piece) *cycle {
	for _, c := range cycles {
		if c.underlying != p.underlying || c.currency != p.currency {
			continue
		}
		if p.status == "assigned" && p.putCall == "P" && c.delivered(p) {
			return c
		}
		if p.openKey >= c.startKey && (c.endKey == "" || p.openKey <= c.endKey) {
			return c
		}
	}
	return nil
}

// delivered reports whether a stock buy of the cycle is the delivery of an assigned put:
// the same day, at the strike.
func (c *cycle) delivered(p piece) bool {
	for _, d := range c.deliveries {
		if d.tradeDate == p.closeDate && d.price.Cmp(p.strike) == 0 {
			return true
		}
	}
	return false
}

func (c *cycle) print(alias string, outside *big.Rat, ms markSet) compute.LedgerCycle {
	shares := new(big.Rat)
	cost := new(big.Rat)
	for _, l := range c.lots {
		shares.Add(shares, l.rem)
		cost.Add(cost, new(big.Rat).Mul(l.rem, l.price))
	}
	basis := new(big.Rat).Quo(c.boughtCost, c.boughtQty)
	if shares.Sign() > 0 {
		basis = new(big.Rat).Quo(cost, shares)
	}
	if outside == nil {
		outside = new(big.Rat)
	}
	out := compute.LedgerCycle{
		CycleID:              c.id,
		Alias:                alias,
		Underlying:           c.underlying,
		Currency:             c.currency,
		Shares:               trimmed(shares),
		CostBasisPerShare:    fixed4(basis),
		StockRealizedPnL:     fixed4(c.realized),
		PremiumIncome:        fixed4(c.premium),
		StockCommissions:     fixed4(c.commissions),
		PremiumOutsideCycles: fixed4(outside),
	}
	if px, ok := ms.stockMark(c.underlying); ok && shares.Sign() > 0 {
		out.UnrealizedStockPnL = fixed4(new(big.Rat).Sub(new(big.Rat).Mul(px, shares), cost))
	}
	return out
}

// sell takes the shares of a sale first in, first out; false when the sale is larger than
// the shares held.
func (c *cycle) sell(e event) bool {
	c.commissions.Add(c.commissions, e.commission)
	left := new(big.Rat).Abs(e.qty)
	total := new(big.Rat).Set(left)
	for left.Sign() > 0 && len(c.lots) > 0 {
		l := c.lots[0]
		take := minAbs(l.rem, left)
		c.realized.Add(c.realized, share(e.proceeds, take, total))
		c.realized.Sub(c.realized, new(big.Rat).Mul(take, l.price))
		c.realized.Sub(c.realized, share(e.commission, take, total))
		l.rem.Sub(l.rem, take)
		left.Sub(left, take)
		if l.rem.Sign() == 0 {
			c.lots = c.lots[1:]
		}
	}
	return left.Sign() == 0
}
