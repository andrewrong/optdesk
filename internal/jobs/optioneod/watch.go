package optioneod

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// NewWithWatch builds the option_eod job from the embedded job.toml with the underlyings of the US stock
// entries of watch (market US, sec_type STK); the [[underlying]] tables of job.toml are not used. The DTE
// window and max_strikes of job.toml apply, and an entry with a BuyPrice gets the strike range of that price.
func NewWithWatch(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, watch []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
	return NewWithWatchSymbols(b, res, w, watch, clk, nil)
}

var _ jobs.NewOptionEodWatchSymbolsFunc = NewWithWatchSymbols

// NewWithWatchSymbols is NewWithWatch with a translator that gives the option root of each watched
// underlying; nil means no translation.
func NewWithWatchSymbols(
	b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, watch []jobs.WatchEntry, clk clock.Clock, t symbol.Translator,
) (contracts.Job, error) {
	if b == nil || res == nil || w == nil || clk == nil {
		return nil, errors.New("optioneod: broker, resolver, writer and clock are required")
	}
	j, err := build(manifestBytes, b, res, w, clk)
	if err != nil {
		return nil, err
	}
	j.sym = t
	us, buy, err := watchUnderlyings(watch)
	if err != nil {
		return nil, err
	}
	j.m.Underlying, j.m.buy = us, buy
	if err := j.m.validate(); err != nil {
		return nil, err
	}
	return j, nil
}

// NewWithWatchWindow behaves as NewWithWatch and then replaces min_dte, max_dte and max_strikes of the
// manifest with each non-zero field of win; a zero field keeps the manifest value. A window that breaks the
// manifest rule (min_dte above max_dte) gives the error the manifest validation gives.
func NewWithWatchWindow(
	b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, watch []jobs.WatchEntry,
	win jobs.OptionEodWindow, clk clock.Clock,
) (contracts.Job, error) {
	if b == nil || res == nil || w == nil || clk == nil {
		return nil, errors.New("optioneod: broker, resolver, writer and clock are required")
	}
	j, err := build(manifestBytes, b, res, w, clk)
	if err != nil {
		return nil, err
	}
	us, buy, err := watchUnderlyings(watch)
	if err != nil {
		return nil, err
	}
	j.m.Underlying, j.m.buy = us, buy
	if win.MinDTE != 0 {
		j.m.MinDTE = win.MinDTE
	}
	if win.MaxDTE != 0 {
		j.m.MaxDTE = win.MaxDTE
	}
	if win.MaxStrikes != 0 {
		j.m.MaxStrikes = win.MaxStrikes
	}
	if err := j.m.validate(); err != nil {
		return nil, err
	}
	return j, nil
}

// watchUnderlyings keeps the US stock entries, sorted by symbol, and the BuyPrice text of those that have one.
func watchUnderlyings(watch []jobs.WatchEntry) ([]underlying, map[string]string, error) {
	var out []underlying
	buy := map[string]string{}
	seen := map[string]bool{}
	for _, e := range watch {
		if e.Market != "US" || e.Contract.SecType != "STK" {
			continue
		}
		c := e.Contract
		if seen[c.Symbol] {
			return nil, nil, fmt.Errorf("optioneod: watch list holds %s twice", c.Symbol)
		}
		seen[c.Symbol] = true
		out = append(out, underlying{Symbol: c.Symbol, SecType: c.SecType, Exchange: c.Exchange, Currency: c.Currency,
			ConID: c.ConID, CalendarID: "XNYS", PriceIncrement: "0.01", LotSize: "1"})
		if e.BuyPrice != "" {
			if _, ok := new(big.Rat).SetString(e.BuyPrice); !ok {
				return nil, nil, fmt.Errorf("optioneod: buy price of %s is not a number", c.Symbol)
			}
			buy[c.Symbol] = e.BuyPrice
		}
	}
	if len(out) == 0 {
		return nil, nil, errors.New("optioneod: the watch list holds no US stock entry")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, buy, nil
}

// dteExpiries lists the expirations (YYYYMMDD) whose calendar days after day are in min..max, sorted.
func dteExpiries(all []string, day string, min, max int) []string {
	d, err := time.Parse("20060102", day)
	if err != nil {
		return nil
	}
	var keep []string
	for _, e := range all {
		t, err := time.Parse("20060102", e)
		if err != nil {
			continue
		}
		if dte := int(t.Sub(d).Hours() / 24); dte >= min && dte <= max {
			keep = append(keep, e)
		}
	}
	sort.Strings(keep)
	return keep
}

// rangeStrikes keeps the strikes from 0.80 x buy to max(1.05 x buy, price) and, when more than limit
// remain, the limit nearest buy (a tie keeps the lower strike). Arithmetic is exact.
func rangeStrikes(all []string, buy string, price float64, limit int) []strikeValue {
	b, ok := new(big.Rat).SetString(buy)
	if !ok {
		return nil
	}
	lo := new(big.Rat).Mul(b, big.NewRat(80, 100))
	hi := new(big.Rat).Mul(b, big.NewRat(105, 100))
	if p := new(big.Rat).SetFloat64(price); p != nil && p.Cmp(hi) > 0 {
		hi = p
	}
	type cand struct {
		sv   strikeValue
		dist *big.Rat
	}
	var in []cand
	for _, t := range all {
		r, ok := new(big.Rat).SetString(strings.TrimSpace(t))
		if !ok || r.Cmp(lo) < 0 || r.Cmp(hi) > 0 {
			continue
		}
		v, err := strconv.ParseFloat(t, 64)
		if err != nil {
			continue
		}
		in = append(in, cand{strikeValue{t, v}, new(big.Rat).Abs(new(big.Rat).Sub(r, b))})
	}
	sort.Slice(in, func(i, j int) bool {
		if c := in[i].dist.Cmp(in[j].dist); c != 0 {
			return c < 0
		}
		return in[i].sv.value < in[j].sv.value
	})
	if limit > 0 && len(in) > limit {
		in = in[:limit]
	}
	out := make([]strikeValue, len(in))
	for i, c := range in {
		out[i] = c.sv
	}
	sort.Slice(out, func(i, j int) bool { return out[i].value < out[j].value })
	return out
}

// jpWatchUnderlyings keeps the JP stock entries (market JP, sec_type STK) as stock underlyings with the values of
// the [stock] table, sorted by code, and the BuyPrice text of those that have one. No entry is not an error.
func jpWatchUnderlyings(watch []jobs.WatchEntry, st stockTable) ([]underlying, map[string]string, error) {
	var out []underlying
	buy := map[string]string{}
	seen := map[string]bool{}
	for _, e := range watch {
		if e.Market != "JP" || e.Contract.SecType != "STK" {
			continue
		}
		c := e.Contract
		if seen[c.Symbol] {
			return nil, nil, fmt.Errorf("optioneod: watch list holds %s twice", c.Symbol)
		}
		seen[c.Symbol] = true
		out = append(out, underlying{Symbol: c.Symbol, SecType: c.SecType, Exchange: c.Exchange, Currency: c.Currency,
			ConID: c.ConID, MIC: st.MIC, CalendarID: st.CalendarID, PriceIncrement: st.PriceIncrement, LotSize: st.LotSize})
		if e.BuyPrice != "" {
			if _, ok := new(big.Rat).SetString(e.BuyPrice); !ok {
				return nil, nil, fmt.Errorf("optioneod: buy price of %s is not a number", c.Symbol)
			}
			buy[c.Symbol] = e.BuyPrice
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, buy, nil
}
