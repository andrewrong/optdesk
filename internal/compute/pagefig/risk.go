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
	// why is a gap reason known for the whole line (unsupported_currency); it wins over the sum.
	why string
}

// bucketFor returns the bucket of key, made when absent.
func bucketFor(m map[string]*bucket, key string) *bucket {
	b := m[key]
	if b == nil {
		b = &bucket{sum: new(big.Rat)}
		m[key] = b
	}
	return b
}

// risk sums stock quantity and option delta by underlying, option vega by expiry. Mark is never
// read. A stock of any currency adds its share count (shares do not depend on the currency). A greek
// with Currency JPY has its vega divided by in.UsdJpy; without a positive rate its vega line and the
// vega total are the gap unsupported_currency and its delta is added unchanged. An unreadable number
// makes its line and the total missing_input.
func risk(in blockapi.RiskIn) blockapi.RiskResult {
	delta, vega := map[string]*bucket{}, map[string]*bucket{}
	add := func(m map[string]*bucket, key, text string) {
		b := bucketFor(m, key)
		if r, ok := parse(text); ok {
			b.sum.Add(b.sum, r)
		} else {
			b.bad = true
		}
	}
	rate, rateOK := parse(in.UsdJpy)
	rateOK = rateOK && rate.Sign() > 0
	for _, s := range in.Stocks {
		add(delta, s.Underlying, s.Quantity)
	}
	for _, g := range in.Greeks {
		add(delta, g.Underlying, g.Delta)
		if g.Currency != "JPY" {
			add(vega, string(g.Expiry), g.Vega)
			continue
		}
		if !rateOK {
			bucketFor(vega, string(g.Expiry)).why = "unsupported_currency"
			continue
		}
		v, ok := parse(g.Vega)
		if !ok {
			add(vega, string(g.Expiry), g.Vega)
			continue
		}
		add(vega, string(g.Expiry), new(big.Rat).Quo(v, rate).FloatString(12))
	}
	var out blockapi.RiskResult
	out.Delta, out.DeltaTotal = lines(delta, "")
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
		if b.why != "" {
			out = append(out, blockapi.RiskLine{Key: k, Value: gap(b.why)})
			if g == "" {
				g = b.why
			}
			continue
		}
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
