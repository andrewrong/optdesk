// Package recon is the pure compare of the version 2 pairs of the base data page and the verdict of
// a pair. It reads no clock, no network and no store.
// Design: docs/design/base-data-page.md#19.
package recon

import (
	"sort"
	"strings"

	"math/big"

	"github.com/andrewrong/optdesk/internal/compute/reconapi"
)

var _ reconapi.Func = Compare

// SourceLacks is the reason of a key only on our side when the other side is allowed to lack it.
const SourceLacks = "source_lacks"

func parse(s string) (*big.Rat, bool) {
	return new(big.Rat).SetString(strings.TrimSpace(s))
}

// places is the count of decimal places of a decimal text.
func places(s string) int {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

// within says whether |d| is inside the tolerance of the amounts a and b.
func within(d, a, b *big.Rat, tol reconapi.Tolerance) bool {
	d = new(big.Rat).Abs(d)
	if abs, ok := parse(tol.Abs); ok && d.Cmp(abs) <= 0 {
		return true
	}
	if rel, ok := parse(tol.Rel); ok {
		larger := new(big.Rat).Abs(a)
		if x := new(big.Rat).Abs(b); x.Cmp(larger) > 0 {
			larger = x
		}
		if d.Cmp(new(big.Rat).Mul(rel, larger)) <= 0 {
			return true
		}
	}
	return d.Sign() == 0
}

// Compare matches two keyed lists. A key on both sides is match inside the tolerance, else diff
// with ours minus other as decimal text; a key in Skip is skipped with its reason; a key only on
// our side is missing_other (skipped, reason source_lacks, when OtherMissingSkipped); a key only on
// the other side is missing_ours. Items are in key order and skipped is not counted in N. An
// amount that is not a decimal is a diff with no difference text. A key listed twice keeps its last
// amount.
func Compare(in reconapi.Input) reconapi.Output {
	ours := map[string]string{}
	other := map[string]string{}
	keys := map[string]bool{}
	for _, v := range in.Ours {
		ours[v.Key] = v.Amount
		keys[v.Key] = true
	}
	for _, v := range in.Other {
		other[v.Key] = v.Amount
		keys[v.Key] = true
	}
	for k := range in.Skip {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var out reconapi.Output
	for _, k := range sorted {
		o, hasO := ours[k]
		t, hasT := other[k]
		it := reconapi.Item{Key: k, Ours: o, Other: t}
		if reason, skip := in.Skip[k]; skip {
			it.Kind, it.Reason = reconapi.Skipped, reason
			out.Skipped++
			out.Items = append(out.Items, it)
			continue
		}
		switch {
		case hasO && hasT:
			a, okA := parse(o)
			b, okB := parse(t)
			if okA && okB {
				d := new(big.Rat).Sub(a, b)
				if within(d, a, b, in.Tol) {
					it.Kind = reconapi.Match
					out.Matched++
				} else {
					it.Kind = reconapi.Diff
					p := places(o)
					if q := places(t); q > p {
						p = q
					}
					it.Diff = d.FloatString(p)
					out.Diff++
				}
			} else {
				it.Kind = reconapi.Diff
				out.Diff++
			}
		case hasO:
			if in.OtherMissingSkipped {
				it.Kind, it.Reason = reconapi.Skipped, SourceLacks
				out.Skipped++
			} else {
				it.Kind = reconapi.MissingOther
				out.MissingOther++
			}
		default:
			it.Kind = reconapi.MissingOurs
			out.MissingOurs++
		}
		out.Items = append(out.Items, it)
	}
	out.N = out.Matched + out.Diff + out.MissingOurs + out.MissingOther
	return out
}
