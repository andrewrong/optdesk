package replay

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// fates that have an actual row of their own; rolled lots have one row outside every total.
var actualFates = []string{"held", "assigned", "assigned_early", "closed_early"}

// rec is one lot with the figures the rows read; no field sums two currencies because rows are per currency.
type rec struct {
	lot              replayapi.Lot
	credit           *big.Rat
	hold, actual     *big.Rat
	holdITM          bool
	variantGross     *big.Rat // set only on a record taken from a priced variant outcome
	variantITM       bool
	variantClosedEar bool
}

// buildLines makes the rows of the report: per currency the hold row, an actual row per fate, the check row of the
// held lots and the rolled row; then per variant, in the order given, its row over the lots priced under it.
func buildLines(lots []replayapi.Lot, outs []replayapi.Outcome, variants []replayapi.Variant, minLots int) []replayapi.Line {
	byKey := map[string]replayapi.Outcome{}
	for _, o := range outs {
		byKey[o.LotID+"\x00"+o.Variant] = o
	}
	seen := map[string]bool{}
	var currencies []string
	for _, l := range lots {
		if !seen[l.Currency] {
			seen[l.Currency] = true
			currencies = append(currencies, l.Currency)
		}
	}
	sort.Strings(currencies)

	var lines []replayapi.Line
	perCur := map[string][]rec{} // lots hold prices, rolled and open ones left out
	for _, c := range currencies {
		var main, roll []rec
		for _, l := range lots {
			r, ok := recOf(l, byKey)
			switch {
			case l.Currency != c || !ok || l.Fate == "open":
			case l.Fate == "rolled":
				roll = append(roll, r)
			default:
				main = append(main, r)
			}
		}
		perCur[c] = main
		lines = append(lines, baseLines(c, main, roll, minLots)...)
	}
	for _, v := range variants {
		for _, c := range currencies {
			lines = append(lines, variantLine(v, c, perCur[c], byKey, minLots))
		}
	}
	return lines
}

// recOf is the record of a lot hold can price and whose actual is known.
func recOf(l replayapi.Lot, byKey map[string]replayapi.Outcome) (rec, bool) {
	h := byKey[l.ID+"\x00"+HoldVariant]
	credit, ok1 := ratOf(l.Credit)
	hold, ok2 := ratOf(h.Gross)
	actual, ok3 := ratOf(l.ActualGross)
	if !h.Priced || !ok1 || !ok2 || !ok3 {
		return rec{}, false
	}
	return rec{lot: l, credit: credit, hold: hold, actual: actual, holdITM: h.InTheMoney}, true
}

func baseLines(cur string, main, roll []rec, minLots int) []replayapi.Line {
	holdLine := mkLine("all", HoldVariant, "", cur, main, func(r rec) *big.Rat { return r.hold }, minLots)
	holdLine.Held = holdLine.Lots
	lines := []replayapi.Line{holdLine}
	var held []rec
	for _, f := range actualFates {
		var rs []rec
		for _, r := range main {
			if r.lot.Fate == f {
				rs = append(rs, r)
			}
		}
		if len(rs) == 0 {
			continue
		}
		if f == "held" {
			held = rs
		}
		ln := mkLine("all", "actual:"+f, "", cur, rs, func(r rec) *big.Rat { return r.actual }, minLots)
		fateCounts(&ln, f)
		lines = append(lines, ln)
	}
	if len(held) > 0 {
		lines = append(lines, checkLine(cur, held, minLots))
	}
	if len(roll) > 0 {
		ln := mkLine("rolled", "actual", "", cur, roll, func(r rec) *big.Rat { return r.actual }, minLots)
		ln.Rolled = ln.Lots
		lines = append(lines, ln)
	}
	return lines
}

func fateCounts(ln *replayapi.Line, fate string) {
	switch fate {
	case "held":
		ln.Held = ln.Lots
	case "closed_early":
		ln.ClosedEarly = ln.Lots
	default:
		ln.Assigned = ln.Lots
	}
}

