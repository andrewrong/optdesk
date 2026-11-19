package recon

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
)

var _ reconapi.VerdictFunc = Verdict

func ratOf(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return new(big.Rat)
	}
	return r
}

// Verdict is the level of a pair from the counts of a run. R6: N of 0 is unknown; the share of Diff
// plus MissingOurs over N is warn above WarnShare and fail above FailShare, else pass; fewer than
// MinComparedShare of pairsAsked compared makes a pass a warn (at best warn). R1 to R5: any Diff or
// missing count above 0 is fail, else pass. Any other pair id is unknown: its verdict is not cut yet.
func Verdict(pair string, o reconapi.Output, pairsAsked, pairsCompared int) basecheckapi.Level {
	r, ok := RuleOf(pair)
	if !ok {
		return basecheckapi.Unknown
	}
	switch r.ID {
	case "R1", "R2", "R3", "R4", "R5":
		if o.Diff+o.MissingOurs+o.MissingOther > 0 {
			return basecheckapi.Fail
		}
		return basecheckapi.Pass
	case "R6":
		if o.N <= 0 {
			return basecheckapi.Unknown
		}
		share := big.NewRat(int64(o.Diff+o.MissingOurs), int64(o.N))
		level := basecheckapi.Pass
		switch {
		case share.Cmp(ratOf(r.FailShare)) > 0:
			level = basecheckapi.Fail
		case share.Cmp(ratOf(r.WarnShare)) > 0:
			level = basecheckapi.Warn
		}
		if level == basecheckapi.Pass && pairsAsked > 0 &&
			big.NewRat(int64(pairsCompared), int64(pairsAsked)).Cmp(ratOf(r.MinComparedShare)) < 0 {
			level = basecheckapi.Warn
		}
		return level
	}
	return basecheckapi.Unknown
}
