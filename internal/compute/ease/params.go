package ease

import "math/big"

// input is one of the four inputs: the figure that scores 0 and the one that scores 100
// (the sub-score is linear between them), and how old its data day may be. weekdays says
// the age counts Monday to Friday days, without a holiday calendar; otherwise it counts
// calendar days.
type input struct {
	at0, at100 *big.Rat
	maxAge     int
	weekdays   bool
}

// paramSet holds every number of the score that is not in rules.json. The fixed points, the
// ages and the band edges live in the body of params and not in a data file, so the rule
// hash covers them.
type paramSet struct {
	// inputs are net_liquidity, us10y, fed_funds and hy_oas, in the order of the parts.
	inputs [4]input
	// usLookback is the number of observations back of the us10y change.
	usLookback int
	// recentWeeks is the number of weeks summed, ending at a Wednesday; backWeeks is how
	// many weeks before that the same count of weeks is summed for the divisor.
	recentWeeks, backWeeks int
	// basisPoints turns a change in percent into basis points.
	basisPoints *big.Rat
	// tighterTo and steadyTo are the last whole numbers of the bands tighter and steady.
	tighterTo, steadyTo int
}

// params gives the fixed numbers of ease version 2.
func params() paramSet {
	return paramSet{
		inputs: [4]input{
			{at0: big.NewRat(-3, 100), at100: big.NewRat(3, 100), maxAge: 10},
			{at0: big.NewRat(75, 1), at100: big.NewRat(-75, 1), maxAge: 3, weekdays: true},
			{at0: big.NewRat(50, 1), at100: big.NewRat(-50, 1), maxAge: 7},
			{at0: big.NewRat(11, 2), at100: big.NewRat(5, 2), maxAge: 3, weekdays: true},
		},
		usLookback:  63,
		recentWeeks: 4,
		backWeeks:   13,
		basisPoints: big.NewRat(100, 1),
		tighterTo:   33,
		steadyTo:    66,
	}
}
