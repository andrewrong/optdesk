package optioncandidates

import (
	"fmt"
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/jobs"
)

// Needs returns the declarations of the [[need]] rows of the embedded job.toml, DTE from [params].
func Needs() ([]jobs.Need, error) {
	return needsOf(manifestBytes)
}

// Params returns the [params] of the embedded job.toml, as parseManifest reads them.
func Params() (compute.CandidateParams, error) {
	_, _, p, err := parseManifest(manifestBytes)
	return p, err
}

func needsOf(b []byte) ([]jobs.Need, error) {
	m, _, p, err := parseManifest(b)
	if err != nil {
		return nil, err
	}
	out := make([]jobs.Need, 0, len(m.Need))
	for i, r := range m.Need {
		if err := checkNeedRow(r); err != nil {
			return nil, fmt.Errorf("optioncandidates: manifest: need row %d: %w", i+1, err)
		}
		out = append(out, jobs.Need{
			Strategy: r.Strategy, Group: r.Group, Rights: append([]string(nil), r.Rights...),
			MinDTE: p.MinDTE, MaxDTE: p.MaxDTE, Ref: r.Ref, Low: r.Low, High: r.High, Expiry: r.Expiry,
		})
	}
	return out, nil
}

// checkNeedRow names the key that is wrong, never its value.
func checkNeedRow(r needRow) error {
	if r.Strategy == "" {
		return fmt.Errorf("strategy is empty")
	}
	if r.Group == "" {
		return fmt.Errorf("group is empty")
	}
	if r.Ref != "buy_price" && r.Ref != "spot" {
		return fmt.Errorf("ref must be buy_price or spot")
	}
	for _, x := range r.Rights {
		if x != "C" && x != "P" {
			return fmt.Errorf("rights must be C and P")
		}
	}
	if len(r.Rights) == 0 || len(r.Rights) > 2 || (len(r.Rights) == 2 && r.Rights[0] == r.Rights[1]) {
		return fmt.Errorf("rights must be C and P")
	}
	lo, ok1 := new(big.Rat).SetString(r.Low)
	hi, ok2 := new(big.Rat).SetString(r.High)
	if !ok1 || lo.Sign() <= 0 {
		return fmt.Errorf("low must be a positive decimal")
	}
	if !ok2 || hi.Sign() <= 0 {
		return fmt.Errorf("high must be a positive decimal")
	}
	if lo.Cmp(hi) >= 0 {
		return fmt.Errorf("low must be below high")
	}
	if r.Expiry != "all" && r.Expiry != "monthly" {
		return fmt.Errorf("expiry must be all or monthly")
	}
	return nil
}
