// Algorithm and definition table after QuantConnect Lean, OptionStrategyMatcher, Apache-2.0, https://github.com/QuantConnect/Lean

package optionstruct

import (
	"cmp"
	"math/big"
	"slices"
	"strings"
)

// piece is one open ledger position; left is the lots of it not yet given to a unit, and given
// the commission of each fill already written into earlier slices.
type piece struct {
	l     leg
	left  *big.Rat
	given []*big.Rat
}

// contract is one matching position: the open pieces of one contract and side, drawn in order
// of OpenedOn then PositionID (Decision 2). left is the lots not yet matched.
type contract struct {
	right, expiry string
	strike, mult  *big.Rat
	short         bool
	pieces        []*piece
	left          *big.Rat
}

// id is the id of the first piece; it only breaks ties between candidates.
func (c *contract) id() string { return c.pieces[0].l.p.PositionID }

// book is the live contracts of one underlying and currency with the shares of its cycles
// not yet used by a collar.
type book struct {
	contracts []*contract
	shares    *big.Rat
	basis     *big.Rat
}

// newBook groups parsed legs into contracts; the pieces of a contract are ordered by
// OpenedOn then PositionID.
func newBook(legs []leg, shares, basis *big.Rat) *book {
	b := &book{shares: shares, basis: basis}
	sorted := slices.Clone(legs)
	slices.SortStableFunc(sorted, func(x, y leg) int {
		if c := cmp.Compare(x.p.OpenedOn, y.p.OpenedOn); c != 0 {
			return c
		}
		return cmp.Compare(x.p.PositionID, y.p.PositionID)
	})
	index := map[string]*contract{}
	for _, l := range sorted {
		side := "L"
		if l.short() {
			side = "S"
		}
		key := strings.Join([]string{l.p.Right, l.strike.RatString(), l.p.Expiry, l.mult.RatString(), side}, "|")
		c := index[key]
		if c == nil {
			c = &contract{right: l.p.Right, expiry: l.p.Expiry, strike: l.strike, mult: l.mult,
				short: l.short(), left: new(big.Rat)}
			index[key] = c
			b.contracts = append(b.contracts, c)
		}
		lots := new(big.Rat).Abs(l.qty)
		c.pieces = append(c.pieces, &piece{l: l, left: lots, given: make([]*big.Rat, len(l.p.Fills))})
		c.left = add(c.left, lots)
	}
	return b
}

// matchGroup applies each definition of the table in order until it matches nothing, then
// the next (Decision 4); inside a definition the best combination wins (Decision 5). It returns
// the matched units; the lots left stay in the book.
func matchGroup(b *book) []unit {
	var out []unit
	for _, d := range definitions {
		for {
			cands, n := b.best(d)
			if n == 0 {
				break
			}
			out = append(out, b.apply(d, cands, n))
		}
	}
	return out
}

// best finds, among all combinations of contracts that fit the definition, the one with the
// largest whole multiple, then the narrowest strike distance, then the lowest ids.
func (b *book) best(d definition) (best []*contract, mult int64) {
	var bestDist *big.Rat
	var walk func(chosen []*contract)
	walk = func(chosen []*contract) {
		i := len(chosen)
		if i == len(d.legs) {
			n := b.multiple(d, chosen)
			if n == 0 {
				return
			}
			dist := distance2(chosen)
			if n < mult || (n == mult && !better(dist, chosen, bestDist, best)) {
				return
			}
			best, mult, bestDist = slices.Clone(chosen), n, dist
			return
		}
		ld := d.legs[i]
		for _, c := range b.contracts {
			if c.left.Sign() <= 0 || c.right != ld.right || c.short != (ld.sign < 0) ||
				slices.Contains(chosen, c) || !ld.ok(chosen, c) {
				continue
			}
			walk(append(chosen, c))
		}
	}
	walk(nil)
	return best, mult
}

// better tells whether a combination beats the current best of the same multiple.
func better(dist *big.Rat, chosen []*contract, bestDist *big.Rat, best []*contract) bool {
	if bestDist == nil {
		return true
	}
	if c := dist.Cmp(bestDist); c != 0 {
		return c < 0
	}
	for i := range chosen {
		if c := cmp.Compare(chosen[i].id(), best[i].id()); c != 0 {
			return c < 0
		}
	}
	return false
}

