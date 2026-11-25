// Package optionattr splits the profit of one short put lot into direction, move cost, IV re-marking and
// time, so that entry + exit + steps + commission + assigned intrinsic is the ledger figure
// (docs/design/option-attribution.md, Decisions 1 to 8). Decimal arithmetic only; no float, no clock.
package optionattr

import (
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout      = "2006-01-02"
	daysInYear     = 365 // trial: the basis of the ITM probability (Decision 1)
	defaultRateAge = 7   // Input.MaxRateAgeDays when it is not set
)

var (
	// deltaBump is h as a fraction of spot (trial, appendix A of the plan; Decision 3).
	deltaBump = decimal.New(5, -3)
	// wideLimit is the quote width above which a row is flagged Wide: the candidate gate's parameter.
	wideLimit = decimal.New(10, -2)
	two       = decimal.NewFromInt(2)
)

var _ optionattrapi.Attributor = (*attributor)(nil)

type attributor struct {
	p    optionattrapi.Pricer
	bump decimal.Decimal
}

// New returns the Attributor that re-prices with p.
func New(p optionattrapi.Pricer) optionattrapi.Attributor { return newWithBump(p, deltaBump) }

func newWithBump(p optionattrapi.Pricer, bump decimal.Decimal) *attributor {
	return &attributor{p: p, bump: bump}
}

type fill struct {
	day         contracts.Day
	open        bool
	qty, price  decimal.Decimal
	commission  decimal.Decimal
	idx         int // quote index it is charged on
	chargedMark decimal.Decimal
}

type quoteDay struct {
	day        contracts.Day
	mark, spot decimal.Decimal
	wide       bool
	ref        optionattrapi.Ref
}

func badf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", optionattrapi.ErrBadInput, fmt.Sprintf(format, a...))
}

func parseDec(name, s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return d, badf("%s %q is not a number", name, s)
	}
	return d, nil
}

func calDays(a, b contracts.Day) (int, error) {
	ta, err := time.Parse(dayLayout, string(a))
	if err != nil {
		return 0, badf("day %q", a)
	}
	tb, err := time.Parse(dayLayout, string(b))
	if err != nil {
		return 0, badf("day %q", b)
	}
	return int(tb.Sub(ta).Hours() / 24), nil
}

func intrinsic(strike, spot decimal.Decimal) decimal.Decimal {
	return decimal.Max(strike.Sub(spot), decimal.Zero)
}

func ended(end string) bool { return end == "expired" || end == "assigned" || end == "exercised" }

// lotData is the parsed lot.
type lotData struct {
	in                 optionattrapi.Input
	strike, mult       decimal.Decimal
	endClose, ledger   decimal.Decimal
	fills              []fill
	commission         decimal.Decimal
	openQty, closedQty decimal.Decimal
}

func parseLot(in optionattrapi.Input) (*lotData, error) {
	var err error
	l := &lotData{in: in}
	if l.strike, err = parseDec("strike", in.Lot.Strike); err != nil {
		return nil, err
	}
	if l.mult, err = parseDec("multiplier", in.Lot.Multiplier); err != nil {
		return nil, err
	}
	if l.ledger, err = parseDec("LedgerPnL", in.LedgerPnL); err != nil {
		return nil, err
	}
	if ended(in.Lot.End) {
		if l.endClose, err = parseDec("EndClose", in.Lot.EndClose); err != nil {
			return nil, err
		}
	}
	opens := 0
	for _, f := range in.Lot.Fills {
		fl := fill{day: f.Day}
		switch f.Side {
		case "open":
			fl.open = true
			opens++
		case "close":
		default:
			return nil, badf("fill side %q", f.Side)
		}
		if fl.qty, err = parseDec("quantity", f.Quantity); err != nil {
			return nil, err
		}
		if fl.price, err = parseDec("price", f.Price); err != nil {
			return nil, err
		}
		if f.Commission != "" {
			if fl.commission, err = parseDec("commission", f.Commission); err != nil {
				return nil, err
			}
		}
		if !fl.qty.IsPositive() {
			return nil, badf("quantity %q", f.Quantity)
		}
		l.commission = l.commission.Add(fl.commission)
		if fl.open {
			l.openQty = l.openQty.Add(fl.qty)
		} else {
			l.closedQty = l.closedQty.Add(fl.qty)
		}
		l.fills = append(l.fills, fl)
	}
	if opens == 0 {
		return nil, badf("lot without an open fill")
	}
	return l, nil
}

