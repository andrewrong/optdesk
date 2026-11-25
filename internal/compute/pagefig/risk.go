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
	// cur is the currency of a Delta line ("" read as USD); mixed is set when two currencies met.
	cur   string
	seen  bool
	mixed bool
}

// setCur records the currency c of a part of the line.
func (b *bucket) setCur(c string) {
	if c == "" {
		c = "USD"
	}
	if b.seen && b.cur != c {
		b.mixed = true
	}
	b.cur, b.seen = c, true
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
// read. A Delta line has the currency of its name; the Delta total is one for each currency (USD, JPY). A greek
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
		bucketFor(delta, s.Underlying).setCur(s.Currency)
	}
	for _, g := range in.Greeks {
		add(delta, g.Underlying, g.Delta)
		bucketFor(delta, g.Underlying).setCur(g.Currency)
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
	out.Delta, out.DeltaTotals = deltaCurrencies(delta, out.Delta)
	out.Vega, out.VegaTotal = lines(vega, "")
	return out
}

// deltaCurrencies sets the currency of each Delta line (from the sorted buckets, in line order), turns an unsupported
// currency or a mixed name into its line gap, and sums one total for each of USD and JPY.
func deltaCurrencies(m map[string]*bucket, ls []blockapi.RiskLine) ([]blockapi.RiskLine, []blockapi.RiskTotal) {
	sums := map[string]*big.Rat{"USD": new(big.Rat), "JPY": new(big.Rat)}
	gaps := map[string]string{}
	counted := map[string]bool{}
	for i := range ls {
		b := m[ls[i].Key]
		switch {
		case b.mixed:
			ls[i].Value = gap("missing_input")
			continue
		case b.cur != "USD" && b.cur != "JPY":
			ls[i].Currency = b.cur
			ls[i].Value = gap("unsupported_currency")
			continue
		}
		ls[i].Currency = b.cur
		counted[b.cur] = true
		if ls[i].Value.Gap != "" {
			if gaps[b.cur] == "" {
				gaps[b.cur] = ls[i].Value.Gap
			}
			continue
		}
		sums[b.cur].Add(sums[b.cur], b.sum)
	}
	var totals []blockapi.RiskTotal
	for _, c := range []string{"USD", "JPY"} {
		if counted[c] {
			totals = append(totals, blockapi.RiskTotal{Currency: c, Value: total(sums[c], gaps[c])})
		}
	}
	return ls, totals
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
