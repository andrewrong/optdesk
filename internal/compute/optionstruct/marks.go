package optionstruct

import (
	"math/big"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// DefaultCaptureTarget is the share of the premium whose capture asks for a decision
// (Decision 13); a caller passes it through StructureParams.CaptureTarget.
const DefaultCaptureTarget = "0.50"

// DefaultExitDTE is the days to expiry at which a sold leg asks for a decision (Decision 13);
// a caller passes it through StructureParams.ExitDTE.
const DefaultExitDTE = 21

// The gaps the marked figures set besides those of optionstruct.go.
const (
	gapMissingQuote = "missing_quote"
	gapStale        = "stale_input"
)

const dayLayout = "2006-01-02"

// marker holds what the marked figures read: the stored quotes, the parameters, the day of
// the evaluation and the session days. It has no clock; the day is the input's.
type marker struct {
	quotes []compute.StructureQuote
	params compute.StructureParams
	asOf   contracts.Day
	days   []contracts.Day
}

// legMark is the mark of one leg at mid and the profit it gives; gap names why not.
type legMark struct {
	mid, pnl *big.Rat
	quote    *compute.StructureQuote
	gap      string
}

// find matches a quote to a contract by (underlying, right, strike, expiry), never by an id,
// and takes the newest day that is not after the day of the evaluation.
func (m marker) find(underlying, right string, strike *big.Rat, expiry string) *compute.StructureQuote {
	var best *compute.StructureQuote
	for i := range m.quotes {
		q := &m.quotes[i]
		if q.Underlying != underlying || q.Right != right || q.Expiry != expiry {
			continue
		}
		if k, ok := parseDec(q.Strike); !ok || k.Cmp(strike) != 0 {
			continue
		}
		if m.asOf != "" && q.Day > m.asOf {
			continue
		}
		if best == nil || q.Day > best.Day {
			best = q
		}
	}
	return best
}

// stale tells whether a quote day is older than MarkMaxAgeSessions session days.
func (m marker) stale(day contracts.Day) bool {
	age := 0
	for _, d := range m.days {
		if d > day && (m.asOf == "" || d <= m.asOf) {
			age++
		}
	}
	return age > m.params.MarkMaxAgeSessions
}

// quoteOf is the quote of a leg or the reason there is none; an empty quote set is no_data.
func (m marker) quoteOf(l leg) (*compute.StructureQuote, string) {
	if len(m.quotes) == 0 {
		return nil, gapNoData
	}
	q := m.find(l.p.Underlying, l.p.Right, l.strike, l.p.Expiry)
	if q == nil {
		return nil, gapMissingQuote
	}
	return q, ""
}

// midOf is the mid of bid and ask rounded half-even to 4 places; a bid of zero or no ask has
// no mark (Decision 7).
func midOf(q *compute.StructureQuote) (*big.Rat, bool) {
	bid, ok1 := parseDec(q.Bid)
	ask, ok2 := parseDec(q.Ask)
	if !ok1 || !ok2 || bid.Sign() <= 0 || ask.Sign() <= 0 {
		return nil, false
	}
	return parseDec(fixed4(mul(add(bid, ask), big.NewRat(1, 2))))
}

// markOf marks one leg at mid; figure: structure.leg_pnl is signed quantity times multiplier
// times the mid minus the opening price.
func (m marker) markOf(l leg) legMark {
	q, gap := m.quoteOf(l)
	if gap != "" {
		return legMark{gap: gap}
	}
	mid, ok := midOf(q)
	switch {
	case !ok:
		return legMark{gap: gapMissingQuote}
	case m.stale(q.Day):
		return legMark{gap: gapStale}
	}
	pnl := mul(mul(l.qty, l.mult), sub(mid, l.price))
	return legMark{mid: mid, pnl: pnl, quote: q}
}

// spotOf is the underlying close of the newest quote of the underlying that has one.
func (m marker) spotOf(underlying string) (*big.Rat, string) {
	if len(m.quotes) == 0 {
		return nil, gapNoData
	}
	var day contracts.Day
	var spot *big.Rat
	for _, q := range m.quotes {
		c, ok := parseDec(q.UnderlyingClose)
		if q.Underlying != underlying || !ok || c.Sign() <= 0 || (m.asOf != "" && q.Day > m.asOf) {
			continue
		}
		if spot == nil || q.Day > day {
			spot, day = c, q.Day
		}
	}
	switch {
	case spot == nil:
		return nil, gapMissingQuote
	case m.stale(day):
		return nil, gapStale
	}
	return spot, ""
}

// fill sets the figures that need marks, the Greeks, the compare row and Decide.
func (m marker) fill(s *compute.Structure, u unit) {
	if s.Currency != "USD" {
		g := compute.FigureValue{Gap: gapUnsupported}
		f := &s.Figures
		f.UnrealizedPnL, f.EarnedShare, f.ExpiryDistance = g, g, g
		f.CloseCost, f.RollCredit, f.RollNet, f.RecentCommission, f.CashAfter = g, g, g, g, g
		s.Greeks = compute.GreekSet{Delta: g, Gamma: g, Theta: g, Vega: g}
		return
	}
	marks := make([]legMark, len(u.legs))
	for i, l := range u.legs {
		marks[i] = m.markOf(l)
		if marks[i].mid != nil {
			s.Legs[i].Mid = fixed4(marks[i].mid)
			s.Legs[i].LegPnL = fixed4(marks[i].pnl)
		}
	}
	m.fillGreeks(s, u)
	s.Figures.RecentCommission = m.recentCommission(u)
	m.fillDistances(s, u)
	if u.kind == kindOther {
		s.Figures.UnrealizedPnL, s.Figures.EarnedShare = noRule, noRule
		return
	}
	s.Figures.UnrealizedPnL = unrealized(marks)
	s.Figures.EarnedShare, _ = earnedShare(u, s.Figures.UnrealizedPnL)
	if shorts := shortLegs(u); len(shorts) == 1 {
		s.Decide = m.decide(u, s.Figures.EarnedShare, shorts[0])
	}
	if len(u.legs) == 1 && u.legs[0].short() { // Decision 13: the compare row
		s.CaptureParam, s.ExitDTEParam = m.params.CaptureTarget, itoa(m.params.ExitDTE)
		m.fillCompare(s, u, marks[0])
	}
}

// unrealized, figure: structure.unrealized_pnl, is the sum of the leg profits; one leg
// without a mark makes the sum a gap with that reason.
func unrealized(marks []legMark) compute.FigureValue {
	sum := new(big.Rat)
	for _, k := range marks {
		if k.gap != "" {
			return compute.FigureValue{Gap: k.gap}
		}
		sum = add(sum, k.pnl)
	}
	return val(sum)
}

// earnedShare, figure: structure.earned_share, is the premium kept now over the opening net
// premium: the profit now divided by the net credit, as a ratio of 6 places. The ratio is
// also returned exactly for Decide.
func earnedShare(u unit, pnl compute.FigureValue) (compute.FigureValue, *big.Rat) {
	if pnl.Gap != "" {
		return compute.FigureValue{Gap: pnl.Gap}, nil
	}
	credit := new(big.Rat)
	for _, l := range u.legs {
		credit = sub(credit, mul(mul(l.qty, l.mult), l.price))
	}
	if credit.Sign() <= 0 {
		return noData, nil
	}
	p, _ := parseDec(pnl.Value)
	r := new(big.Rat).Quo(p, credit)
	return compute.FigureValue{Value: fixed6(r)}, r
}

func shortLegs(u unit) []leg {
	var out []leg
	for _, l := range u.legs {
		if l.short() {
			out = append(out, l)
		}
	}
	return out
}

// decide is true when the short leg expires within ExitDTE days or the earned share is at
// least CaptureTarget (Decision 13).
func (m marker) decide(u unit, earned compute.FigureValue, short leg) bool {
	if target, ok := parseDec(m.params.CaptureTarget); ok && earned.Gap == "" {
		if e, ok := parseDec(earned.Value); ok && e.Cmp(target) >= 0 {
			return true
		}
	}
	exp, ok1 := parseDay(contracts.Day(short.p.Expiry))
	now, ok2 := parseDay(m.asOf)
	return ok1 && ok2 && int(exp.Sub(now).Hours()/24) <= m.params.ExitDTE
}

func parseDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse(dayLayout, string(d))
	return t, err == nil
}

