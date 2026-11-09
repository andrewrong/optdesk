package optioncandidates

import (
	"context"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	calendarID    = "XNYS"
	rateSeries    = "DGS3MO"
	fundsTag      = "AvailableFunds"
	termsChunk    = 500
	lookbackDays  = 14  // calendar days of session days, chain catalog and rate
	earningsDays  = 120 // calendar days of earnings rows read
	captureLookup = 3   // session days searched back for the newest account capture
)

var aliases = []string{"A", "B"}

// underlyingRef names one watched underlying and its instrument id.
type underlyingRef struct {
	issuer, symbol string
	id             int64
	chainDay       contracts.Day
}

func parseDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, string(d))
	return t, err == nil
}

func addDays(d contracts.Day, n int) contracts.Day {
	t, ok := parseDay(d)
	if !ok {
		return d
	}
	return contracts.Day(t.AddDate(0, 0, n).Format(time.DateOnly))
}

// sessionDays lists the XNYS session days of the lookback window, ascending.
func sessionDays(ctx context.Context, rd store.SessionDayReader, day contracts.Day) ([]contracts.Day, error) {
	rows, err := rd.SessionDays(ctx, calendarID, addDays(day, -lookbackDays), day)
	if err != nil {
		return nil, err
	}
	var out []contracts.Day
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(time.DateOnly)))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// trimSessionDays keeps the days from the oldest of the input dates on; an input older than the list keeps it whole.
func trimSessionDays(all []contracts.Day, inputs ...contracts.Day) []contracts.Day {
	oldest := contracts.Day("")
	for _, d := range inputs {
		if d != "" && (oldest == "" || d < oldest) {
			oldest = d
		}
	}
	i := sort.Search(len(all), func(i int) bool { return all[i] >= oldest })
	return all[i:]
}

// locate finds the watched US stock entries among the underlyings the chain catalog holds: the id of a symbol
// comes from the instrument names, the chain day from the catalog.
func (j *job) locate(ctx context.Context, day contracts.Day, us []jobs.WatchEntry) (found []underlyingRef, missing []string, err error) {
	latest, err := j.st.Catalog.OptionEodLatest(ctx, addDays(day, -lookbackDays))
	if err != nil {
		return nil, nil, err
	}
	ids := make([]int64, 0, len(latest))
	chain := map[int64]contracts.Day{}
	for _, l := range latest {
		ids = append(ids, l.UnderlyingID)
		chain[l.UnderlyingID] = l.SessionDay
	}
	// The catalog gives the greatest chain day with no upper bound: a rerun of a past day looks for the newest
	// chain on or before the run day instead (Decision 6).
	for _, l := range latest {
		if l.SessionDay <= day {
			continue
		}
		prior, err := j.priorChainDay(ctx, l.UnderlyingID, day)
		if err != nil {
			return nil, nil, err
		}
		if prior == "" {
			delete(chain, l.UnderlyingID)
		} else {
			chain[l.UnderlyingID] = prior
		}
	}
	bySymbol := map[string]int64{}
	for start := 0; start < len(ids); start += termsChunk {
		end := min(start+termsChunk, len(ids))
		names, err := j.st.Names.InstrumentNames(ctx, ids[start:end])
		if err != nil {
			return nil, nil, err
		}
		for _, n := range names {
			if n.Type == "stock" {
				bySymbol[strings.ToUpper(n.Name)] = n.InstrumentID
			}
		}
	}
	for _, e := range us {
		id, ok := bySymbol[strings.ToUpper(e.Contract.Symbol)]
		if !ok {
			missing = append(missing, e.Contract.Symbol)
			continue
		}
		if _, has := chain[id]; !has {
			missing = append(missing, e.Contract.Symbol)
			continue
		}
		found = append(found, underlyingRef{issuer: e.Issuer, symbol: e.Contract.Symbol, id: id, chainDay: chain[id]})
	}
	return found, missing, nil
}

// priorChainDay is the newest session day on or before day, within the look-back, that holds rows of the
// underlying; empty when there is none.
func (j *job) priorChainDay(ctx context.Context, id int64, day contracts.Day) (contracts.Day, error) {
	rows, err := j.st.OptionEod.OptionEod(ctx, id, addDays(day, -lookbackDays), day)
	if err != nil {
		return "", err
	}
	best := contracts.Day("")
	for _, r := range rows {
		if d := contracts.Day(r.SessionDay.Format(time.DateOnly)); d <= day && d > best {
			best = d
		}
	}
	return best, nil
}

