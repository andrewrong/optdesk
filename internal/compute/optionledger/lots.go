package optionledger

import (
	"math/big"
	"sort"
)

// piece is an open lot or the part a close left (Decision 8): a lot closed in two steps
// is two pieces. Quantity is signed: negative is short.
type piece struct {
	openTx, closeTx string
	n               int
	openKey         string
	closeDate       string
	underlying      string
	contract        string
	markKeys        []string
	currency        string
	putCall         string
	expiry          string
	strike          *big.Rat
	qty             *big.Rat
	mult            *big.Rat
	premium         *big.Rat // cash of the opens plus the cash of the closes
	commission      *big.Rat
	status          string
	early           bool
}

type lot struct {
	ev        event
	rem, orig *big.Rat
	n         int
}

// matchOptions matches closing events to opens first in, first out, per contract. A close
// part with no open before it is returned as unresolved and enters no piece. The events
// must be in time order.
func matchOptions(evs []event) ([]piece, []string) {
	lots := map[string][]*lot{}
	var out []piece
	var unresolved []string
	for _, e := range evs {
		if e.kind == kindOpen {
			lots[e.contract] = append(lots[e.contract], &lot{ev: e, rem: new(big.Rat).Set(e.qty), orig: new(big.Rat).Set(e.qty)})
			continue
		}
		left := new(big.Rat).Set(e.qty)
		q := lots[e.contract]
		for left.Sign() != 0 && len(q) > 0 {
			l := q[0]
			if l.rem.Sign() == left.Sign() {
				break // same direction: nothing to close
			}
			take := minAbs(l.rem, left)
			signed := new(big.Rat).Set(take)
			if l.rem.Sign() < 0 {
				signed.Neg(signed)
			}
			l.n++
			p := newPiece(l.ev, l.n, signed)
			p.premium = add(share(l.ev.proceeds, take, l.orig), share(e.proceeds, take, e.qty))
			p.commission = add(share(l.ev.commission, take, l.orig), share(e.commission, take, e.qty))
			p.closeTx = e.txID
			p.closeDate = e.tradeDate
			p.status = statusOf(e.kind)
			p.early = e.kind == kindAssign && e.expiry != "" && e.tradeDate < e.expiry
			out = append(out, p)
			l.rem.Sub(l.rem, signed)
			left.Add(left, signed)
			if l.rem.Sign() == 0 {
				q = q[1:]
			}
		}
		lots[e.contract] = q
		if left.Sign() != 0 {
			unresolved = append(unresolved, e.txID)
		}
	}
	for _, q := range lots {
		for _, l := range q {
			l.n++
			p := newPiece(l.ev, l.n, l.rem)
			p.premium = share(l.ev.proceeds, l.rem, l.orig)
			p.commission = share(l.ev.commission, l.rem, l.orig)
			p.status = "open"
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.openKey != b.openKey {
			return a.openKey < b.openKey
		}
		if a.openTx != b.openTx {
			return txLess(a.openTx, b.openTx)
		}
		return a.n < b.n
	})
	return out, unresolved
}

func newPiece(open event, n int, qty *big.Rat) piece {
	return piece{
		openTx: open.txID, n: n, openKey: open.timeKey,
		underlying: open.underlying, contract: open.contract, markKeys: open.markKeys,
		currency: open.currency, putCall: open.putCall, expiry: open.expiry,
		strike: open.strike, qty: new(big.Rat).Set(qty), mult: open.mult,
	}
}

func statusOf(kind string) string {
	switch kind {
	case kindExpire:
		return "expired"
	case kindAssign:
		return "assigned"
	case kindExercise:
		return "exercised"
	}
	return "closed"
}

// share is whole x |part| / |total|.
func share(whole, part, total *big.Rat) *big.Rat {
	r := new(big.Rat).Mul(whole, new(big.Rat).Abs(part))
	return r.Quo(r, new(big.Rat).Abs(total))
}

func minAbs(a, b *big.Rat) *big.Rat {
	x, y := new(big.Rat).Abs(a), new(big.Rat).Abs(b)
	if x.Cmp(y) <= 0 {
		return x
	}
	return y
}

func add(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
