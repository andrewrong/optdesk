// Package companydata derives the single quarters of one issuer from its stored SEC facts
// (companydataapi.Calculator.Quarters). Pure: no clock, no I/O. Numbers are exact decimals.
package companydata

import (
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const shareBand = "1.10" // largest over smallest diluted shares of Q1, Q2, Q3 and FY (a trial bound)

// cumulative end of quarter n, in days from the year start; a 3M length is the first band too.
var bands = [5][2]int{{}, {84, 98}, {175, 189}, {266, 280}, {yearMin, yearMax}}

// bandOf returns the quarter number 1 to 4 whose cumulative band holds d, or 0.
func bandOf(d int) int {
	for n := 1; n <= 4; n++ {
		if d >= bands[n][0] && d <= bands[n][1] {
			return n
		}
	}
	return 0
}

// oddYear is a length close to a year that is in no band.
func oddYear(d int) bool { return d > bands[3][1] && bandOf(d) == 0 && d <= yearMax+30 }

func isQuarterLength(d int) bool { return d >= bands[1][0] && d <= bands[1][1] }

// year is one fiscal year: its start, the quarter ends found and whether an odd long year exists.
type year struct {
	start  time.Time
	ends   [5]time.Time
	yearly bool // a fact of more than a year starts here but is in no band
}

type calc struct {
	idx   map[string]map[pkey][]row
	orig  map[time.Time]filing
	years []*year
}

// Quarters is the entry of this package.
func Quarters(facts []contracts.FundamentalFact) ([]companydataapi.Quarter, error) {
	rows := parse(facts)
	c := &calc{idx: index(rows), orig: originals(rows)}
	c.findYears()
	out := []companydataapi.Quarter{}
	for _, y := range c.years {
		out = append(out, c.quarters(y)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PeriodEnd != out[j].PeriodEnd {
			return out[i].PeriodEnd > out[j].PeriodEnd
		}
		return out[i].Number > out[j].Number
	})
	return out, nil
}

// findYears finds the fiscal year starts: the start of a 6M, 9M or 12M fact, or of a 3M fact that
// does not continue a year already found.
func (c *calc) findYears() {
	starts := map[time.Time]bool{}
	threeM := map[time.Time]bool{}
	for _, g := range c.idx {
		for k := range g {
			if k.start.IsZero() {
				continue
			}
			d := daysBetween(k.start, k.end)
			switch n := bandOf(d); {
			case n > 1:
				starts[k.start] = true
			case n == 1:
				threeM[k.start] = true
			}
		}
	}
	for _, t := range sortedDays(threeM) {
		if starts[t] || c.continues(starts, t) {
			continue
		}
		starts[t] = true
	}
	for _, s := range sortedDays(starts) {
		y := &year{start: s}
		c.years = append(c.years, y)
		c.findEnds(y)
	}
}

// continues reports whether a 3M fact starting at t belongs to a year starting before it.
func (c *calc) continues(starts map[time.Time]bool, t time.Time) bool {
	for s := range starts {
		if s.Before(t) && daysBetween(s, t) <= bands[3][1]+1 {
			return true
		}
	}
	return false
}

// findEnds sets the end of each quarter: the end most facts of the year give.
func (c *calc) findEnds(y *year) {
	count := [5]map[time.Time]int{}
	for _, g := range c.idx {
		for k := range g {
			if k.start.IsZero() || k.start.Before(y.start) {
				continue
			}
			ln := daysBetween(k.start, k.end)
			n := bandOf(daysBetween(y.start, k.end))
			switch {
			case k.start.Equal(y.start) && bandOf(ln) == n && n > 0:
			case isQuarterLength(ln) && n > 1 && k.start.After(y.start) && bandOf(daysBetween(y.start, k.start)-1) == n-1:
			case k.start.Equal(y.start) && oddYear(ln):
				y.yearly = true
				continue
			default:
				continue
			}
			if count[n] == nil {
				count[n] = map[time.Time]int{}
			}
			count[n][k.end]++
		}
	}
	for n := 1; n <= 4; n++ {
		for e, v := range count[n] {
			if b := y.ends[n]; b.IsZero() || v > count[n][b] || (v == count[n][b] && e.Before(b)) {
				y.ends[n] = e
			}
		}
	}
	if y.yearly && y.ends[4].IsZero() {
		for _, g := range c.idx {
			for k := range g {
				if k.start.Equal(y.start) && oddYear(daysBetween(k.start, k.end)) && (y.ends[4].IsZero() || k.end.Before(y.ends[4])) {
					y.ends[4] = k.end
				}
			}
		}
	}
}

// ytd finds the picked year-to-date fact of concept up to quarter n.
func (c *calc) ytd(y *year, concept string, n int) (operand, bool) {
	if y.ends[n].IsZero() {
		return operand{}, false
	}
	if rs := c.idx[concept][pkey{y.start, y.ends[n]}]; len(rs) > 0 && bandOf(daysBetween(y.start, y.ends[n])) == n {
		return pick(rs), true
	}
	return operand{}, false
}

// direct finds the picked fact of concept that covers quarter n alone (a 3M fact).
func (c *calc) direct(y *year, concept string, n int) (operand, bool) {
	e := y.ends[n]
	if e.IsZero() {
		return operand{}, false
	}
	for k, rs := range c.idx[concept] {
		if !k.end.Equal(e) || k.start.IsZero() || !isQuarterLength(daysBetween(k.start, k.end)) {
			continue
		}
		if n == 1 && !k.start.Equal(y.start) {
			continue
		}
		if n > 1 && bandOf(daysBetween(y.start, k.start)-1) != n-1 {
			continue
		}
		return pick(rs), true
	}
	return operand{}, false
}

func fmtDec(d decimal.Decimal) string {
	places := int32(0)
	if e := d.Exponent(); e < 0 {
		places = -e
	}
	return d.StringFixed(places)
}

func valueCell(op operand, concept string) companydataapi.Cell {
	return companydataapi.Cell{Value: fmtDec(op.val), Restated: op.restated, Concept: concept}
}

func derived(v decimal.Decimal, concept string, ops ...operand) companydataapi.Cell {
	c := companydataapi.Cell{Value: fmtDec(v), Derived: true, Concept: concept}
	for _, o := range ops {
		c.Restated = c.Restated || o.restated
	}
	return c
}

func missing(why companydataapi.Why) companydataapi.Cell {
	return companydataapi.Cell{Missing: true, Why: why}
}

func (c *calc) flow(y *year, col column, n int) companydataapi.Cell {
	related := false
	for _, k := range col.concepts {
		if op, ok := c.direct(y, k, n); ok {
			return valueCell(op, k)
		}
		if n == 1 {
			continue
		}
		a, okA := c.ytd(y, k, n)
		b, okB := c.ytd(y, k, n-1)
		related = related || okA || okB
		if okA && okB && consistent(a, b) {
			return derived(a.val.Sub(b.val), k, a, b)
		}
	}
	if related || (n == 4 && y.yearly) {
		return missing(companydataapi.WhyCannotDerive)
	}
	return missing(companydataapi.WhyNoFact)
}

func (c *calc) instant(y *year, col column, n int) companydataapi.Cell {
	if y.ends[n].IsZero() {
		return missing(companydataapi.WhyNoFact)
	}
	for _, k := range col.concepts {
		if rs := c.idx[k][pkey{time.Time{}, y.ends[n]}]; len(rs) > 0 {
			return valueCell(pick(rs), k)
		}
	}
	return missing(companydataapi.WhyNoFact)
}

func (c *calc) shares(y *year, n int) companydataapi.Cell {
	for _, k := range colShares.concepts {
		if op, ok := c.direct(y, k, n); ok {
			return valueCell(op, k)
		}
	}
	return missing(companydataapi.WhyNoFact)
}

// eps reads Q1 to Q3 from 3M facts; Q4 from a 3M fact, else the year minus three quarters.
func (c *calc) eps(y *year, n int) companydataapi.Cell {
	k := colEPS.concepts[0]
	if op, ok := c.direct(y, k, n); ok {
		return valueCell(op, k)
	}
	if n < 4 {
		return missing(companydataapi.WhyNoFact)
	}
	fy, okY := c.ytd(y, k, 4)
	qs := make([]operand, 0, 3)
	for i := 1; i <= 3; i++ {
		if q, ok := c.direct(y, k, i); ok {
			qs = append(qs, q)
		}
	}
	if !okY && len(qs) == 0 {
		return missing(companydataapi.WhyNoFact)
	}
	if !okY || len(qs) < 3 {
		return missing(companydataapi.WhyCannotDerive)
	}
	why, ok := c.shareBasis(y)
	if !ok {
		return missing(why)
	}
	if !consistent(append([]operand{fy}, qs...)...) {
		return missing(companydataapi.WhyCannotDerive)
	}
	v := fy.val.Sub(qs[0].val).Sub(qs[1].val).Sub(qs[2].val)
	return derived(v, k, append([]operand{fy}, qs...)...)
}

// shareBasis checks that the diluted shares of Q1, Q2, Q3 and FY lie within one band.
func (c *calc) shareBasis(y *year) (companydataapi.Why, bool) {
	band, _ := decimal.NewFromString(shareBand)
	for _, k := range colShares.concepts {
		var vs []decimal.Decimal
		for i := 1; i <= 3; i++ {
			if op, ok := c.direct(y, k, i); ok {
				vs = append(vs, op.val)
			}
		}
		if op, ok := c.ytd(y, k, 4); ok {
			vs = append(vs, op.val)
		}
		if len(vs) < 4 {
			continue
		}
		lo, hi := vs[0], vs[0]
		for _, v := range vs {
			if v.LessThan(lo) {
				lo = v
			}
			if v.GreaterThan(hi) {
				hi = v
			}
		}
		if !lo.IsPositive() {
			return companydataapi.WhyCannotDerive, false
		}
		if hi.GreaterThan(lo.Mul(band)) {
			return companydataapi.WhyShareBasisChanged, false
		}
		return companydataapi.WhyNone, true
	}
	return companydataapi.WhyCannotDerive, false
}

var fpOf = [5]string{"", "Q1", "Q2", "Q3", "FY"}

// mismatch reports whether the original filing of a quarter end names another quarter than the
// periods do; an empty fp is "not re-fetched yet" and skips the check.
func (c *calc) mismatch(y *year) bool {
	for n := 1; n <= 4; n++ {
		if f, ok := c.orig[y.ends[n]]; ok && f.fp != "" && f.fp != fpOf[n] {
			return true
		}
	}
	return false
}

func (c *calc) quarters(y *year) []companydataapi.Quarter {
	var out []companydataapi.Quarter
	bad := c.mismatch(y)
	month := 0
	if !y.ends[4].IsZero() && bandOf(daysBetween(y.start, y.ends[4])) == 4 {
		month = int(y.ends[4].Month())
	}
	for n := 1; n <= 4; n++ {
		e := y.ends[n]
		if e.IsZero() {
			continue
		}
		q := companydataapi.Quarter{Number: n, FiscalYearEndMonth: month, PeriodEnd: day(e)}
		if f, ok := c.orig[e]; ok {
			q.FilingDate = day(f.filed)
			if f.fy != nil {
				q.FiscalYear = *f.fy
				q.Label = fmt.Sprintf("FY%d Q%d", *f.fy, n)
			}
		}
		if bad {
			m := missing(companydataapi.WhyPeriodMismatch)
			q.Revenue, q.OperatingIncome, q.NetIncome, q.EPSDiluted, q.SharesDiluted = m, m, m, m, m
			q.OperatingCashFlow, q.Capex, q.FreeCash, q.Buybacks, q.Dividends = m, m, m, m, m
			q.Cash, q.LongTermDebt, q.ShortTermDebt = m, m, m
			out = append(out, q)
			continue
		}
		q.Revenue = c.flow(y, colRevenue, n)
		q.OperatingIncome = c.flow(y, colOperatingIncome, n)
		q.NetIncome = c.flow(y, colNetIncome, n)
		q.EPSDiluted = c.eps(y, n)
		q.SharesDiluted = c.shares(y, n)
		q.OperatingCashFlow = c.flow(y, colOCF, n)
		q.Capex = c.flow(y, colCapex, n)
		q.FreeCash = freeCash(q.OperatingCashFlow, q.Capex)
		q.Buybacks = c.flow(y, colBuybacks, n)
		q.Dividends = c.flow(y, colDividends, n)
		q.Cash = c.instant(y, colCash, n)
		q.LongTermDebt = c.instant(y, colLongDebt, n)
		q.ShortTermDebt = c.instant(y, colShortDebt, n)
		out = append(out, q)
	}
	return out
}

func day(t time.Time) contracts.Day { return contracts.Day(t.Format("2006-01-02")) }