// distance2 is the strike distance between the lowest and the highest strike of a combination.
func distance2(cs []*contract) *big.Rat {
	lo, hi := cs[0].strike, cs[0].strike
	for _, c := range cs {
		if c.strike.Cmp(lo) < 0 {
			lo = c.strike
		}
		if c.strike.Cmp(hi) > 0 {
			hi = c.strike
		}
	}
	return sub(hi, lo)
}

// multiple is the smallest, over the legs (and the shares of a collar), of the whole number of
// times the leg's lots fit in the lots the contract still has.
func (b *book) multiple(d definition, chosen []*contract) int64 {
	var n int64 = -1
	fit := func(have, need *big.Rat) {
		q := new(big.Int).Quo(new(big.Int).Mul(have.Num(), need.Denom()), new(big.Int).Mul(have.Denom(), need.Num()))
		if m := q.Int64(); n < 0 || m < n {
			n = m
		}
	}
	for i, c := range chosen {
		fit(c.left, new(big.Rat).SetInt64(int64(d.legs[i].lots)))
	}
	if d.covered {
		fit(b.shares, mul(chosen[0].mult, new(big.Rat).SetInt64(int64(d.legs[0].lots))))
	}
	return max(n, 0)
}

// apply takes n times the lots of each leg from its contract and makes one unit of the slices.
func (b *book) apply(d definition, chosen []*contract, n int64) unit {
	u := unit{kind: d.kind}
	for i, c := range chosen {
		u.legs = append(u.legs, c.take(new(big.Rat).SetInt64(n*int64(d.legs[i].lots)))...)
	}
	if d.covered {
		b.shares = sub(b.shares, mul(mul(chosen[0].mult, new(big.Rat).SetInt64(n)), new(big.Rat).SetInt64(int64(d.legs[0].lots))))
		u.basis = b.basis
	}
	return u
}

// take draws lots from the pieces of the contract in order and returns the slices.
func (c *contract) take(lots *big.Rat) []leg {
	var out []leg
	for _, p := range c.pieces {
		if lots.Sign() <= 0 {
			break
		}
		if p.left.Sign() <= 0 {
			continue
		}
		t := p.left
		if lots.Cmp(t) < 0 {
			t = lots
		}
		out = append(out, p.slice(t))
		lots = sub(lots, t)
		c.left = sub(c.left, t)
	}
	return out
}

// rest gives every piece's remaining lots as one slice per piece.
func (b *book) rest() []leg {
	var out []leg
	for _, c := range b.contracts {
		out = append(out, c.take(c.left)...)
	}
	return out
}

// slice is the part of a piece of t lots (Decision 7). A slice that is the whole piece keeps its
// text; any other is a copy with Quantity set to the slice and the commission of each fill scaled by
// slice over piece quantity. The slice that ends the piece takes what the others left of each
// commission, so the commissions of the slices add up to the commission of the piece.
func (p *piece) slice(t *big.Rat) leg {
	whole := new(big.Rat).Abs(p.l.qty)
	if t.Cmp(whole) == 0 {
		p.left = new(big.Rat)
		return p.l
	}
	last := t.Cmp(p.left) == 0
	p.left = sub(p.left, t)
	l := p.l
	q := new(big.Rat).Set(t)
	if l.short() {
		q.Neg(q)
	}
	l.qty = q
	l.p.Quantity = exactText(q)
	l.p.Fills = slices.Clone(p.l.p.Fills)
	for i, f := range l.p.Fills {
		c, ok := parseDec(f.Commission)
		if !ok {
			continue
		}
		var part *big.Rat
		if last {
			part = c
			if p.given[i] != nil {
				part = sub(c, p.given[i])
			}
		} else {
			scaled := mul(c, new(big.Rat).Quo(t, whole))
			part, _ = parseDec(fixed4(scaled))
		}
		if p.given[i] == nil {
			p.given[i] = new(big.Rat)
		}
		p.given[i] = add(p.given[i], part)
		l.p.Fills[i].Commission = exactText(part)
	}
	return l
}

// exactText writes a rational that has a finite decimal expansion with the fewest places, at
// least none.
func exactText(r *big.Rat) string {
	for places := range 20 {
		s := r.FloatString(places)
		if back, ok := parseDec(s); ok && back.Cmp(r) == 0 {
			return s
		}
	}
	return fixed4(r)
}
