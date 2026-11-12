package optionopen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/atombender/go-jsonschema/pkg/types"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// stored inputs shared by the plan job and the evening jobs: the stocks, their close, sigma and held contracts.
// Nothing here calls the broker.

const (
	heldLookbackDays = 7
	closeLookback    = 10
	sigmaLookback    = 30
	priorSigmaDays   = 5
)

var heldAliases = []string{"A", "B"}

// stock is one underlying of the plan: the watch list and the held stocks.
type stock struct {
	Symbol, Exchange, Currency string
	ConID                      int64
}

// Stored is the set of readers of the stored inputs. Accounts and Eod may be nil: no held names, sigma by default.
type Stored struct {
	Watch       []jobs.WatchEntry
	Instruments store.InstrumentResolver
	Bars        bar.PriceBarReader
	Options     openquote.OptionContractStore
	Accounts    store.AccountReader
	Eod         store.OptionEodReader
}

// positions returns the newest rows of each alias, searching back from day.
func (s Stored) positions(ctx context.Context, day time.Time) ([]store.Position, error) {
	if s.Accounts == nil {
		return nil, nil
	}
	var out []store.Position
	for _, alias := range heldAliases {
		for back := 0; back <= heldLookbackDays; back++ {
			rows, err := s.Accounts.Positions(ctx, alias, day.AddDate(0, 0, -back).Format(time.DateOnly))
			if err != nil {
				return nil, err
			}
			if len(rows) > 0 {
				out = append(out, rows...)
				break
			}
		}
	}
	return out, nil
}

func nonzero(q string) bool {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(q))
	return ok && r.Sign() != 0
}

// stocks lists the US stock entries of the watch list and the held stocks that are not on it, by symbol.
func (s Stored) stocks(ctx context.Context, day time.Time) ([]stock, []store.Position, error) {
	pos, err := s.positions(ctx, day)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	var out []stock
	for _, e := range s.Watch {
		c := e.Contract
		if e.Market != "US" || c.SecType != "STK" || seen[c.Symbol] {
			continue
		}
		seen[c.Symbol] = true
		out = append(out, stock{Symbol: c.Symbol, Exchange: c.Exchange, Currency: c.Currency, ConID: c.ConID})
	}
	for _, p := range pos {
		if p.SecType != "stock" || p.ConID == 0 || seen[p.Symbol] || !nonzero(p.Quantity) {
			continue
		}
		seen[p.Symbol] = true
		out = append(out, stock{Symbol: p.Symbol, Exchange: p.Exchange, Currency: p.Currency, ConID: p.ConID})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, pos, nil
}

// underlyingID is the instrument id of the stock, created when absent as option_eod does.
func (s Stored) underlyingID(ctx context.Context, st stock) (int64, error) {
	spec := store.InstrumentSpec{Type: "stock", Name: st.Symbol, Currency: st.Currency, CalendarID: "XNYS",
		Status: "active", PriceIncrement: "0.01", LotSize: "1"}
	return s.Instruments.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(st.ConID, 10)}, spec, nil)
}

// lastClose is the close of the newest bar on or before through; ok is false when there is none.
func (s Stored) lastClose(ctx context.Context, id int64, through time.Time) (float64, bool, error) {
	bars, err := s.Bars.PriceBars(ctx, id, "trades",
		contracts.Day(through.AddDate(0, 0, -closeLookback).Format(time.DateOnly)), contracts.Day(through.Format(time.DateOnly)))
	if err != nil {
		return 0, false, err
	}
	for i := len(bars) - 1; i >= 0; i-- {
		if v, err := strconv.ParseFloat(bars[i].Close, 64); err == nil && v > 0 {
			return v, true, nil
		}
	}
	return 0, false, nil
}

// rv20Closes is the closes of a 20-day realised vol, and rv20Lookback the calendar days searched for them.
const (
	rv20Closes   = 21
	rv20Lookback = 45
)

