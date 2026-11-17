package valuelines

import (
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

// ProposeR returns the R values of the bucket of a 10-year yield. A bucket ends at its
// YieldUpTo, inclusive, so a yield on an edge takes the lower bucket; "" is the top bucket.
func (Calc) ProposeR(rows []valuelinesapi.RRow, yield string) (valuelinesapi.RValues, error) {
	y, err := parse(yield)
	if err != nil {
		return valuelinesapi.RValues{}, err
	}
	type bucket struct {
		row valuelinesapi.RRow
		up  decimal.Decimal
		top bool
	}
	bs := make([]bucket, 0, len(rows))
	for _, r := range rows {
		if r.YieldUpTo == "" {
			bs = append(bs, bucket{row: r, top: true})
			continue
		}
		up, err := parse(r.YieldUpTo)
		if err != nil {
			return valuelinesapi.RValues{}, err
		}
		bs = append(bs, bucket{row: r, up: up})
	}
	sort.SliceStable(bs, func(i, j int) bool {
		if bs[i].top != bs[j].top {
			return bs[j].top
		}
		return bs[i].up.LessThan(bs[j].up)
	})
	for _, b := range bs {
		if b.top || y.LessThanOrEqual(b.up) {
			return valuelinesapi.RValues{US: b.row.US, JP: b.row.JP, HKCN: b.row.HKCN}, nil
		}
	}
	return valuelinesapi.RValues{}, fmt.Errorf("%w: no bucket for yield %s", valuelinesapi.ErrBadInput, yield)
}

// RStepWarnings returns one text per market whose R moves by more than RStepWarn. A market
// whose R is not a number on either side gives no text.
func (Calc) RStepWarnings(previous, next valuelinesapi.RValues) []string {
	limit := decimal.RequireFromString(valuelinesapi.RStepWarn)
	var out []string
	for _, m := range []struct{ name, prev, next string }{
		{valuelinesapi.MarketUS, previous.US, next.US},
		{valuelinesapi.MarketJP, previous.JP, next.JP},
		{valuelinesapi.MarketHKCN, previous.HKCN, next.HKCN},
	} {
		p, err1 := decimal.NewFromString(m.prev)
		n, err2 := decimal.NewFromString(m.next)
		if err1 != nil || err2 != nil {
			continue
		}
		if n.Sub(p).Abs().GreaterThan(limit) {
			out = append(out, fmt.Sprintf("%s R moves from %s to %s, more than %s", m.name, p, n, limit))
		}
	}
	return out
}

// RoundText rounds a decimal text half away from zero to places digits (StringFixed does
// the rounding). A text that is not a number comes back unchanged.
func (Calc) RoundText(value string, places int) string {
	d, err := decimal.NewFromString(value)
	if err != nil {
		return value
	}
	if places < 0 {
		places = 0
	}
	return d.StringFixed(int32(places))
}
