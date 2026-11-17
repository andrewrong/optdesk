package optionattr

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// stepper re-prices neighbouring quote days in the fixed order of Decision 1.
type stepper struct {
	p      optionattrapi.Pricer
	bump   decimal.Decimal
	strike decimal.Decimal
	expiry contracts.Day
	rates  []optionattrapi.Rate
	maxAge int
	noRate []contracts.Day
}

// endpoint is where a step ends: a quote day, or the terminal point at EndClose.
type endpoint struct {
	day      contracts.Day
	spot     decimal.Decimal
	value    decimal.Decimal // the stored mid, or the intrinsic value
	unsolved bool            // the end is the intrinsic value: sigma1 is not solved, the IV step is 0
	wide     bool
	ref      string
	terminal bool
	multi    bool
}

func (s *stepper) run(l *lotData, valid []quoteDay, gaps []contracts.Day, first, last int) ([]optionattrapi.Step, error) {
	steps := []optionattrapi.Step{}
	for i := first; i < last; i++ {
		q := l.openedBy(valid[i].day).Sub(l.closedBefore(valid[i+1].day))
		if !q.IsPositive() {
			continue
		}
		b := valid[i+1]
		e := endpoint{day: b.day, spot: b.spot, value: b.mark, unsolved: b.day == s.expiry,
			wide: b.wide && b.day != s.expiry, ref: b.ref.IV}
		for _, g := range gaps {
			if g > valid[i].day && g < b.day {
				e.multi = true
			}
		}
		st, err := s.split(valid[i], e, q.Mul(l.mult))
		if err != nil {
			return nil, err
		}
		steps = append(steps, st)
	}
	if !ended(l.in.Lot.End) {
		return steps, nil
	}
	q := l.openQty.Sub(l.closedQty)
	if !q.IsPositive() {
		return steps, nil
	}
	n := valid[last]
	end := intrinsic(l.strike, l.endClose)
	days, err := calDays(n.day, l.in.Lot.EndDay)
	if err != nil {
		return nil, err
	}
	qm := q.Mul(l.mult)
	if n.day == s.expiry { // Decision 7: no pricer call, all of the move is direction
		return append(steps, optionattrapi.Step{
			From: n.day, To: l.in.Lot.EndDay, CalendarDays: days, Terminal: true,
			Direction: end.Sub(n.mark).Neg().Mul(qm).String(),
			MoveCost:  "0", IVRemark: "0", Time: "0", Unsplit: "0", RefIVFrom: n.ref.IV,
		}), nil
	}
	st, err := s.split(n, endpoint{day: l.in.Lot.EndDay, spot: l.endClose, value: end, unsolved: true,
		terminal: true}, qm)
	if err != nil {
		return nil, err
	}
	return append(steps, st), nil
}

// rate is the latest point on or before day that is at most maxAge days old, as a fraction.
func (s *stepper) rate(day contracts.Day) (decimal.Decimal, bool) {
	var best *optionattrapi.Rate
	for i := range s.rates {
		r := &s.rates[i]
		if r.Day > day {
			continue
		}
		if age, err := calDays(r.Day, day); err != nil || age > s.maxAge {
			continue
		}
		if best == nil || r.Day > best.Day {
			best = r
		}
	}
	if best == nil {
		return decimal.Zero, false
	}
	p, err := decimal.NewFromString(best.Percent)
	if err != nil {
		return decimal.Zero, false
	}
	return p.Div(decimal.NewFromInt(100)), true
}

func (s *stepper) years(from contracts.Day) (decimal.Decimal, error) {
	d, err := calDays(from, s.expiry)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromInt(int64(d)).Div(decimal.NewFromInt(daysInYear)), nil
}

func (s *stepper) value(spot, strike, years, rate, vol decimal.Decimal) (decimal.Decimal, error) {
	v, err := s.p.Value("P", spot.String(), strike.String(), years.String(), rate.String(), vol.String())
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(v)
}

func (s *stepper) implied(spot, strike, years, rate, price decimal.Decimal) (decimal.Decimal, error) {
	v, err := s.p.Implied("P", spot.String(), strike.String(), years.String(), rate.String(), price.String())
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(v)
}

// split prices one step from quote day a to e; qm is quantity times multiplier.
func (s *stepper) split(a quoteDay, e endpoint, qm decimal.Decimal) (optionattrapi.Step, error) {
	days, err := calDays(a.day, e.day)
	if err != nil {
		return optionattrapi.Step{}, err
	}
	st := optionattrapi.Step{From: a.day, To: e.day, CalendarDays: days, MultiDay: e.multi, Wide: a.wide || e.wide,
		Terminal: e.terminal, RefIVFrom: a.ref.IV, RefIVTo: e.ref}
	seller := func(d decimal.Decimal) string { return d.Neg().Mul(qm).String() }
	unsplit := func(reason string) (optionattrapi.Step, error) {
		st.Direction, st.MoveCost, st.IVRemark, st.Time = "0", "0", "0", "0"
		st.Unsplit, st.Reason = seller(e.value.Sub(a.mark)), reason
		return st, nil
	}
	rate, ok := s.rate(a.day)
	if !ok {
		s.noRate = append(s.noRate, a.day)
		return unsplit("no rate within " + fmt.Sprint(s.maxAge) + " days of " + string(a.day))
	}
	t0, err := s.years(a.day)
	if err != nil {
		return optionattrapi.Step{}, err
	}
	sigma0, err := s.implied(a.spot, s.strike, t0, rate, a.mark)
	if err != nil {
		return unsplit("IV not solved on " + string(a.day) + ": " + err.Error())
	}
	st.IVFrom = sigma0.String()
	h := a.spot.Mul(s.bump)
	up, err := s.value(a.spot.Add(h), s.strike, t0, rate, sigma0)
	if err != nil {
		return unsplit("delta: " + err.Error())
	}
	down, err := s.value(a.spot.Sub(h), s.strike, t0, rate, sigma0)
	if err != nil {
		return unsplit("delta: " + err.Error())
	}
	delta0 := up.Sub(down).Div(h.Mul(two))
	b, err := s.value(e.spot, s.strike, t0, rate, sigma0)
	if err != nil {
		return unsplit("value: " + err.Error())
	}
	c := b
	if !e.unsolved {
		t1, err := s.years(e.day)
		if err != nil {
			return optionattrapi.Step{}, err
		}
		sigma1, err := s.implied(e.spot, s.strike, t1, rate, e.value)
		if err != nil {
			return unsplit("IV not solved on " + string(e.day) + ": " + err.Error())
		}
		st.IVTo = sigma1.String()
		if c, err = s.value(e.spot, s.strike, t0, rate, sigma1); err != nil {
			return unsplit("value: " + err.Error())
		}
	}
	deltaPart := delta0.Mul(e.spot.Sub(a.spot))
	st.Direction = seller(deltaPart)
	st.MoveCost = seller(b.Sub(a.mark).Sub(deltaPart))
	st.IVRemark = seller(c.Sub(b))
	st.Time = seller(e.value.Sub(c))
	st.Unsplit = "0"
	return st, nil
}
