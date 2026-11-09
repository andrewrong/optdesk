package optionledger

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// dec reads a decimal text exactly.
func dec(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "/") {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

// fixed4 prints x rounded half-even to 4 places.
func fixed4(x *big.Rat) string {
	scaled := new(big.Rat).Mul(x, big.NewRat(10000, 1))
	num := new(big.Int).Abs(scaled.Num())
	q, r := new(big.Int).QuoRem(num, scaled.Denom(), new(big.Int))
	switch new(big.Int).Lsh(r, 1).Cmp(scaled.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	whole, frac := new(big.Int).QuoRem(q, big.NewInt(10000), new(big.Int))
	sign := ""
	if scaled.Sign() < 0 && q.Sign() != 0 {
		sign = "-"
	}
	f := strconv.FormatInt(frac.Int64(), 10)
	return sign + whole.String() + "." + strings.Repeat("0", 4-len(f)) + f
}

// trimmed prints a quantity: at most 4 decimals, no trailing zero.
func trimmed(x *big.Rat) string {
	s := fixed4(x)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// markSet holds the newest mark of each contract that is not after the replay day, and the
// rule of its age (Decision 9): a mark older than the limit in session days is no mark.
type markSet struct {
	byContract map[string]compute.LedgerMark
	underlying map[string]string // mark contract name -> underlying of the option
	sessions   []contracts.Day
	asOf       contracts.Day
	maxAge     int
}

func newMarkSet(in compute.LedgerInput, p compute.LedgerParams, opts []event) markSet {
	m := markSet{
		byContract: map[string]compute.LedgerMark{},
		underlying: map[string]string{},
		sessions:   in.SessionDays,
		asOf:       in.AsOf,
		maxAge:     p.MarkMaxAgeSessions,
	}
	for _, mk := range in.Marks {
		if in.AsOf != "" && mk.Day > in.AsOf {
			continue
		}
		if cur, ok := m.byContract[mk.Contract]; !ok || mk.Day > cur.Day {
			m.byContract[mk.Contract] = mk
		}
	}
	for _, e := range opts {
		for _, k := range e.markKeys {
			m.underlying[k] = e.underlying
		}
	}
	return m
}

func (m markSet) fresh(mk compute.LedgerMark) bool {
	age := 0
	for _, d := range m.sessions {
		if d > mk.Day && (m.asOf == "" || d <= m.asOf) {
			age++
		}
	}
	return age <= m.maxAge
}

// option returns the mid of bid and ask of the newest mark of a contract.
func (m markSet) option(keys []string) (*big.Rat, contracts.Day, bool) {
	var best compute.LedgerMark
	found := false
	for _, k := range keys {
		if mk, ok := m.byContract[k]; ok && (!found || mk.Day > best.Day) {
			best, found = mk, true
		}
	}
	if !found || !m.fresh(best) {
		return nil, "", false
	}
	bid, ok1 := dec(best.Bid)
	ask, ok2 := dec(best.Ask)
	if !ok1 || !ok2 || bid.Sign() < 0 || ask.Sign() <= 0 {
		return nil, "", false
	}
	mid := new(big.Rat).Add(bid, ask)
	return mid.Quo(mid, big.NewRat(2, 1)), best.Day, true
}

// stockMark returns the underlying price of the newest mark of a name.
func (m markSet) stockMark(name string) (*big.Rat, bool) {
	var best compute.LedgerMark
	found := false
	for k, u := range m.underlying {
		if mk, ok := m.byContract[k]; ok && u == name && (!found || mk.Day > best.Day) {
			best, found = mk, true
		}
	}
	if !found || !m.fresh(best) {
		return nil, false
	}
	px, ok := dec(best.UnderlyingClose)
	if !ok || px.Sign() <= 0 {
		return nil, false
	}
	return px, true
}

// position builds the printed figures of a piece (Decision 9).
func (p piece) position(alias string, ms markSet) compute.LedgerPosition {
	broker := ""
	if p.broker != nil {
		broker = fixed4(p.broker)
	}
	out := compute.LedgerPosition{
		PositionID:     alias + ":" + p.openTx + ":" + strconv.Itoa(p.n),
		Alias:          alias,
		Underlying:     p.underlying,
		Contract:       p.contract,
		Status:         p.status,
		Early:          p.early,
		Quantity:       trimmed(p.qty),
		Multiplier:     trimmed(p.mult),
		Currency:       p.currency,
		PremiumIncome:  fixed4(p.premium),
		Commissions:    fixed4(p.commission),
		Right:          p.putCall,
		Expiry:         string(dayOf(p.expiry)),
		Strike:         trimmed(p.strike),
		OpenedOn:       dayOf(p.openDate),
		ClosedOn:       dayOf(p.closeDate),
		OpenPrice:      p.openPrice,
		Fills:          p.fills,
		BrokerRealized: broker,
	}

	net := new(big.Rat).Sub(p.premium, p.commission)
	if p.status != "open" {
		out.RealizedPnL = fixed4(net)
		return out
	}
	mid, day, ok := ms.option(p.markKeys)
	if !ok {
		out.MarkStatus = "no_mark"
		return out
	}
	value := new(big.Rat).Mul(mid, p.qty)
	value.Mul(value, p.mult)
	out.Mark = fixed4(mid)
	out.MarkDay = day
	out.MarkStatus = "ok"
	out.UnrealizedPnL = fixed4(net.Add(net, value))
	return out
}
