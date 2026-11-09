package optionresult

import (
	"math/big"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

// The sides of a section and the endings of a closed position.
const (
	sideSold   = "sold"
	sideBought = "bought"
)

var endingOrder = []string{"closed", "expired", "assigned", "exercised"}

type results struct{}

// New returns the rule: exact (math/big.Rat), pure, no clock and no I/O.
func New() compute.OptionResults { return results{} }

// row is one closed position with its figures parsed.
type row struct {
	pos      compute.LedgerPosition
	realized *big.Rat
	premium  *big.Rat // opening premium, positive
	sold     bool
}

type sectionKey struct{ side, currency string }

// Summarise cuts the closed positions into one section per side and currency. A position
// is closed when its Status is closed, expired, assigned or exercised and it has a
// RealizedPnL and a ClosedOn; every other row (open ones, rows without a figure) is
// ignored. Unresolved and Review ids are copied into the reliability of every section and
// never enter a figure. The sections are ordered sold before bought, then by currency.
func (results) Summarise(in compute.ResultInput) compute.ResultSummary {
	groups := map[sectionKey][]row{}
	for _, p := range in.Positions {
		r, ok := parseRow(p)
		if !ok {
			continue
		}
		side := sideBought
		if r.sold {
			side = sideSold
		}
		k := sectionKey{side, p.Currency}
		groups[k] = append(groups[k], r)
	}
	keys := make([]sectionKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].side != keys[j].side {
			return keys[i].side == sideSold
		}
		return keys[i].currency < keys[j].currency
	})
	var out compute.ResultSummary
	for _, k := range keys {
		out.Sections = append(out.Sections, section(k, groups[k], in))
	}
	return out
}

func section(k sectionKey, rows []row, in compute.ResultInput) compute.ResultSection {
	realized, premium := new(big.Rat), new(big.Rat)
	assignedPremium, assignedShares := new(big.Rat), new(big.Rat)
	for _, r := range rows {
		realized.Add(realized, r.realized)
		premium.Add(premium, r.premium)
		if r.pos.Status == "assigned" {
			if v, ok := parse(r.pos.PremiumIncome); ok {
				assignedPremium.Add(assignedPremium, v)
			}
			assignedShares.Add(assignedShares, shares(r.pos))
		}
	}
	sec := compute.ResultSection{
		Side: k.side, Currency: k.currency,
		Months:          months(rows, in.AsOf),
		Endings:         endings(rows),
		Realized:        fixed(realized, 4),
		Premium:         fixed(premium, 4),
		AssignedPremium: fixed(assignedPremium, 4),
		AssignedShares:  fixed(assignedShares, 4),
		Reliability:     reliability(rows, in.Unresolved, in.Review),
	}
	if k.side == sideSold && premium.Sign() != 0 {
		sec.KeptShare = fixed(new(big.Rat).Quo(realized, premium), 6)
	}
	return sec
}

func endings(rows []row) []compute.ResultEnding {
	count := map[string]int{}
	sum := map[string]*big.Rat{}
	for _, r := range rows {
		s := r.pos.Status
		count[s]++
		if sum[s] == nil {
			sum[s] = new(big.Rat)
		}
		sum[s].Add(sum[s], r.realized)
	}
	out := make([]compute.ResultEnding, 0, len(endingOrder))
	for _, s := range endingOrder {
		v := sum[s]
		if v == nil {
			v = new(big.Rat)
		}
		out = append(out, compute.ResultEnding{Status: s, Count: count[s], Realized: fixed(v, 4)})
	}
	return out
}

func parseRow(p compute.LedgerPosition) (row, bool) {
	switch p.Status {
	case "closed", "expired", "assigned", "exercised":
	default:
		return row{}, false
	}
	realized, ok := parse(p.RealizedPnL)
	if !ok || len(p.ClosedOn) < 7 {
		return row{}, false
	}
	qty, _ := parse(p.Quantity)
	return row{pos: p, realized: realized, premium: openingPremium(p), sold: qty != nil && qty.Sign() < 0}, true
}

// openingPremium is the sum of quantity times price times multiplier over the open fills.
func openingPremium(p compute.LedgerPosition) *big.Rat {
	total := new(big.Rat)
	mult, ok := parse(p.Multiplier)
	if !ok {
		return total
	}
	for _, f := range p.Fills {
		if f.Action != "open" {
			continue
		}
		q, okq := parse(f.Quantity)
		px, okp := parse(f.Price)
		if !okq || !okp {
			continue
		}
		v := new(big.Rat).Mul(q, px)
		total.Add(total, v.Mul(v, mult).Abs(v))
	}
	return total
}

// shares is the absolute quantity times the multiplier.
func shares(p compute.LedgerPosition) *big.Rat {
	q, ok1 := parse(p.Quantity)
	m, ok2 := parse(p.Multiplier)
	if !ok1 || !ok2 {
		return new(big.Rat)
	}
	v := new(big.Rat).Mul(q, m)
	return v.Abs(v)
}

func parse(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

// fixed prints x rounded half-even to places decimals.
func fixed(x *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	n := new(big.Int).Mul(x.Num(), scale)
	neg := n.Sign() < 0
	n.Abs(n)
	q, r := new(big.Int).QuoRem(n, x.Denom(), new(big.Int))
	switch r.Lsh(r, 1).Cmp(x.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	s := q.String()
	for len(s) <= places {
		s = "0" + s
	}
	s = s[:len(s)-places] + "." + s[len(s)-places:]
	if neg && q.Sign() != 0 {
		s = "-" + s
	}
	return s
}