// fillDistances sets figure: structure.expiry_distance and figure: structure.level_distance:
// (price - spot) / spot as a ratio of 6 places; the page moves the point two places.
func (m marker) fillDistances(s *compute.Structure, u unit) {
	spot, gap := m.spotOf(u.legs[0].p.Underlying)
	s.Figures.ExpiryDistance = noData
	if shorts := shortLegs(u); len(shorts) > 0 && u.kind != kindOther {
		n := shorts[0]
		for _, l := range shorts[1:] {
			if l.p.Expiry < n.p.Expiry {
				n = l
			}
		}
		s.Figures.ExpiryDistance = distance(n.strike, spot, gap)
	} else if u.kind == kindOther {
		s.Figures.ExpiryDistance = noRule
	}
	for i, lv := range s.Levels {
		if p, ok := parseDec(lv.Price); ok && gap == "" {
			s.Levels[i].Distance = distance(p, spot, gap).Value
		}
	}
}

func distance(price, spot *big.Rat, gap string) compute.FigureValue {
	if gap != "" {
		return compute.FigureValue{Gap: gap}
	}
	return compute.FigureValue{Value: fixed6(new(big.Rat).Quo(sub(price, spot), spot))}
}

// recentCommission, figure: structure.recent_commission, is the commissions of the rows of
// the legs on the last CommissionDays days up to the day of the evaluation.
func (m marker) recentCommission(u unit) compute.FigureValue {
	now, ok := parseDay(m.asOf)
	if !ok || m.params.CommissionDays <= 0 {
		return noData
	}
	from := now.AddDate(0, 0, -m.params.CommissionDays)
	sum, rows := new(big.Rat), 0
	for _, l := range u.legs {
		for _, f := range l.p.Fills {
			rows++
			c, ok := parseDec(f.Commission)
			if !ok {
				return missing
			}
			if d, ok := parseDay(f.Day); ok && !d.Before(from) && !d.After(now) {
				sum = add(sum, c)
			}
		}
	}
	if rows == 0 {
		return noData
	}
	return val(sum)
}

// fixed6 writes a ratio with 6 decimals, rounding half to even: the 4 places of the ratio
// times 100, with the point moved back two places.
func fixed6(r *big.Rat) string {
	t := fixed4(mul(r, big.NewRat(100, 1)))
	neg := strings.HasPrefix(t, "-")
	t = strings.TrimPrefix(t, "-")
	digits := strings.Replace(t, ".", "", 1)
	if len(digits) < 7 {
		digits = strings.Repeat("0", 7-len(digits)) + digits
	}
	out := digits[:len(digits)-6] + "." + digits[len(digits)-6:]
	if neg {
		out = "-" + out
	}
	return out
}

func itoa(n int) string { return new(big.Int).SetInt64(int64(n)).String() }