// rv20 is the 20-day realised vol of the stored daily closes on or before through: the sample standard deviation
// of the 20 log returns of the last 21 closes, times sqrt(252). ok is false with fewer than 21 closes.
func (s Stored) rv20(ctx context.Context, id int64, through time.Time) (float64, bool, error) {
	bars, err := s.Bars.PriceBars(ctx, id, "trades",
		contracts.Day(through.AddDate(0, 0, -rv20Lookback).Format(time.DateOnly)), contracts.Day(through.Format(time.DateOnly)))
	if err != nil {
		return 0, false, err
	}
	var closes []float64
	for _, b := range bars {
		if v, err := strconv.ParseFloat(b.Close, 64); err == nil && v > 0 {
			closes = append(closes, v)
		}
	}
	if len(closes) < rv20Closes {
		return 0, false, nil
	}
	closes = closes[len(closes)-rv20Closes:]
	rets := make([]float64, 0, rv20Closes-1)
	mean := 0.0
	for i := 1; i < len(closes); i++ {
		r := math.Log(closes[i] / closes[i-1])
		rets = append(rets, r)
		mean += r
	}
	mean /= float64(len(rets))
	ss := 0.0
	for _, r := range rets {
		ss += (r - mean) * (r - mean)
	}
	return math.Sqrt(ss/float64(len(rets)-1)) * math.Sqrt(252), true, nil
}

