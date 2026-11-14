package pepb

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const maxHistory = 120

// day is one stored day with a ratio.
type day struct {
	d     time.Time
	ratio decimal.Decimal
}

func fmtDay(t time.Time) string { return t.Format("2006-01-02") }

func ordinal(n int) string {
	s := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			s = "st"
		case 2:
			s = "nd"
		case 3:
			s = "rd"
		}
	}
	return strconv.Itoa(n) + s
}

// rankPercent is the share of values strictly below x, times 100, rounded half up.
func rankPercent(stats compute.Stats, values []float64, x float64) (int, bool) {
	r, err := stats.PercentileRank(values, x, compute.Below)
	if err != nil {
		return 0, false
	}
	return int(math.Floor(r*100 + 0.5 + 1e-9)), true
}

// nearestRank is the value at rank p (percent) of the sorted values, nearest rank.
func nearestRank(sorted []decimal.Decimal, p int) decimal.Decimal {
	i := int(math.Ceil(float64(p)/100*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

// history picks every Nth day, counting back from the last, so at most 120 points result,
// the last is today and the order is oldest first.
func history(days []day) []pepbapi.Point {
	n := (len(days) + maxHistory - 1) / maxHistory
	if n < 1 {
		n = 1
	}
	var rev []pepbapi.Point
	for i := len(days) - 1; i >= 0; i -= n {
		t := days[i].ratio.StringFixed(1)
		v, _ := strconv.ParseFloat(t, 64)
		rev = append(rev, pepbapi.Point{Day: contracts.Day(fmtDay(days[i].d)), Text: t, Value: v})
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// fill sets the percentile, the price points, the window and the history of an OK reading.
// num is today's TTM (PE) or equity (PB) and shares today's count.
func fill(r *pepbapi.Reading, stats compute.Stats, days []day, num, shares decimal.Decimal) {
	today := days[len(days)-1]
	r.WindowDays = len(days)
	r.WindowFrom = contracts.Day(fmtDay(days[0].d))
	r.History = history(days)
	kind := upper(r.Kind)
	if len(days) < pepbapi.MinDays {
		r.PercentileReason = "too_few_days"
		return
	}
	vals := make([]float64, len(days))
	sorted := make([]decimal.Decimal, len(days))
	for i, d := range days {
		vals[i], _ = d.ratio.Float64()
		sorted[i] = d.ratio
	}
	x, _ := today.ratio.Float64()
	pct, ok := rankPercent(stats, vals, x)
	if !ok {
		r.PercentileReason = "too_few_days"
		return
	}
	r.Percentile = strconv.Itoa(pct)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LessThan(sorted[j]) })
	for _, p := range []int{20, 50} {
		ratio := nearestRank(sorted, p)
		price := ratio.Mul(num).Div(shares).Round(2)
		r.At = append(r.At, pepbapi.AtPercentile{
			Percentile: p,
			Ratio:      ratio.StringFixed(1),
			Price:      price.StringFixed(2),
			Text: fmt.Sprintf("%s at the %s percentile of %d days since %s: price %s (history, not a buy price)",
				kind, ordinal(p), r.WindowDays, r.WindowFrom, price.StringFixed(2)),
		})
	}
}

func upper(kind string) string {
	if kind == "pb" {
		return "PB"
	}
	return "PE"
}
