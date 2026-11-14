// Package bsm is a Black-Scholes-Merton pricer of European options with no dividend yield.
//
// It is an own implementation, because no mainstream Go library fits (a maintained, permissively licensed
// library with few dependencies and decimal text in and out). float64 is used only in this package: the
// callers hand in and take back decimal text.
package bsm

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/strategy/risk/stress"
)

// Errors of the pricer.
var (
	ErrExpired      = errors.New("bsm: option expired or no time left")
	ErrBadInput     = errors.New("bsm: input is not a usable number")
	ErrBelowPayoff  = errors.New("bsm: price is at or below intrinsic value")
	ErrAboveBound   = errors.New("bsm: price is at or above the upper bound of the option")
	ErrNoConvergent = errors.New("bsm: volatility did not converge")
)

// Pricer implements stress.Pricer.
type Pricer struct{}

var _ stress.Pricer = Pricer{}

// New returns the pricer.
func New() Pricer { return Pricer{} }

type params struct {
	put                       bool
	spot, strike, years, rate float64
}

func parse(in stress.Input) (params, error) {
	p := params{}
	switch strings.ToUpper(strings.TrimSpace(in.Right)) {
	case "P", "PUT":
		p.put = true
	case "C", "CALL":
	default:
		return p, fmt.Errorf("%w: right %q", ErrBadInput, in.Right)
	}
	for _, f := range []struct {
		dst *float64
		src string
	}{{&p.spot, in.Spot}, {&p.strike, in.Strike}, {&p.years, in.Years}} {
		v, err := strconv.ParseFloat(f.src, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return p, fmt.Errorf("%w: %q", ErrBadInput, f.src)
		}
		*f.dst = v
	}
	if in.Rate != "" {
		v, err := strconv.ParseFloat(in.Rate, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return p, fmt.Errorf("%w: rate %q", ErrBadInput, in.Rate)
		}
		p.rate = v
	}
	if p.spot <= 0 || p.strike <= 0 {
		return p, fmt.Errorf("%w: spot and strike must be positive", ErrBadInput)
	}
	if p.years <= 0 {
		return p, ErrExpired
	}
	return p, nil
}

func cdf(x float64) float64 { return 0.5 * math.Erfc(-x/math.Sqrt2) }

func (p params) value(vol float64) float64 {
	sd := vol * math.Sqrt(p.years)
	disc := math.Exp(-p.rate * p.years)
	d1 := (math.Log(p.spot/p.strike) + (p.rate+vol*vol/2)*p.years) / sd
	d2 := d1 - sd
	if p.put {
		return p.strike*disc*cdf(-d2) - p.spot*cdf(-d1)
	}
	return p.spot*cdf(d1) - p.strike*disc*cdf(d2)
}

func (p params) intrinsic() float64 {
	if p.put {
		return math.Max(p.strike-p.spot, 0)
	}
	return math.Max(p.spot-p.strike, 0)
}

func text(v float64) string { return strconv.FormatFloat(v, 'f', 8, 64) }

// Value is the price of one unit of the option at in.Vol.
func (Pricer) Value(in stress.Input) (string, error) {
	p, err := parse(in)
	if err != nil {
		return "", err
	}
	vol, err := strconv.ParseFloat(in.Vol, 64)
	if err != nil || math.IsNaN(vol) || vol <= 0 || math.IsInf(vol, 0) {
		return "", fmt.Errorf("%w: vol %q", ErrBadInput, in.Vol)
	}
	return text(p.value(vol)), nil
}

// Implied is the volatility at which the option is worth price, found by bisection; a price at or below
// the intrinsic value is an error, as is an expired option.
func (Pricer) Implied(in stress.Input, price string) (string, error) {
	p, err := parse(in)
	if err != nil {
		return "", err
	}
	target, err := strconv.ParseFloat(price, 64)
	if err != nil || math.IsNaN(target) || math.IsInf(target, 0) {
		return "", fmt.Errorf("%w: price %q", ErrBadInput, price)
	}
	if target <= p.intrinsic() {
		return "", ErrBelowPayoff
	}
	lo, hi := 1e-6, 10.0
	if target <= p.value(lo) {
		return "", ErrBelowPayoff
	}
	if target >= p.value(hi) {
		return "", ErrAboveBound
	}
	for i := 0; i < 200; i++ {
		mid := (lo + hi) / 2
		if p.value(mid) < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return strconv.FormatFloat((lo+hi)/2, 'f', 8, 64), nil
}
