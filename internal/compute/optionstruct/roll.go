package optionstruct

import (
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
)

// fillCompare sets the compare row of a structure with one short leg and no long leg
// (Decision 13): close cost, roll credit, roll net and cash after.
func (m marker) fillCompare(s *compute.Structure, u unit, mk legMark) {
	l := u.legs[0]
	f := &s.Figures
	f.CloseCost = closeCost(l, mk)
	f.RollCredit, f.RollNet = m.roll(l, mk, f.CloseCost)
	f.CashAfter = cashAfter(l, f.CloseCost)
}

// closeCost, figure: structure.close_cost, is the mid of the short leg times multiplier
// times contracts.
func closeCost(l leg, mk legMark) compute.FigureValue {
	if mk.gap != "" {
		return compute.FigureValue{Gap: mk.gap}
	}
	return val(mul(mk.mid, scale(l)))
}

// roll, figure: structure.roll_credit, is the bid of the same right and strike at the next
// monthly expiry times multiplier times contracts; the net is the credit minus the close cost.
// No such quote, or no bid, is no_data.
func (m marker) roll(l leg, mk legMark, closeFig compute.FigureValue) (credit, net compute.FigureValue) {
	credit, net = noData, noData
	if len(m.quotes) == 0 {
		return credit, net
	}
	q := m.find(l.p.Underlying, l.p.Right, l.strike, nextMonthly(l.p.Expiry))
	if q == nil {
		return credit, net
	}
	bid, ok := parseDec(q.Bid)
	switch {
	case !ok || bid.Sign() <= 0:
		return credit, net
	case m.stale(q.Day):
		return compute.FigureValue{Gap: gapStale}, compute.FigureValue{Gap: gapStale}
	}
	c := mul(bid, scale(l))
	credit = val(c)
	if closeFig.Gap != "" {
		return credit, compute.FigureValue{Gap: closeFig.Gap}
	}
	return credit, val(sub(c, mul(mk.mid, scale(l))))
}

// cashAfter is the premium income minus the commissions minus the close cost of the single
// short leg; a gap when the close cost is one.
func cashAfter(l leg, closeFig compute.FigureValue) compute.FigureValue {
	if closeFig.Gap != "" {
		return compute.FigureValue{Gap: closeFig.Gap}
	}
	income, ok1 := parseDec(l.p.PremiumIncome)
	comm, ok2 := parseDec(l.p.Commissions)
	cost, _ := parseDec(closeFig.Value)
	if !ok1 || !ok2 {
		return missing
	}
	return val(sub(sub(income, comm), cost))
}

// nextMonthly is the first third Friday after the expiry, by date arithmetic; an expiry that
// does not parse gives "".
func nextMonthly(expiry string) string {
	t, err := time.Parse(dayLayout, expiry)
	if err != nil {
		return ""
	}
	y, mo := t.Year(), t.Month()
	for {
		first := time.Date(y, mo, 1, 0, 0, 0, 0, time.UTC)
		third := first.AddDate(0, 0, (int(time.Friday)-int(first.Weekday())+7)%7+14)
		if third.After(t) {
			return third.Format(dayLayout)
		}
		mo++
		if mo > time.December {
			y, mo = y+1, time.January
		}
	}
}