// assigned is the intrinsic value moved to the shares (Decision 5).
func (l *lotData) assigned() decimal.Decimal {
	if l.in.Lot.End != "assigned" && l.in.Lot.End != "exercised" {
		return decimal.Zero
	}
	return intrinsic(l.strike, l.endClose).Mul(l.openQty.Sub(l.closedQty)).Mul(l.mult)
}

// quoteDays splits the stored rows into quote days and gap days (Decision 8), ignoring rows after the
// life of the lot.
func (l *lotData) quoteDays() (valid []quoteDay, gaps []contracts.Day) {
	limit := l.in.Lot.Expiry
	if ended(l.in.Lot.End) && l.in.Lot.EndDay != "" && l.in.Lot.EndDay < limit {
		limit = l.in.Lot.EndDay
	}
	for _, q := range l.in.Quotes {
		if q.Day > limit {
			continue
		}
		bid, e1 := decimal.NewFromString(q.Bid)
		ask, e2 := decimal.NewFromString(q.Ask)
		spot, e3 := decimal.NewFromString(q.Spot)
		if e1 != nil || e2 != nil || e3 != nil || !ask.IsPositive() || bid.IsNegative() || !spot.IsPositive() {
			gaps = append(gaps, q.Day)
			continue
		}
		mid := bid.Add(ask).Div(two)
		qd := quoteDay{day: q.Day, mark: mid, spot: spot, ref: q.Ref,
			wide: ask.Sub(bid).Div(mid).GreaterThan(wideLimit)}
		if q.Day == l.in.Lot.Expiry { // Decision 7: the expiry day is priced at the intrinsic value
			qd.mark = intrinsic(l.strike, spot)
		}
		valid = append(valid, qd)
	}
	return valid, gaps
}

// openedBy is the quantity opened on days up to day; closedBefore the quantity closed on days before day.
func (l *lotData) openedBy(day contracts.Day) decimal.Decimal {
	s := decimal.Zero
	for _, f := range l.fills {
		if f.open && f.day <= day {
			s = s.Add(f.qty)
		}
	}
	return s
}

func (l *lotData) closedBefore(day contracts.Day) decimal.Decimal {
	s := decimal.Zero
	for _, f := range l.fills {
		if !f.open && f.day < day {
			s = s.Add(f.qty)
		}
	}
	return s
}

// charge finds the quote day each fill is charged on (Decision 4); false names a fill without one.
func (l *lotData) charge(valid []quoteDay) (string, bool) {
	for i := range l.fills {
		f := &l.fills[i]
		f.idx = -1
		if f.open {
			for j, q := range valid {
				if q.day >= f.day {
					f.idx = j
					break
				}
			}
		} else {
			for j, q := range valid {
				if q.day <= f.day {
					f.idx = j
				}
			}
		}
		if f.idx < 0 {
			return fmt.Sprintf("no quote day for the fill of %s", f.day), false
		}
		f.chargedMark = valid[f.idx].mark
	}
	return "", true
}

