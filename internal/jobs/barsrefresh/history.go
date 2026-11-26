package barsrefresh

import (
	"context"
	"math/big"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

// routeOf is the yfinance pair of a key, and false for a key with no route: an [[index]] key, a stock in a
// currency other than USD or JPY, or a job built without a yfinance source (WithFallback). A key with a
// route asks IB for no daily bar at all (docs/design/stock-history-yfinance.md, Decisions 1 and 6).
func (r *runState) routeOf(c broker.Contract) (sources.BarPair, bool) {
	if r.j.src == nil || r.j.cal == nil {
		return sources.BarPair{}, false
	}
	return pairOf(c)
}

// newestSourceDay is the latest session day of the rows of the given source, and false when none has it.
func newestSourceDay(rows []contracts.PriceBar, source string) (string, bool) {
	newest, found := "", false
	for _, row := range rows {
		if string(row.Source) != source {
			continue
		}
		if d := row.SessionDay.Format(dayLayout); !found || d > newest {
			newest, found = d, true
		}
	}
	return newest, found
}

// yfinanceKey is the history step of a key with a route: stored trades and adjusted_last rows, then one
// yfinance request. From the session day of the newest stored yfinance row, with actions: a dividend or
// split after it starts one full pull written by ReplacePriceBars; none writes the new days by
// UpsertPriceBars and asks nothing more. A failed or empty answer leaves every stored row as it is and
// ends Retryable, so the key is asked on the next run.
func (r *runState) yfinanceKey(
	ctx context.Context, key string, id int64, c broker.Contract, pair sources.BarPair, trades []contracts.PriceBar,
) (int, int, contracts.TypedError) {
	adj, err := r.j.r.PriceBars(ctx, id, basisAdjusted, firstDay, r.day)
	if err != nil {
		return 0, 0, asTyped(err, contracts.Retryable)
	}
	last, hasTrades := newestSourceDay(trades, sourceYfinance)
	_, hasAdj := newestSourceDay(adj, sourceYfinance)
	full := !hasTrades || !hasAdj
	from := firstDay
	if !full {
		from = contracts.Day(last)
	}
	res, terr := r.askYfinance(ctx, key, pair, from)
	if terr != nil {
		return 0, 0, terr
	}
	if !full {
		for _, a := range res.Actions {
			if a.Pair == pair && string(a.Day) > last && a.Day <= r.day {
				r.j.log.Info("bars_refresh yfinance event: full pull", "key", key, "kind", a.Kind, "day", string(a.Day))
				full = true
				break
			}
		}
		if full {
			if res, terr = r.askYfinance(ctx, key, pair, firstDay); terr != nil {
				return 0, 0, terr
			}
		}
	}
	rows, skipped, terr := r.yfinanceRows(ctx, key, id, c, pair, res)
	if terr != nil {
		return 0, 0, terr
	}
	if skipped > 0 {
		r.j.log.Warn("bars_refresh yfinance bars without a full price are left as a gap", "key", key, "bars", skipped)
	}
	if full {
		if rep := r.rep; rep == nil {
			return 0, 0, newErr(contracts.Retryable, "%s: the bar store cannot replace rows (%s is not set)", key, "PriceBarReplacer")
		} else if len(rows) > 0 {
			cnt, err := rep.ReplacePriceBars(ctx, rows, "bars_refresh:"+string(r.day))
			if err != nil {
				return 0, 0, asTyped(err, contracts.Retryable)
			}
			r.j.log.Info("bars_refresh yfinance full pull", "key", key, "rows", len(rows), "written", cnt.Written, "unchanged", cnt.Unchanged)
		}
	} else if terr := r.upsert(ctx, rows); terr != nil {
		return 0, 0, terr
	}
	return len(rows), len(rows), nil
}

// askYfinance makes one DailyBars call for the pair from the day from to the run day, with actions and the
// adjusted close. An error, an unanswered pair or an answer with no bar is a Retryable failure, logged with
// the key; after a rate_limited answer the later keys of the run are not asked.
func (r *runState) askYfinance(ctx context.Context, key string, pair sources.BarPair, from contracts.Day) (sources.BarResult, contracts.TypedError) {
	fail := func(code string, cause error) (sources.BarResult, contracts.TypedError) {
		r.j.log.Warn("bars_refresh yfinance not answered", "key", key, "code", code, "error", cause)
		return sources.BarResult{}, newErr(contracts.Retryable, "%s: yfinance history not answered (%s); stored rows are kept", key, code)
	}
	if r.yfDown {
		return fail(sources.BarCodeRateLimited, nil)
	}
	res, err := r.j.src.DailyBars(ctx, sources.BarRequest{
		Pairs: []sources.BarPair{pair}, From: from, To: r.day, Actions: true, Adjusted: true,
	})
	if err != nil {
		return fail(codeSourceFailed, err)
	}
	for _, u := range res.Unanswered {
		if u.Pair == pair {
			if u.Code == sources.BarCodeRateLimited {
				r.yfDown = true
			}
			return fail(u.Code, nil)
		}
	}
	if len(res.Bars) == 0 {
		return fail(sources.BarCodeNoData, nil)
	}
	return res, nil
}

// yfinanceRows maps the bars of the answer to a trades row and an adjusted_last row each. A bar without a
// positive open, high, low and close gives no trades row, and one without an adjusted close gives no
// adjusted row: a gap stays a gap. The bar of the run day is kept only when its session is over.
func (r *runState) yfinanceRows(
	ctx context.Context, key string, id int64, c broker.Contract, pair sources.BarPair, res sources.BarResult,
) (rows []contracts.PriceBar, skipped int, terr contracts.TypedError) {
	now := r.j.clk.Now().UTC()
	for _, b := range res.Bars {
		if b.Pair != pair || b.Day > r.day {
			continue
		}
		d, perr := time.Parse(dayLayout, string(b.Day))
		if perr != nil {
			return nil, 0, newErr(contracts.Permanent, "%s: bad bar day", key)
		}
		if b.Day == r.day {
			closed, cerr := r.sessionClosed(ctx, c.Currency, b.Day)
			if cerr != nil {
				return nil, 0, newErr(contracts.Retryable, "%s: session calendar: %v", key, cerr)
			}
			if !closed {
				continue
			}
		}
		base := contracts.PriceBar{
			Currency: c.Currency, InstrumentID: int(id), ReceivedAt: now,
			SessionDay: types.SerializableDate{Time: d}, Source: sourceYfinance,
		}
		if positive(b.Open) && positive(b.High) && positive(b.Low) && positive(b.Close) {
			t := base
			t.Basis, t.Open, t.High, t.Low, t.Close = basisTrades, b.Open, b.High, b.Low, b.Close
			if b.Volume != "" {
				v := b.Volume
				t.Volume = &v
			}
			rows = append(rows, t)
			if o, h, l, ok := adjustedOHL(b); ok {
				a := base
				a.Basis, a.Open, a.High, a.Low, a.Close = basisAdjusted, o, h, l, b.AdjClose
				rows = append(rows, a)
			}
		} else {
			skipped++
		}
	}
	return rows, skipped, nil
}

// adjustedOHL scales the trades open, high and low of a bar by AdjClose / Close, the adjustment factor of
// that bar, to 4 decimals. ok is false when the adjusted close is not positive or a result rounds to zero.
func adjustedOHL(b sources.RefBar) (o, h, l string, ok bool) {
	if !positive(b.AdjClose) {
		return "", "", "", false
	}
	adj, _ := parseRat(b.AdjClose)
	cl, _ := parseRat(b.Close)
	f := new(big.Rat).Quo(adj, cl)
	out := make([]string, 3)
	for i, v := range []string{b.Open, b.High, b.Low} {
		x, _ := parseRat(v)
		out[i] = new(big.Rat).Mul(x, f).FloatString(4)
		if !positive(out[i]) {
			return "", "", "", false
		}
	}
	return out[0], out[1], out[2], true
}

func positive(s string) bool {
	x, err := parseRat(s)
	return s != "" && err == nil && x.Sign() > 0
}
