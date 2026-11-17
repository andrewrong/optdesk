// Package valuelines implements valuelinesapi.Calculator: the price lines, the zone flags, the
// price band, the R table proposal and the review check. It works in decimal text and rounds
// nothing except in RoundText.
package valuelines

import (
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

// Calc is the calculator; it holds no state.
type Calc struct{}

var _ valuelinesapi.Calculator = Calc{}

// New returns the calculator.
func New() Calc { return Calc{} }

var (
	dOne     = decimal.NewFromInt(1)
	dFive    = decimal.NewFromInt(5)
	dHundred = decimal.NewFromInt(100)
	dClear   = decimal.RequireFromString("1.2")
	dCore    = decimal.RequireFromString("1.5")
	dBatch   = decimal.RequireFromString("0.05")
)

func parse(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: %q", valuelinesapi.ErrBadInput, s)
	}
	return d, nil
}

type marginRow struct{ pFrom, m, w, h, s decimal.Decimal }

// pickMargin returns the row with the greatest PFrom at or below p.
func pickMargin(rows []valuelinesapi.MarginRow, p decimal.Decimal) (marginRow, bool, error) {
	var best marginRow
	found := false
	for _, r := range rows {
		from, err := parse(r.PFrom)
		if err != nil {
			return best, false, err
		}
		if from.GreaterThan(p) || (found && from.LessThanOrEqual(best.pFrom)) {
			continue
		}
		var mr marginRow
		mr.pFrom = from
		for i, dst := range []*decimal.Decimal{&mr.m, &mr.w, &mr.h, &mr.s} {
			v, err := parse([]string{r.M, r.W, r.H, r.S}[i])
			if err != nil {
				return best, false, err
			}
			*dst = v
		}
		best, found = mr, true
	}
	return best, found, nil
}

// Lines computes the lines of a valuation; see valuelinesapi.Calculator.
func (Calc) Lines(in valuelinesapi.Input) (valuelinesapi.Lines, error) {
	var out valuelinesapi.Lines
	v, err := parse(in.V)
	if err != nil {
		return out, err
	}
	p, err := parse(in.P)
	if err != nil {
		return out, err
	}
	if !v.IsPositive() || p.LessThan(dOne) || p.GreaterThan(dFive) {
		return out, fmt.Errorf("%w: V %s P %s", valuelinesapi.ErrBadInput, in.V, in.P)
	}
	var rText string
	switch in.Market {
	case valuelinesapi.MarketUS:
		rText = in.R.US
	case valuelinesapi.MarketJP:
		rText = in.R.JP
	case valuelinesapi.MarketHKCN:
		rText = in.R.HKCN
	default:
		return out, fmt.Errorf("%w: market %q", valuelinesapi.ErrBadInput, in.Market)
	}
	r, err := parse(rText)
	if err != nil {
		return out, err
	}
	row, ok, err := pickMargin(in.Margin, p)
	if err != nil {
		return out, err
	}
	if !ok {
		return out, fmt.Errorf("%w: no margin row for P %s", valuelinesapi.ErrBadInput, in.P)
	}

	c := v.Mul(r)
	heavy := c.Mul(dOne.Sub(row.h))
	buyLow := c.Mul(dOne.Sub(row.m).Sub(dBatch))
	buyHigh := c.Mul(dOne.Sub(row.m))
	fairLow := c.Mul(dOne.Sub(row.w))
	fairHigh := c.Mul(dOne.Add(row.w))
	trim := c.Mul(dOne.Add(row.s))
	clear := v.Mul(dClear)
	core := v.Mul(dCore)

	out = valuelinesapi.Lines{
		C: c.String(), R: r.String(),
		Heavy: heavy.String(), BuyLow: buyLow.String(), BuyHigh: buyHigh.String(),
		FairLow: fairLow.String(), FairHigh: fairHigh.String(),
		Trim: trim.String(), Clear: clear.String(), Core: core.String(),
		M: row.m.String(), W: row.w.String(), H: row.h.String(), S: row.s.String(),
	}
	if in.Market == valuelinesapi.MarketHKCN {
		out.ZoneNote = "no_zone_market"
		return out, nil
	}
	if in.Close == "" {
		out.ZoneNote = "no_close"
		return out, nil
	}
	cl, err := parse(in.Close)
	if err != nil {
		return valuelinesapi.Lines{}, err
	}
	f := valuelinesapi.Flags{
		AtHeavy: cl.LessThanOrEqual(heavy),
		InBuy:   cl.LessThanOrEqual(buyHigh),
		InFair:  cl.GreaterThanOrEqual(fairLow) && cl.LessThanOrEqual(fairHigh),
		AtTrim:  cl.GreaterThanOrEqual(trim),
		AtClear: cl.GreaterThanOrEqual(clear),
		AtCore:  cl.GreaterThanOrEqual(core),
	}
	out.Flags = &f
	out.Zone = zoneLabel(f)
	return out, nil
}

// zoneLabel picks the label by the fixed precedence core, clear, trim, fair, heavy, buy.
func zoneLabel(f valuelinesapi.Flags) string {
	switch {
	case f.AtCore:
		return "core"
	case f.AtClear:
		return "clear"
	case f.AtTrim:
		return "trim"
	case f.InFair:
		return "fair"
	case f.AtHeavy:
		return "heavy"
	case f.InBuy:
		return "buy"
	}
	return "between"
}

// Band returns the price axis for the lines and the close. A text that is not a number is
// left out; with nothing to place the band is empty.
func (Calc) Band(l valuelinesapi.Lines, closeText string) valuelinesapi.Band {
	type named struct {
		name, text string
		val        decimal.Decimal
	}
	cands := []named{
		{name: "heavy", text: l.Heavy}, {name: "buy_low", text: l.BuyLow}, {name: "buy_high", text: l.BuyHigh},
		{name: "fair_low", text: l.FairLow}, {name: "fair_high", text: l.FairHigh},
		{name: "trim", text: l.Trim}, {name: "clear", text: l.Clear}, {name: "core", text: l.Core},
		{name: "close", text: closeText},
	}
	var marks []named
	for _, c := range cands {
		if c.text == "" {
			continue
		}
		d, err := decimal.NewFromString(c.text)
		if err != nil {
			continue
		}
		c.val = d
		marks = append(marks, c)
	}
	if len(marks) == 0 {
		return valuelinesapi.Band{}
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].val.LessThan(marks[j].val) })
	lo, hi := marks[0].val, marks[len(marks)-1].val
	pad := hi.Sub(lo).Mul(dBatch)
	if pad.IsZero() {
		pad = dOne
	}
	axisMin, axisMax := lo.Sub(pad), hi.Add(pad)
	span := axisMax.Sub(axisMin)
	band := valuelinesapi.Band{Min: axisMin.String(), Max: axisMax.String()}
	for _, m := range marks {
		band.Marks = append(band.Marks, valuelinesapi.Mark{
			Name: m.name, Value: m.val.String(), Pos: m.val.Sub(axisMin).Div(span).String(),
		})
	}
	return band
}

// ChangePercent returns (to-from)/from*100 as decimal text.
func (Calc) ChangePercent(from, to string) (string, error) {
	f, err := parse(from)
	if err != nil {
		return "", err
	}
	t, err := parse(to)
	if err != nil {
		return "", err
	}
	if f.IsZero() {
		return "", fmt.Errorf("%w: change from 0", valuelinesapi.ErrBadInput)
	}
	return t.Sub(f).Div(f).Mul(dHundred).String(), nil
}