func (a *attributor) Attribute(in optionattrapi.Input) (optionattrapi.Result, error) {
	l, err := parseLot(in)
	if err != nil {
		return optionattrapi.Result{}, err
	}
	res := optionattrapi.Result{
		Currency: in.Lot.Currency, Commission: l.commission.Neg().String(),
		AssignedIntrinsic: l.assigned().String(), LedgerPnL: in.LedgerPnL,
		Steps: []optionattrapi.Step{}, GapDays: []contracts.Day{}, NoRate: []contracts.Day{},
	}
	valid, gaps := l.quoteDays()
	if len(valid) == 0 {
		res.NoQuotes, res.Reason = true, "no stored quotes"
		return res, nil
	}
	if why, ok := l.charge(valid); !ok {
		res.NoQuotes, res.Reason = true, why
		return res, nil
	}
	first, last := len(valid), len(valid)-1
	maxClose := -1
	for _, f := range l.fills {
		if f.open && f.idx < first {
			first = f.idx
		}
		if !f.open && f.idx > maxClose {
			maxClose = f.idx
		}
	}
	if in.Lot.End == "closed" && maxClose >= 0 {
		last = maxClose
	}
	if last < first {
		last = first
	}

	entry, exit, hasExit := l.lines(valid, &res)
	res.Entry = entry.String()
	if hasExit {
		res.Exit = exit.String()
	}

	sp := &stepper{p: a.p, bump: a.bump, strike: l.strike, expiry: in.Lot.Expiry, rates: in.Rates, maxAge: in.MaxRateAgeDays}
	if sp.maxAge <= 0 {
		sp.maxAge = defaultRateAge
	}
	steps, err := sp.run(l, valid, gaps, first, last)
	if err != nil {
		return optionattrapi.Result{}, err
	}
	res.Steps = steps
	res.NoRate = append(res.NoRate, sp.noRate...)
	res.GapDays = gapsBetween(gaps, valid[first].day, valid[last].day)

	dir, mc, iv, tm, un := sumSteps(&res)
	if in.Lot.End == "open" {
		res.MarkDay = valid[last].day
		if in.LedgerMarkDay != "" && in.LedgerMarkDay != res.MarkDay {
			res.Reason = fmt.Sprintf("ledger mark day %s differs from last quote day %s", in.LedgerMarkDay, res.MarkDay)
			return res, nil
		}
	}
	sum := entry.Add(exit).Add(dir).Add(mc).Add(iv).Add(tm).Add(un).Sub(l.commission).Add(l.assigned())
	res.Difference = l.ledger.Sub(sum).RoundBank(4).String() // the only rounding of the package
	return res, nil
}

// lines is the entry and exit lines of the fills (Decision 4) and whether there is an exit.
func (l *lotData) lines(valid []quoteDay, res *optionattrapi.Result) (entry, exit decimal.Decimal, hasExit bool) {
	for _, f := range l.fills {
		line := f.price.Sub(f.chargedMark).Mul(f.qty).Mul(l.mult)
		if f.open {
			entry = entry.Add(line)
			if f.day != valid[f.idx].day {
				res.LateEntry = true
			}
		} else {
			exit = exit.Sub(line)
			hasExit = true
		}
	}
	return entry, exit, hasExit
}

// sumSteps totals the blocks of the steps into res and counts the unsplit ones.
func sumSteps(res *optionattrapi.Result) (dir, mc, iv, tm, un decimal.Decimal) {
	for _, s := range res.Steps {
		dir = dir.Add(decimal.RequireFromString(s.Direction))
		mc = mc.Add(decimal.RequireFromString(s.MoveCost))
		iv = iv.Add(decimal.RequireFromString(s.IVRemark))
		tm = tm.Add(decimal.RequireFromString(s.Time))
		un = un.Add(decimal.RequireFromString(s.Unsplit))
		if s.Reason != "" {
			res.UnsplitSteps++
		}
	}
	res.Direction, res.MoveCost, res.IVRemark, res.Time, res.Unsplit = dir.String(), mc.String(), iv.String(), tm.String(), un.String()
	return dir, mc, iv, tm, un
}

// gapsBetween lists the gap days strictly between from and to, ascending; never nil.
func gapsBetween(gaps []contracts.Day, from, to contracts.Day) []contracts.Day {
	out := []contracts.Day{}
	for _, d := range gaps {
		if d > from && d < to {
			out = append(out, d)
		}
	}
	slices.Sort(out)
	return out
}
