package optioneod

import (
	"fmt"
	"math/big"

	"github.com/andrewrong/optdesk/internal/jobs"
)

// Covers is nil when the stock capture (the embedded job.toml) takes everything n declares; else an error that
// names the strategy and the dimension. Only the group watch:options is known: an index group is refused.
func Covers(n jobs.Need) error {
	m, err := parseManifest(manifestBytes)
	if err != nil {
		return err
	}
	refuse := func(dim string) error {
		return fmt.Errorf("optioneod: need of %q is outside the capture: %s", n.Strategy, dim)
	}
	if n.Group != "watch:options" {
		return refuse("group")
	}
	if len(n.Rights) == 0 {
		return refuse("rights")
	}
	for _, r := range n.Rights {
		if r != "C" && r != "P" {
			return refuse("rights")
		}
	}
	if n.MinDTE < m.MinDTE || n.MaxDTE > m.MaxDTE || n.MinDTE > n.MaxDTE {
		return refuse("dte")
	}
	if n.Expiry != "all" {
		return refuse("expiry kind")
	}
	if n.TradingClass != "" {
		return refuse("trading class")
	}
	return coversReach(n, m, refuse)
}

// coversReach compares the strike reach of n with the band the capture takes for its ref, exactly.
func coversReach(n jobs.Need, m manifest, refuse func(string) error) error {
	lo, ok1 := new(big.Rat).SetString(n.Low)
	hi, ok2 := new(big.Rat).SetString(n.High)
	if !ok1 || !ok2 {
		return refuse("reach")
	}
	var capLo, capHi *big.Rat
	switch n.Ref {
	case "buy_price":
		var ok bool
		if m.BuyLow == "" || m.BuyHigh == "" {
			return refuse("reach")
		}
		if capLo, ok = new(big.Rat).SetString(m.BuyLow); !ok {
			return refuse("reach")
		}
		if capHi, ok = new(big.Rat).SetString(m.BuyHigh); !ok {
			return refuse("reach")
		}
	case "spot":
		// The band keys are float64; their shortest decimal text is the value the owner wrote.
		below := new(big.Rat)
		above := new(big.Rat)
		below.SetString(fmt.Sprintf("%v", m.StrikeBandBelow))
		above.SetString(fmt.Sprintf("%v", m.StrikeBandAbove))
		capLo = new(big.Rat).Sub(big.NewRat(1, 1), below)
		capHi = new(big.Rat).Add(big.NewRat(1, 1), above)
	default:
		return refuse("ref")
	}
	if lo.Cmp(capLo) < 0 || hi.Cmp(capHi) > 0 || lo.Cmp(hi) >= 0 {
		return refuse("reach")
	}
	return nil
}