func strikeValue(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// strikeKey is a strike as a canonical text, so that "150" and "150.0" are one strike.
func strikeKey(s string) string {
	if r, ok := new(big.Rat).SetString(strings.TrimSpace(s)); ok {
		return r.RatString()
	}
	return s
}

func dayTime(d contracts.Day) (time.Time, error) { return time.Parse(time.DateOnly, string(d)) }

// refKey is the identity of a contract of the stock: expiry, strike, right and trading class.
func refKey(expiry contracts.Day, strike, right, class string) string {
	return string(expiry) + "|" + strikeKey(strike) + "|" + right + "|" + class
}

func markKey(m contracts.OptionNotListed) string {
	return refKey(contracts.Day(m.Expiry.Format(time.DateOnly)), m.Strike, string(m.Right), m.TradingClass)
}

// sigmas returns the prior-close sigma and the last known sigma of a stock: the implied vol of the listed
// contract nearest the spot on the newest option_eod day, the prior close when that day is within five days.
func (s Stored) sigmas(ctx context.Context, id int64, refs []openquote.OptionRef, spot float64, day time.Time) (prior, last float64, err error) {
	if s.Eod == nil || len(refs) == 0 {
		return 0, 0, nil
	}
	rows, err := s.Eod.OptionEod(ctx, id, contracts.Day(day.AddDate(0, 0, -sigmaLookback).Format(time.DateOnly)),
		contracts.Day(day.AddDate(0, 0, -1).Format(time.DateOnly)))
	if err != nil {
		return 0, 0, err
	}
	strike := map[int64]float64{}
	for _, r := range refs {
		strike[r.InstrumentID] = strikeValue(r.Strike)
	}
	type best struct {
		dist, iv float64
	}
	byDay := map[string]best{}
	for _, r := range rows {
		k, ok := strike[int64(r.InstrumentID)]
		if !ok || r.IvDecimal == nil || *r.IvDecimal <= 0 {
			continue
		}
		d := r.SessionDay.Format(time.DateOnly)
		dist := k - spot
		if dist < 0 {
			dist = -dist
		}
		if b, has := byDay[d]; !has || dist < b.dist {
			byDay[d] = best{dist, *r.IvDecimal}
		}
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)
	if len(days) == 0 {
		return 0, 0, nil
	}
	newest := days[len(days)-1]
	if t, perr := time.Parse(time.DateOnly, newest); perr == nil && day.Sub(t) <= priorSigmaDays*24*time.Hour {
		prior = byDay[newest].iv
		days = days[:len(days)-1]
	}
	if len(days) > 0 {
		last = byDay[days[len(days)-1]].iv
	}
	return prior, last, nil
}

// heldRefs maps the open option positions of symbol to the Stored contracts; a position with no Stored con_id is
// not returned (the run reports it as unresolved).
func heldRefs(pos []store.Position, symbol string, refs []openquote.OptionRef) (held []openquote.OptionRef, missing int) {
	byKey := func(expiry, strike, right string) (openquote.OptionRef, bool) {
		for _, r := range refs {
			if string(r.Expiry) == expiry && strikeKey(r.Strike) == strikeKey(strike) && r.Right == right {
				return r, true
			}
		}
		return openquote.OptionRef{}, false
	}
	seen := map[int64]bool{}
	for _, p := range pos {
		if p.SecType != "option" || p.Symbol != symbol || !nonzero(p.Quantity) {
			continue
		}
		right := "P"
		if strings.EqualFold(p.Right, "call") || strings.EqualFold(p.Right, "c") {
			right = "C"
		}
		if r, ok := byKey(p.Expiry, p.Strike, right); ok {
			if !seen[r.InstrumentID] {
				seen[r.InstrumentID] = true
				held = append(held, r)
			}
		} else {
			missing++
		}
	}
	return held, missing
}

func hasPosition(pos []store.Position, symbol string) bool {
	for _, p := range pos {
		if p.Symbol == symbol && nonzero(p.Quantity) {
			return true
		}
	}
	return false
}

func sessionDate(d time.Time) types.SerializableDate { return types.SerializableDate{Time: d} }

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// spotOf is the prior close of the name from the stored bars; a missing bar is asked of the gateway once. A name
// with neither has no spot and the run ends Retryable.
func (r *listRun) spotOf(ctx context.Context, n *listName) (float64, bool, contracts.TypedError) {
	if n.spotSt != spotUnknown {
		return n.spot, n.spotSt == spotKnown, nil
	}
	v, ok, err := r.j.d.lastClose(ctx, n.id, r.t.AddDate(0, 0, -1))
	if err != nil {
		return 0, false, asTyped(err, contracts.Retryable)
	}
	if !ok {
		v, ok = r.historySpot(ctx, n)
	}
	if ok {
		n.spot, n.spotSt = v, spotKnown
	} else {
		n.spotSt, r.noSpot = spotNone, true
	}
	return n.spot, ok, nil
}

// historySpot asks the gateway for the last daily bar of the name, once.
func (r *listRun) historySpot(ctx context.Context, n *listName) (float64, bool) {
	h := r.j.d.History
	if h == nil {
		return 0, false
	}
	bars, err := h.Bars(ctx, broker.BarQuery{Contract: r.underlying(n), Kind: broker.BarTrades, DurationDays: 5,
		End: r.t.AddDate(0, 0, -1).Format(time.DateOnly)})
	if err != nil {
		return 0, false
	}
	for i := len(bars) - 1; i >= 0; i-- {
		if v, err := strconv.ParseFloat(bars[i].Close, 64); err == nil && v > 0 {
			return v, true
		}
	}
	return 0, false
}

// sigmaOf is the sigma of the band of the name: the larger of the stored ATM implied vol and the 20-day realised
// vol of the stored closes, else DefaultSigma with the source default.
func (r *listRun) sigmaOf(ctx context.Context, n *listName, spot float64) (float64, string, contracts.TypedError) {
	if n.sigmaSet {
		return n.sigma, n.sigSrc, nil
	}
	if terr := r.loadRefs(ctx, n); terr != nil {
		return 0, "", terr
	}
	prior, last, err := r.j.d.sigmas(ctx, n.id, n.refs, spot, r.t)
	if err != nil {
		return 0, "", asTyped(err, contracts.Retryable)
	}
	iv := prior
	if iv <= 0 {
		iv = last
	}
	rv, _, err := r.j.d.rv20(ctx, n.id, r.t.AddDate(0, 0, -1))
	if err != nil {
		return 0, "", asTyped(err, contracts.Retryable)
	}
	switch {
	case iv <= 0 && rv <= 0:
		n.sigma, n.sigSrc = DefaultSigma, "default"
	case iv >= rv:
		n.sigma, n.sigSrc = iv, "iv"
	default:
		n.sigma, n.sigSrc = rv, "rv20"
	}
	n.sigmaSet = true
	return n.sigma, n.sigSrc, nil
}
