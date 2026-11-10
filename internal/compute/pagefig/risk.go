package pagefig

import (
	"math/big"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func (holdingFigures) Risk(in blockapi.RiskIn) blockapi.RiskResult { return risk(in) }

// bucket sums one key; bad is set when a part was unreadable.
type bucket struct {
	sum *big.Rat
	bad bool
}

// risk sums stock quantity and option delta by underlying, option vega by expiry. Mark is never
// read. A stock in another currency than in.Currency is skipped and the delta total is the gap
// unsupported_currency; an unreadable number makes its line and the total missing_input.
func risk(in blockapi.RiskIn) blockapi.RiskResult {
	delta, vega := map[string]*bucket{}, map[string]*bucket{}
	add := func(m map[string]*bucket, key, text string) {
		b := m[key]
		if b == nil {
			b = &bucket{sum: new(big.Rat)}
			m[key] = b
		}
		if r, ok := parse(text); ok {
			b.sum.Add(b.sum, r)
		} else {
			b.bad = true
		}
	}
	foreign := false
	for _, s := range in.Stocks {
		if s.Currency != in.Currency {
			foreign = true
			continue
		}
		add(delta, s.Underlying, s.Quantity)
	}
	for _, g := range in.Greeks {
		add(delta, g.Underlying, g.Delta)
		add(vega, string(g.Expiry), g.Vega)
	}
	var out blockapi.RiskResult
	var gd string
	if foreign {
		gd = "unsupported_currency"
	}
	out.Delta, out.DeltaTotal = lines(delta, gd)
	out.Vega, out.VegaTotal = lines(vega, "")
	return out
}

// lines turns buckets into lines sorted by key and a total; g is a gap already known for it.
func lines(m map[string]*bucket, g string) ([]blockapi.RiskLine, compute.FigureValue) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, strings.Compare)
	var out []blockapi.RiskLine
	sum := new(big.Rat)
	for _, k := range keys {
		b := m[k]
		if b.bad {
			out = append(out, blockapi.RiskLine{Key: k, Value: gap("missing_input")})
			if g == "" {
				g = "missing_input"
			}
			continue
		}
		out = append(out, blockapi.RiskLine{Key: k, Value: total(b.sum, "")})
		sum.Add(sum, b.sum)
	}
	return out, total(sum, g)
}
