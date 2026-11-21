package pagefig

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// familyRiskCap is the most rows of Puts and of Companies.
const familyRiskCap = 60

var _ blockapi.FamilyRiskFigure = familyRisk{}

type familyRisk struct{}

// NewFamilyRisk returns the figure of the family's assignment cash, cash after assignment and company exposure.
func NewFamilyRisk() blockapi.FamilyRiskFigure { return familyRisk{} }

// usdNeed is one obligation in USD, or the gap that stops it.
func usdNeed(o blockapi.ObligationIn, usdJpy string) (*big.Rat, string) {
	n, ok := notional(o)
	if !ok {
		return nil, "missing_input"
	}
	switch o.Currency {
	case "USD":
		return n, ""
	case "JPY":
		rate, ok := parse(usdJpy)
		if !ok || rate.Sign() <= 0 {
			return nil, "unsupported_currency"
		}
		return n.Quo(n, rate), ""
	}
	return nil, "unsupported_currency"
}

// usdPremium is the premium of one obligation in USD, or false when unknown.
func usdPremium(o blockapi.ObligationIn, usdJpy string) (*big.Rat, bool) {
	p, ok := parse(o.PremiumIncome)
	if !ok {
		return nil, false
	}
	switch o.Currency {
	case "USD":
		return p, true
	case "JPY":
		rate, ok := parse(usdJpy)
		if !ok || rate.Sign() <= 0 {
			return nil, false
		}
		return p.Quo(p, rate), true
	}
	return nil, false
}

// ratOf reads a figure as a number; gap is its gap, or missing_input when the text is unreadable.
func ratOf(v compute.FigureValue) (r *big.Rat, g string) {
	if v.Gap != "" {
		return nil, v.Gap
	}
	r, ok := parse(v.Value)
	if !ok {
		return nil, "missing_input"
	}
	return r, ""
}

// pctOf is v over net in percent; a gap on either side, or a net that is not positive, is a gap.
func pctOf(v, net compute.FigureValue) compute.FigureValue {
	a, g := ratOf(v)
	if g != "" {
		return gap(g)
	}
	n, g := ratOf(net)
	if g != "" {
		return gap(g)
	}
	if n.Sign() <= 0 {
		return gap("no_data")
	}
	return share(a, n, true)
}

// frSum is an exact sum that turns into the gap of its first missing part.
type frSum struct {
	sum *big.Rat
	gap string
}

func newSum() *frSum { return &frSum{sum: new(big.Rat)} }

func (s *frSum) add(r *big.Rat, g string) {
	if s.gap != "" {
		return
	}
	if g != "" {
		s.gap = g
		return
	}
	s.sum.Add(s.sum, r)
}

func (s *frSum) fig() compute.FigureValue {
	if s.gap != "" {
		return gap(s.gap)
	}
	return compute.FigureValue{Value: money(s.sum)}
}

// val is the number of the sum, nil when it is a gap.
func (s *frSum) val() *big.Rat {
	if s.gap != "" {
		return nil
	}
	return s.sum
}

type frPutAcc struct {
	row     blockapi.FamilyRiskPut
	cash    *frSum
	premium *big.Rat // nil when unknown
	seen    map[string]bool
}

func (familyRisk) FamilyRisk(in blockapi.FamilyRiskIn) blockapi.FamilyRiskResult {
	var res blockapi.FamilyRiskResult
	puts, putOrder, unread := familyAccounts(in, &res)
	res.Puts = familyPutRows(puts, putOrder)
	res.Companies = familyCompanyRows(in, puts, putOrder, unread)
	if n := len(res.Puts); n > familyRiskCap {
		res.Hidden += n - familyRiskCap
		res.Puts = res.Puts[:familyRiskCap]
	}
	if n := len(res.Companies); n > familyRiskCap {
		res.Hidden += n - familyRiskCap
		res.Companies = res.Companies[:familyRiskCap]
	}
	return res
}

