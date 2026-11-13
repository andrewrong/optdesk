package pagefig

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func (holdingFigures) Holdings(in blockapi.HoldingsIn) blockapi.HoldingsResult { return holdings(in) }

// holdings evaluates every row alone, then sums the rows that have a value. A row left out of a
// total (no mark, unreadable input, another currency) puts its gap on that total, so a total is
// never shown as complete when it is not. Percent shares use the rounding of money.
func holdings(in blockapi.HoldingsIn) blockapi.HoldingsResult {
	net, netOK := parse(in.NetLiquidation)
	netOK = netOK && net.Sign() > 0
	out := blockapi.HoldingsResult{Rows: make([]blockapi.HoldingLine, 0, len(in.Rows))}
	value, unreal := new(big.Rat), new(big.Rat)
	var valueGap, unrealGap string
	for _, r := range in.Rows {
		l := blockapi.HoldingLine{Underlying: r.Underlying, SecType: r.SecType, Currency: r.Currency}
		v, u, g := holdingRow(r)
		switch {
		case g != "":
			l.Value, l.Unrealised, l.Share = gap(g), gap(g), gap(g)
			valueGap, unrealGap = firstGap(valueGap, g), firstGap(unrealGap, g)
		case r.Currency == "JPY" && in.Currency == "USD" && rate(in.UsdJpy) != nil:
			rt := rate(in.UsdJpy)
			v, u = new(big.Rat).Quo(v, rt), new(big.Rat).Quo(u, rt)
			l.Currency = "USD"
			l.Value, l.Unrealised = compute.FigureValue{Value: money(v)}, compute.FigureValue{Value: money(u)}
			l.Share = share(v, net, netOK)
			value.Add(value, v)
			unreal.Add(unreal, u)
		case r.Currency != in.Currency:
			// no rate, or another pair: the row keeps its own currency and amounts, never summed.
			l.Value, l.Unrealised = compute.FigureValue{Value: money(v)}, compute.FigureValue{Value: money(u)}
			l.Share = gap("unsupported_currency")
			valueGap, unrealGap = firstGap(valueGap, "unsupported_currency"), firstGap(unrealGap, "unsupported_currency")
		default:
			l.Value, l.Unrealised = compute.FigureValue{Value: money(v)}, compute.FigureValue{Value: money(u)}
			l.Share = share(v, net, netOK)
			value.Add(value, v)
			unreal.Add(unreal, u)
		}
		out.Rows = append(out.Rows, l)
	}
	out.TotalValue = total(value, valueGap)
	out.TotalUnrealised = total(unreal, unrealGap)
	return out
}

// rate reads a USD/JPY rate; nil unless it is a positive number.
func rate(s string) *big.Rat {
	r, ok := parse(s)
	if !ok || r.Sign() <= 0 {
		return nil
	}
	return r
}

// share is v in percent of net, or the gap no_data without a usable net.
func share(v, net *big.Rat, netOK bool) compute.FigureValue {
	if !netOK {
		return gap("no_data")
	}
	pct := new(big.Rat).Quo(new(big.Rat).Mul(v, big.NewRat(100, 1)), net)
	return compute.FigureValue{Value: money(pct)}
}

// holdingRow returns value and unrealised of one row, or the gap that stops it: missing_quote for
// an empty mark, missing_input for any other unreadable number.
func holdingRow(r blockapi.HoldingIn) (value, unrealised *big.Rat, g string) {
	if _, ok := parse(r.Mark); !ok && r.Mark == "" {
		return nil, nil, "missing_quote"
	}
	q, ok1 := parse(r.Quantity)
	m, ok2 := parse(r.Multiplier)
	c, ok3 := parse(r.Cost)
	mark, ok4 := parse(r.Mark)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil, nil, "missing_input"
	}
	qm := new(big.Rat).Mul(q, m)
	value = new(big.Rat).Mul(qm, mark)
	unrealised = new(big.Rat).Sub(value, new(big.Rat).Mul(qm, c))
	return value, unrealised, ""
}

func firstGap(have, g string) string {
	if have == "" || (g == "missing_quote" && have != "missing_quote") {
		return g
	}
	return have
}

func total(sum *big.Rat, g string) compute.FigureValue {
	if g != "" {
		return gap(g)
	}
	return compute.FigureValue{Value: money(sum)}
}
