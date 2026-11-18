package optionnotice

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// cashByDate sums strike x multiplier x contracts of the short puts per expiry day and
// currency, from the build day on; no conversion. When the TotalCashValue row has the currency of
// a row, Available is that cash and Left is Available minus the dues up to and including the row
// (negative is a shortfall); otherwise both are empty.
func cashByDate(cur []held, day int, dayOK bool, values []compute.NoticeValue) []compute.CashDay {
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
	var cash *big.Rat
	var cashCur, cashText string
	for _, v := range values {
		if v.Tag == "TotalCashValue" {
			if r, ok := parseDec(v.Value); ok {
				cash, cashCur, cashText = r, v.Currency, formatTrim(r)
			}
			break
		}
	}
	due := new(big.Rat)
	var out []compute.CashDay
	for _, k := range keys {
		row := compute.CashDay{Day: contracts.Day(dayText(k.day)), Currency: k.cur, Amount: formatTrim(sums[k])}
		if cash != nil && k.cur == cashCur {
			due.Add(due, sums[k])
			row.Available = cashText
			row.Left = formatTrim(new(big.Rat).Sub(cash, due))
		}
		out = append(out, row)
	}
	return out
}