// familyAccounts fills the account rows, the totals and the shares of res and returns the puts per underlying,
// their order of first appearance and whether an account was unread.
func familyAccounts(in blockapi.FamilyRiskIn, res *blockapi.FamilyRiskResult) (map[string]*frPutAcc, []string, bool) {
	needT, cashT, afterT := newSum(), newSum(), newSum()
	unread := false
	puts := map[string]*frPutAcc{}
	var putOrder []string
	for _, a := range in.Accounts {
		out := blockapi.FamilyRiskAccountOut{Alias: a.Alias, Cash: a.Cash}
		cashR, cashG := ratOf(a.Cash)
		cashT.add(cashR, cashG)
		if !a.Read {
			unread = true
			out.Need, out.CashAfter = gap("missing_input"), gap("missing_input")
			res.Accounts = append(res.Accounts, out)
			continue
		}
		need := newSum()
		for _, o := range a.Obligations {
			amt, g := usdNeed(o, in.UsdJpy)
			need.add(amt, g)
			p := puts[o.Underlying]
			if p == nil {
				p = &frPutAcc{seen: map[string]bool{}, premium: new(big.Rat), cash: newSum()}
				p.row.Underlying = o.Underlying
				puts[o.Underlying] = p
				putOrder = append(putOrder, o.Underlying)
			}
			if !p.seen[a.Alias] {
				p.seen[a.Alias] = true
				p.row.Aliases = append(p.row.Aliases, a.Alias)
			}
			p.cash.add(amt, g)
			if pr, ok := usdPremium(o, in.UsdJpy); ok && p.premium != nil {
				p.premium.Add(p.premium, pr)
			} else {
				p.premium = nil
			}
		}
		out.Need = need.fig()
		out.CashAfter = gap(need.gap)
		if need.gap == "" && cashG != "" {
			out.CashAfter = gap(cashG)
		}
		if need.gap == "" && cashG == "" {
			left := new(big.Rat).Sub(cashR, need.sum)
			out.CashAfter = compute.FigureValue{Value: money(left)}
			out.Short = left.Sign() < 0
		}
		needT.add(need.val(), need.gap)
		if out.CashAfter.Gap != "" {
			afterT.add(nil, out.CashAfter.Gap)
		} else {
			afterT.add(new(big.Rat).Sub(cashR, need.sum), "")
		}
		if out.Short {
			res.ShortCount++
		}
		res.Accounts = append(res.Accounts, out)
	}
	res.NeedTotal, res.CashTotal, res.AfterTotal = needT.fig(), cashT.fig(), afterT.fig()
	if unread {
		res.NeedTotal, res.CashTotal, res.AfterTotal = gap("missing_input"), gap("missing_input"), gap("missing_input")
	}
	res.NeedShare, res.AfterShare = pctOf(res.NeedTotal, in.Net), pctOf(res.AfterTotal, in.Net)
	return puts, putOrder, unread
}

// familyPutRows is the sorted Puts list, not yet capped.
func familyPutRows(puts map[string]*frPutAcc, putOrder []string) []blockapi.FamilyRiskPut {
	type rankedPut struct {
		row blockapi.FamilyRiskPut
		val *big.Rat
	}
	var rp []rankedPut
	for _, u := range putOrder {
		p := puts[u]
		p.row.Cash = p.cash.fig()
		if p.premium != nil {
			p.row.Premium = money(p.premium)
		}
		rp = append(rp, rankedPut{p.row, p.cash.val()})
	}
	sort.SliceStable(rp, func(i, j int) bool {
		return rankLess(rp[i].val, rp[j].val, rp[i].row.Underlying, rp[j].row.Underlying)
	})
	var out []blockapi.FamilyRiskPut
	for _, x := range rp {
		out = append(out, x.row)
	}
	return out
}

// familyCompanyRows is the sorted Companies list, one per underlying of the stock lines and the puts, not yet
// capped.
func familyCompanyRows(in blockapi.FamilyRiskIn, puts map[string]*frPutAcc, putOrder []string, unread bool) []blockapi.FamilyRiskCompany {
	stocks := map[string]*frSum{}
	var cOrder []string
	touch := func(u string) {
		if _, ok := stocks[u]; !ok {
			stocks[u] = newSum()
			cOrder = append(cOrder, u)
		}
	}
	for _, l := range in.Stocks {
		touch(l.Underlying)
		v, g := ratOf(l.Value)
		if g != "" {
			g = "missing_quote"
		}
		stocks[l.Underlying].add(v, g)
	}
	for _, u := range putOrder {
		touch(u)
	}
	type rankedCo struct {
		row blockapi.FamilyRiskCompany
		val *big.Rat
	}
	var rc []rankedCo
	for _, u := range cOrder {
		stock := stocks[u]
		c := blockapi.FamilyRiskCompany{Underlying: u, Stock: stock.fig(), Put: compute.FigureValue{Value: money(new(big.Rat))}}
		exp := newSum()
		exp.add(stock.val(), stock.gap)
		if p, ok := puts[u]; ok {
			c.Put = p.cash.fig()
			exp.add(p.cash.val(), p.cash.gap)
		} else if unread {
			c.Put = gap("missing_input")
			exp.add(nil, "missing_input")
		}
		c.Exposure = exp.fig()
		c.Share = pctOf(c.Exposure, in.Net)
		rc = append(rc, rankedCo{c, exp.val()})
	}
	sort.SliceStable(rc, func(i, j int) bool {
		return rankLess(rc[i].val, rc[j].val, rc[i].row.Underlying, rc[j].row.Underlying)
	})
	var out []blockapi.FamilyRiskCompany
	for _, x := range rc {
		out = append(out, x.row)
	}
	return out
}

// rankLess orders by amount descending, a nil amount (a gap row) last, ties by underlying.
func rankLess(a, b *big.Rat, ua, ub string) bool {
	if (a == nil) != (b == nil) {
		return a != nil
	}
	if a != nil {
		if c := a.Cmp(b); c != 0 {
			return c > 0
		}
	}
	return ua < ub
}
