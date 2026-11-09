package optionnotice

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// cashByDate sums strike x multiplier x contracts of the short puts per expiry day and
// currency, from the build day on; no netting against cash and no conversion.
func cashByDate(cur []held, day int, dayOK bool) []compute.CashDay {
	type key struct {
		day int
		cur string
	}
	sums := map[key]*big.Rat{}
	for _, h := range cur {
		if !h.short() || h.right != "P" || (dayOK && h.expDays < day) {
			continue
		}
		k := key{h.expDays, h.currency}
		if sums[k] == nil {
			sums[k] = new(big.Rat)
		}
		sums[k].Add(sums[k], h.exposure())
	}
	keys := make([]key, 0, len(sums))
	for k := range sums {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].day != keys[j].day {
			return keys[i].day < keys[j].day
		}
		return keys[i].cur < keys[j].cur
	})
	var out []compute.CashDay
	for _, k := range keys {
		out = append(out, compute.CashDay{Day: contracts.Day(dayText(k.day)), Currency: k.cur, Amount: formatTrim(sums[k])})
	}
	return out
}
