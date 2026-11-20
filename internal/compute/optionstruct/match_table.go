// Algorithm and definition table after QuantConnect Lean, OptionStrategyMatcher, Apache-2.0, https://github.com/QuantConnect/Lean

package optionstruct

// legDef is one leg of a definition: a right, a sign (+1 bought, -1 sold), a lot count and a
// predicate on the contracts chosen before it and the candidate (strike and expiry).
type legDef struct {
	right string
	sign  int
	lots  int
	ok    func(chosen []*contract, c *contract) bool
}

// definition is one entry of the table. covered says the first leg is a sold call whose
// lots times multiplier must also be held as shares (the collar).
type definition struct {
	kind    string
	legs    []legDef
	covered bool
}

// Predicates compare a candidate with the leg at index i of the legs chosen so far.
func always(_ []*contract, _ *contract) bool { return true }

func atExpiry(i int) func([]*contract, *contract) bool {
	return func(ch []*contract, c *contract) bool { return c.expiry == ch[i].expiry }
}

func notExpiry(i int) func([]*contract, *contract) bool {
	return func(ch []*contract, c *contract) bool { return c.expiry != ch[i].expiry }
}

func strikeAbove(i int) func([]*contract, *contract) bool {
	return func(ch []*contract, c *contract) bool { return c.strike.Cmp(ch[i].strike) > 0 }
}

func strikeBelow(i int) func([]*contract, *contract) bool {
	return func(ch []*contract, c *contract) bool { return c.strike.Cmp(ch[i].strike) < 0 }
}

func strikeEqual(i int) func([]*contract, *contract) bool {
	return func(ch []*contract, c *contract) bool { return c.strike.Cmp(ch[i].strike) == 0 }
}

func strikeDiffers(i int) func([]*contract, *contract) bool {
	return func(ch []*contract, c *contract) bool { return c.strike.Cmp(ch[i].strike) != 0 }
}

// both joins predicates.
func both(ps ...func([]*contract, *contract) bool) func([]*contract, *contract) bool {
	return func(ch []*contract, c *contract) bool {
		for _, p := range ps {
			if !p(ch, c) {
				return false
			}
		}
		return true
	}
}

func put(sign int, ok func([]*contract, *contract) bool) legDef {
	return legDef{right: "P", sign: sign, lots: 1, ok: ok}
}

func call(sign int, ok func([]*contract, *contract) bool) legDef {
	return legDef{right: "C", sign: sign, lots: 1, ok: ok}
}

// condor builds an iron condor; sign +1 is the long-wings form, -1 the reverse.
func condor(kind string, s int) definition {
	return definition{kind: kind, legs: []legDef{
		put(s, always),
		put(-s, both(atExpiry(0), strikeAbove(0))),
		call(-s, both(atExpiry(0), strikeAbove(1))),
		call(s, both(atExpiry(0), strikeAbove(2))),
	}}
}

// pair builds a two-leg definition of the two rights and signs with one predicate on the second.
func pair(kind string, a, b legDef) definition { return definition{kind: kind, legs: []legDef{a, b}} }

// definitions is the table of Decisions 3 and 4, in the order they are tried: four legs, the
// collar (shares count as a leg), then the two-leg entries.
var definitions = []definition{
	condor(kindIronCondor, 1),
	condor(kindReverseCondor, -1),
	{kind: kindCollar, covered: true, legs: []legDef{
		call(-1, always), put(1, both(atExpiry(0), strikeBelow(0))),
	}},
	pair(kindPutSpread, put(-1, always), put(1, both(atExpiry(0), strikeBelow(0)))),
	pair(kindCallSpread, call(-1, always), call(1, both(atExpiry(0), strikeAbove(0)))),
	pair(kindPutDebit, put(-1, always), put(1, both(atExpiry(0), strikeAbove(0)))),
	pair(kindCallDebit, call(-1, always), call(1, both(atExpiry(0), strikeBelow(0)))),
	pair(kindLongStraddle, put(1, always), call(1, both(atExpiry(0), strikeEqual(0)))),
	pair(kindShortStraddle, put(-1, always), call(-1, both(atExpiry(0), strikeEqual(0)))),
	pair(kindLongStrangle, put(1, always), call(1, both(atExpiry(0), strikeAbove(0)))),
	pair(kindShortStrangle, put(-1, always), call(-1, both(atExpiry(0), strikeAbove(0)))),
	pair(kindCalendar, put(-1, always), put(1, both(notExpiry(0), strikeEqual(0)))),
	pair(kindCalendar, call(-1, always), call(1, both(notExpiry(0), strikeEqual(0)))),
	pair(kindDiagonal, put(-1, always), put(1, both(notExpiry(0), strikeDiffers(0)))),
	pair(kindDiagonal, call(-1, always), call(1, both(notExpiry(0), strikeDiffers(0)))),
}
