package computefigures

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	calendarID   = "XNYS"
	rateLookback = 30
	statusOK     = 1
	settlementAM = "AM"
)

// The cases of an input that is missing as data, not as a failure: the job stores a missing figure for them.
var (
	errNoRate    = errors.New("computefigures: no rate point")
	errStaleRate = errors.New("computefigures: the rate point is older than the rule allows")
	errNoSession = errors.New("computefigures: the as-of day has no session row")
)

// inputs is what the mapping reads: the stores, the series and unit helpers, and the rate series with its age bound.
type inputs struct {
	ComputeStores
	ops        compute.SeriesOps
	units      compute.Units
	rateSeries string
	rateMaxAge int
}

type expiryKey struct {
	day contracts.Day
	am  bool
}

// ivInput maps the stored rows of one underlying on day to the arguments of Iv30. The rate is
// returned with errStaleRate when it is older than rateMaxAge calendar days; errNoRate when there is none.
func ivInput(ctx context.Context, s inputs, underlyingID int64, day contracts.Day) ([]compute.ExpiryQuotes, compute.Point, error) {
	chain, rate, _, err := ivInputCounted(ctx, s, underlyingID, day)
	return chain, rate, err
}

// ivInputCounted is ivInput with the count of rows dropped because they have no current terms version.
func ivInputCounted(ctx context.Context, s inputs, underlyingID int64, day contracts.Day) (chain []compute.ExpiryQuotes,
	rate compute.Point, noTerms int, err error) {
	rate, err = rateOf(ctx, s, day)
	if err != nil {
		return nil, rate, 0, err
	}
	rows, err := s.OptionEod.OptionEod(ctx, underlyingID, day, day)
	if err != nil {
		return nil, rate, 0, fmt.Errorf("computefigures: option_eod: %w", err)
	}
	latest := map[int64]contracts.OptionEod{}
	for _, r := range rows {
		if r.Status != statusOK {
			continue
		}
		id := int64(r.InstrumentID)
		if old, ok := latest[id]; !ok || r.ReceivedAt.After(old.ReceivedAt) {
			latest[id] = r
		}
	}
	ids := make([]int64, 0, len(latest))
	for id := range latest {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	terms := map[int64]termsRow{}
	for _, c := range chunks(ids) {
		got, err := s.Terms.OptionTerms(ctx, c)
		if err != nil {
			return nil, rate, 0, fmt.Errorf("computefigures: terms: %w", err)
		}
		for _, t := range got {
			strike, perr := strconv.ParseFloat(t.Strike, 64)
			if perr != nil || (t.Right != "C" && t.Right != "P") {
				continue
			}
			terms[t.InstrumentID] = termsRow{right: t.Right, strike: strike, expiry: t.ExpirationDate,
				am: strings.EqualFold(t.SettlementType, settlementAM)}
		}
	}
	groups := map[expiryKey]map[float64]*compute.StrikeQuote{}
	for _, id := range ids {
		t, ok := terms[id]
		if !ok {
			noTerms++ // no current terms version: dropped and counted
			continue
		}
		k := expiryKey{day: t.expiry, am: t.am}
		if groups[k] == nil {
			groups[k] = map[float64]*compute.StrikeQuote{}
		}
		q := groups[k][t.strike]
		if q == nil {
			q = &compute.StrikeQuote{Strike: t.strike}
			groups[k][t.strike] = q
		}
		applyRow(q, t.right, latest[id])
	}
	chain, err = toChain(ctx, s, day, groups)
	return chain, rate, noTerms, err
}

type termsRow struct {
	right  string
	strike float64
	expiry contracts.Day
	am     bool
}

// applyRow sets one side of a strike: the mid when the quote allows it, the IV when above 0.
func applyRow(q *compute.StrikeQuote, right string, r contracts.OptionEod) {
	mid, hasMid := midOf(r.Bid, r.Ask)
	iv := 0.0
	if r.IvDecimal != nil && *r.IvDecimal > 0 {
		iv = *r.IvDecimal
	}
	if right == "C" {
		q.CallIV, q.CallMid, q.HasCall = iv, mid, hasMid
		return
	}
	q.PutIV, q.PutMid, q.HasPut = iv, mid, hasMid
}

// midOf is (bid+ask)/2 when both sides are present and above zero and the quote is not crossed.
func midOf(bid, ask *float64) (float64, bool) {
	if bid == nil || ask == nil || !(*bid > 0) || !(*ask > 0) || *ask < *bid {
		return 0, false
	}
	return (*bid + *ask) / 2, true
}

// toChain turns the grouped strikes into expiries with minutes from the as-of close to the expiry
// instant; an expiry on a non-session day, or not after the close, is dropped.
func toChain(ctx context.Context, s inputs, day contracts.Day, groups map[expiryKey]map[float64]*compute.StrikeQuote) ([]compute.ExpiryQuotes, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	today, err := s.SessionDays.SessionDays(ctx, calendarID, day, day)
	if err != nil {
		return nil, fmt.Errorf("computefigures: session days: %w", err)
	}
	var closeAt time.Time
	found := false
	for _, d := range today {
		if d.IsSession && dayOf(d.SessionDay) == day {
			closeAt, found = d.CloseAt, true
		}
	}
	if !found {
		return nil, errNoSession
	}
	lo, hi := contracts.Day(""), contracts.Day("")
	for k := range groups {
		if lo == "" || k.day < lo {
			lo = k.day
		}
		if k.day > hi {
			hi = k.day
		}
	}
	span, err := s.SessionDays.SessionDays(ctx, calendarID, lo, hi)
	if err != nil {
		return nil, fmt.Errorf("computefigures: session days: %w", err)
	}
	sessions := map[contracts.Day]contracts.SessionDay{}
	for _, d := range span {
		if d.IsSession {
			sessions[dayOf(d.SessionDay)] = d
		}
	}
	var chain []compute.ExpiryQuotes
	for k, strikes := range groups {
		sd, ok := sessions[k.day]
		if !ok {
			continue
		}
		at := sd.CloseAt
		if k.am {
			at = sd.OpenAt
		}
		minutes := int(at.Sub(closeAt) / time.Minute)
		if minutes <= 0 {
			continue
		}
		e := compute.ExpiryQuotes{MinutesToExpiry: minutes}
		for _, q := range strikes {
			e.Strikes = append(e.Strikes, *q)
		}
		sort.Slice(e.Strikes, func(i, j int) bool { return e.Strikes[i].Strike < e.Strikes[j].Strike })
		chain = append(chain, e)
	}
	sort.Slice(chain, func(i, j int) bool { return chain[i].MinutesToExpiry < chain[j].MinutesToExpiry })
	return chain, nil
}

// rateOf is the newest rate point not after day, as a decimal.
func rateOf(ctx context.Context, s inputs, day contracts.Day) (compute.Point, error) {
	pts, err := s.Series.SeriesPoints(ctx, s.rateSeries, shiftDay(day, -rateLookback), day)
	if err != nil {
		return compute.Point{}, fmt.Errorf("computefigures: rate series: %w", err)
	}
	series := s.ops.FromSeriesPoints(pts)
	for i := len(series) - 1; i >= 0; i-- {
		if series[i].Day > day {
			continue
		}
		p := compute.Point{Day: series[i].Day, Value: s.units.PercentToDecimal(series[i].Value)}
		if daysBetween(p.Day, day) > s.rateMaxAge {
			return p, errStaleRate
		}
		return p, nil
	}
	return compute.Point{}, errNoRate
}
