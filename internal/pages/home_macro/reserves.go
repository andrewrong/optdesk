package home_macro

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/liquidity-view.md, Decision 4.

const (
	reservesID       = "WRESBAL"
	reservesLookback = 60 // days read before day: the 4-week change needs 28
	sayNoReserves    = "Reserves: no data yet."
	textNoReserves   = "Reserves: no data yet"
	sayReserves      = "Reserves %s, 4 weeks %s (%s)."
)

// reservesSay is "Reserves $3.0tn, 4 weeks −$40bn (2026-11-11)." from the Reserves and Reserves4w figures of
// c.Liquidity over the WRESBAL series alone; sayNoReserves with no Series or a figure with a Reason.
func reservesSay(ctx context.Context, c Config, d time.Time) (string, error) {
	if c.Series == nil {
		return sayNoReserves, nil
	}
	day := contracts.Day(d.Format(dayLayout))
	from := contracts.Day(d.AddDate(0, 0, -reservesLookback).Format(dayLayout))
	rows, err := c.Series.SeriesPoints(ctx, reservesID, from, day)
	if err != nil {
		return "", fmt.Errorf("home_macro: read %s: %w", reservesID, err)
	}
	res := c.Liquidity(liquidityapi.Input{Day: day, WRESBAL: seriesOf(rows, d), Params: c.LiquidityParams})
	if res.Reserves.Reason != "" || res.Reserves4w.Reason != "" {
		return sayNoReserves, nil
	}
	level, ok1 := new(big.Rat).SetString(res.Reserves.Value)
	delta, ok2 := new(big.Rat).SetString(res.Reserves4w.Value)
	if !ok1 || !ok2 || res.Reserves.Day == "" {
		return sayNoReserves, nil
	}
	return fmt.Sprintf(sayReserves, level2text(level), bnSigned(delta), res.Reserves.Day), nil
}

var (
	oneTn = big.NewRat(1_000_000_000_000, 1)
	oneBn = big.NewRat(1_000_000_000, 1)
)

// level2text is whole USD as $x.xtn from one trillion, else $xbn.
func level2text(v *big.Rat) string {
	if v.Cmp(oneTn) >= 0 {
		return "$" + roundHalfUp(new(big.Rat).Quo(v, oneTn), 1) + "tn"
	}
	return "$" + roundHalfUp(new(big.Rat).Quo(v, oneBn), 0) + "bn"
}

// bnSigned is a change in billions, signed with the minus sign: −$40bn, +$48bn, $0bn.
func bnSigned(v *big.Rat) string {
	n := roundHalfUp(new(big.Rat).Quo(v, oneBn), 0)
	switch signedText(n) {
	case n:
		return "$" + n + "bn"
	case "+" + n:
		return "+$" + n + "bn"
	}
	return minus + "$" + n[1:] + "bn"
}

// seriesOf keeps, per observation date, the row of the greatest RealtimeStart not after d, drops rows with no
// value or a status other than present, and returns the days ascending; nil when none is left.
func seriesOf(rows []contracts.SeriesPoint, d time.Time) compute.Series {
	kept := map[string]contracts.SeriesPoint{}
	for _, p := range rows {
		if p.RealtimeStart.After(d) || (p.Status != "" && p.Status != "present") {
			continue
		}
		k := p.ObservationDate.Format(dayLayout)
		if cur, ok := kept[k]; !ok || p.RealtimeStart.After(cur.RealtimeStart.Time) {
			kept[k] = p
		}
	}
	days := make([]string, 0, len(kept))
	for k, p := range kept {
		if p.Value != nil {
			days = append(days, k)
		}
	}
	slices.Sort(days)
	var out compute.Series
	for _, k := range days {
		out = append(out, compute.Point{Day: contracts.Day(k), Value: *kept[k].Value})
	}
	return out
}