// checkLine is the difference of hold and actual over the held lots: MeanGross is the mean of hold minus actual
// and WorstGross the largest absolute difference; both are zero when the two are priced on one basis.
func checkLine(cur string, held []rec, minLots int) replayapi.Line {
	ln := mkLine("all", "check:held", "", cur, held, func(r rec) *big.Rat { return r.hold }, minLots)
	ln.KeptShare, ln.HoldKeptShare, ln.ActualKeptShare = "", "", ""
	sum, worst := new(big.Rat), new(big.Rat)
	for _, r := range held {
		d := new(big.Rat).Sub(r.hold, r.actual)
		sum.Add(sum, d)
		if a := new(big.Rat).Abs(d); a.Cmp(worst) > 0 {
			worst = a
		}
	}
	ln.MeanGross = text(new(big.Rat).Quo(sum, big.NewRat(int64(len(held)), 1)))
	ln.WorstGross = text(worst)
	ln.Held = ln.Lots
	return ln
}

// variantLine is the row of one variant over the lots priced under it, with hold and actual over the same lots.
func variantLine(v replayapi.Variant, cur string, main []rec, byKey map[string]replayapi.Outcome, minLots int) replayapi.Line {
	var rs []rec
	for _, r := range main {
		o := byKey[r.lot.ID+"\x00"+v.Name]
		g, ok := ratOf(o.Gross)
		if !o.Priced || !ok {
			continue
		}
		r.variantGross, r.variantITM, r.variantClosedEar = g, o.InTheMoney, o.CloseReason != "expiry"
		rs = append(rs, r)
	}
	ln := mkLine("priced:"+v.Name, v.Name, v.Strategy.Params().Hash, cur, rs, func(r rec) *big.Rat { return r.variantGross }, minLots)
	ln.InTheMoney = 0
	for _, r := range rs {
		if r.variantClosedEar {
			ln.ClosedEarly++
		}
		if r.variantITM {
			ln.InTheMoney++
		}
	}
	ln.Held = ln.Lots - ln.ClosedEarly
	return ln
}

// mkLine fills the figures every row has. KeptShare is the sum of gross over the sum of credit.
func mkLine(set, variant, hash, cur string, rs []rec, gross func(rec) *big.Rat, minLots int) replayapi.Line {
	ln := replayapi.Line{Set: set, Variant: variant, ParamHash: hash, Currency: cur, Lots: len(rs), UnderMinimum: len(rs) < minLots}
	cycles := map[[2]string]bool{}
	sum, credit := new(big.Rat), new(big.Rat)
	var worst *big.Rat
	for _, r := range rs {
		cycles[[2]string{r.lot.Underlying, string(r.lot.Expiry)}] = true
		g := gross(r)
		sum.Add(sum, g)
		credit.Add(credit, r.credit)
		if r.holdITM {
			ln.InTheMoney++
		}
		if worst == nil || g.Cmp(worst) < 0 {
			worst = g
		}
	}
	ln.Cycles = len(cycles)
	if len(rs) == 0 {
		return ln
	}
	ln.MeanGross = text(new(big.Rat).Quo(sum, big.NewRat(int64(len(rs)), 1)))
	ln.WorstGross = text(worst)
	ln.KeptShare = share(rs, gross)
	ln.HoldKeptShare = share(rs, func(r rec) *big.Rat { return r.hold })
	ln.ActualKeptShare = share(rs, func(r rec) *big.Rat { return r.actual })
	return ln
}

func share(rs []rec, gross func(rec) *big.Rat) string {
	sum, credit := new(big.Rat), new(big.Rat)
	for _, r := range rs {
		sum.Add(sum, gross(r))
		credit.Add(credit, r.credit)
	}
	if credit.Sign() == 0 {
		return ""
	}
	return new(big.Rat).Quo(sum, credit).FloatString(4)
}