// quotes reads the rows of the chain day, the terms of their contracts (in chunks of 500 ids) and joins them
// into the puts of the chain. spot is the underlying price of the chain, empty when no row carries one.
func (j *job) quotes(ctx context.Context, ul underlyingRef) (spot string, out []compute.OptionQuote, err error) {
	rows, err := j.st.OptionEod.OptionEod(ctx, ul.id, ul.chainDay, ul.chainDay)
	if err != nil {
		return "", nil, err
	}
	// The last row of an instrument (by generation, then capture) is its newest capture of the day.
	best := map[int]contracts.OptionEod{}
	for _, r := range rows {
		if old, ok := best[r.InstrumentID]; !ok || r.Generation > old.Generation ||
			(r.Generation == old.Generation && r.CaptureID >= old.CaptureID) {
			best[r.InstrumentID] = r
		}
		if spot == "" {
			spot = optText(r.UnderlyingPrice)
		}
	}
	ids := make([]int64, 0, len(best))
	for id := range best {
		ids = append(ids, int64(id))
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	for start := 0; start < len(ids); start += termsChunk {
		end := min(start+termsChunk, len(ids))
		terms, err := j.st.Terms.OptionTerms(ctx, ids[start:end])
		if err != nil {
			return "", nil, err
		}
		for _, t := range terms {
			if t.Right != "P" {
				continue
			}
			r := best[int(t.InstrumentID)]
			out = append(out, compute.OptionQuote{
				InstrumentID: strconv.FormatInt(t.InstrumentID, 10), Expiry: t.ExpirationDate, Right: t.Right,
				Strike: t.Strike, Multiplier: t.Multiplier, Bid: optText(r.Bid), Ask: optText(r.Ask),
				BidSize: intOf(r.BidSize), OpenInterest: intOf(r.OpenInterest), Delta: optText(r.Delta),
				IV: optText(r.IvDecimal), MarketDataType: r.MarketDataType, QuoteOK: r.Status == 1,
			})
		}
	}
	return spot, out, nil
}

func intOf(v *float64) int {
	if v == nil || *v < 0 {
		return 0
	}
	return int(*v)
}

// earnings reads the newest earnings row of the issuer and the newest listed date before the chain day. A nil
// reader gives a nil info, which the rule answers with an unknown gate.
func (j *job) earnings(ctx context.Context, e jobs.WatchEntry, chainDay contracts.Day) (*compute.EarningsInfo, chainEarnings, error) {
	none := chainEarnings{state: "none"}
	if j.st.Earnings == nil {
		return nil, none, nil
	}
	rows, err := j.st.Earnings.EarningsDates(ctx, e.Issuer, addDays(chainDay, -earningsDays), chainDay)
	if err != nil {
		return nil, none, err
	}
	info := &compute.EarningsInfo{Override: contracts.Day(e.ReportDateOverride)}
	if len(rows) == 0 {
		info.State = "absent"
		return info, chainEarnings{state: "none", next: info.Override}, nil
	}
	for _, r := range rows {
		if string(r.Status) != "listed" || r.ReportDate == nil {
			continue
		}
		if d := contracts.Day(r.ReportDate.Format(time.DateOnly)); d < chainDay && d > info.Previous {
			info.Previous = d
		}
	}
	// The newest row is the one observed last; the reader's order is not relied on (ties keep the later row).
	last := rows[0]
	for _, r := range rows[1:] {
		if !r.ObservedOn.Before(last.ObservedOn.Time) {
			last = r
		}
	}
	info.State = string(last.Status)
	info.RowDay = contracts.Day(last.ObservedOn.Format(time.DateOnly))
	if string(last.Status) == "listed" && last.ReportDate != nil {
		info.Next = contracts.Day(last.ReportDate.Format(time.DateOnly))
	}
	ce := chainEarnings{state: info.State, next: info.Next}
	if info.Override != "" {
		ce.next = info.Override
	}
	return info, ce, nil
}

// accounts reads the newest capture of each alias, A and B separately. An alias with no capture in the
// lookup window is left out; a nil reader gives no alias at all.
func (j *job) accounts(ctx context.Context, symbol string, days []contracts.Day) ([]compute.AliasFunds, error) {
	if j.st.Accounts == nil {
		return nil, nil
	}
	var out []compute.AliasFunds
	for _, alias := range aliases {
		for tries, i := 0, len(days)-1; i >= 0 && tries < captureLookup; tries, i = tries+1, i-1 {
			values, err := j.st.Accounts.Values(ctx, alias, string(days[i]))
			if err != nil {
				return nil, err
			}
			if len(values) == 0 {
				continue
			}
			positions, err := j.st.Accounts.Positions(ctx, alias, string(days[i]))
			if err != nil {
				return nil, err
			}
			out = append(out, aliasFunds(alias, symbol, days[i], values, positions))
			break
		}
	}
	return out, nil
}

func aliasFunds(alias, symbol string, day contracts.Day, values []store.AccountValue, positions []store.Position) compute.AliasFunds {
	f := compute.AliasFunds{Alias: alias, CaptureDay: day}
	for _, v := range values {
		if v.Tag == fundsTag {
			f.Funds, f.Currency = v.Value, v.Currency
		}
	}
	for _, p := range positions {
		if !strings.EqualFold(p.Symbol, symbol) {
			continue
		}
		q, ok := new(big.Rat).SetString(p.Quantity)
		if !ok {
			continue
		}
		n := int(new(big.Int).Quo(q.Num(), q.Denom()).Int64())
		switch {
		case p.SecType == "STK":
			f.Shares += n
			f.AvgCost = p.AvgCost
		case p.SecType == "OPT" && p.Right == "call" && n < 0:
			f.OpenShortCalls += -n
		}
	}
	return f
}

// rate reads the newest DGS3MO point of the window: its text in percent and its day. A nil reader, no point or
// a value that is not a rate gives the empty text.
func (j *job) rate(ctx context.Context, day contracts.Day) (string, contracts.Day, error) {
	if j.st.Series == nil {
		return "", "", nil
	}
	pts, err := j.st.Series.SeriesPoints(ctx, rateSeries, addDays(day, -lookbackDays), day)
	if err != nil {
		return "", "", err
	}
	var bestDay contracts.Day
	var text string
	for _, p := range pts {
		d := contracts.Day(p.ObservationDate.Format(time.DateOnly))
		if p.Value == nil || d < bestDay {
			continue
		}
		bestDay, text = d, FloatToDecimal(*p.Value)
	}
	if _, ok := RateRatio(text); !ok {
		return "", "", nil
	}
	return text, bestDay, nil
}
