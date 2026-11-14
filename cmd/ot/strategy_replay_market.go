package main

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// eodOK is status 1 of obs.option_eod.
const eodOK = 1

const (
	basisTrades   = "trades"
	basisAdjusted = "adjusted_last"
)

type eodKey struct {
	instrument int
	day        contracts.Day
}

// resolveMarket resolves the stored option of every lot over its date range and reads what the replay needs:
// the ok quote per contract and day (the latest received_at; equal times, the larger capture id), and the trades
// and adjusted_last closes of the entry and settle days. It returns the lots with InstrumentID set where an option
// matched. A day the store lacks is absent from the Market, never filled. An underlying that never had an
// option_eod row has no instrument id here and so no close: its lots are counted as no_settle_price.
func resolveMarket(ctx context.Context, rd markReaders, bars bar.PriceBarReader, lots []replayapi.Lot) (replayapi.Market, []replayapi.Lot, error) {
	mkt := replayapi.Market{
		Quotes:         map[string]map[contracts.Day]replayapi.Quote{},
		Closes:         map[string]map[contracts.Day]string{},
		AdjustedCloses: map[string]map[contracts.Day]string{},
	}
	out := append([]replayapi.Lot(nil), lots...)
	if len(out) == 0 {
		return mkt, out, nil
	}
	symbols := map[string]bool{}
	from, to := out[0].EntryDay, out[0].Expiry
	for _, l := range out {
		symbols[l.Underlying] = true
		from, to = min(from, l.EntryDay), max(to, l.Expiry)
	}
	idOf, err := underlyingIDs(ctx, rd, symbols, from)
	if err != nil {
		return mkt, out, err
	}
	best, nameOf, instruments, err := readQuotes(ctx, rd, idOf, from, to)
	if err != nil {
		return mkt, out, err
	}
	keyed, err := instrumentsByKey(ctx, rd, instruments, nameOf)
	if err != nil {
		return mkt, out, err
	}
	for i, l := range out {
		inst, ok := keyed[markKeyOf(l.Underlying, string(l.Expiry), l.Strike, l.Right)]
		if !ok {
			continue
		}
		id := strconv.Itoa(inst)
		out[i].InstrumentID = id
		addQuotes(mkt.Quotes, id, inst, best)
	}
	if err := readCloses(ctx, bars, idOf, out, &mkt); err != nil {
		return mkt, out, err
	}
	return mkt, out, nil
}

// newerQuote is true when a is received later than b; equal times go to the larger capture id.
func newerQuote(a, b contracts.OptionEod) bool {
	if !a.ReceivedAt.Equal(b.ReceivedAt) {
		return a.ReceivedAt.After(b.ReceivedAt)
	}
	return a.CaptureID > b.CaptureID
}

// underlyingIDs names the instrument ids of the symbols among the underlyings that have option_eod rows from since.
func underlyingIDs(ctx context.Context, rd markReaders, symbols map[string]bool, since contracts.Day) (map[string]int64, error) {
	latest, err := rd.catalog.OptionEodLatest(ctx, since)
	if err != nil {
		return nil, errors.New("read the option_eod catalog failed")
	}
	ids := make([]int64, 0, len(latest))
	for _, l := range latest {
		ids = append(ids, l.UnderlyingID)
	}
	out := map[string]int64{}
	for lo := 0; lo < len(ids); lo += ledgerBatch {
		got, err := rd.names.InstrumentNames(ctx, ids[lo:min(lo+ledgerBatch, len(ids))])
		if err != nil {
			return nil, errors.New("read the instrument names failed")
		}
		for _, n := range got {
			if _, dup := out[n.Name]; symbols[n.Name] && !dup {
				out[n.Name] = n.InstrumentID
			}
		}
	}
	return out, nil
}

// instrumentsByKey maps the contract key of every instrument to its id; the lowest id wins a key.
func instrumentsByKey(ctx context.Context, rd markReaders, instruments []int64, nameOf map[int]string) (map[markKey]int, error) {
	out := map[markKey]int{}
	for lo := 0; lo < len(instruments); lo += ledgerBatch {
		got, err := rd.terms.OptionTerms(ctx, instruments[lo:min(lo+ledgerBatch, len(instruments))])
		if err != nil {
			return nil, errors.New("read the option terms failed")
		}
		for _, t := range got {
			k := markKeyOf(nameOf[int(t.InstrumentID)], string(t.ExpirationDate), t.Strike, t.Right)
			if old, had := out[k]; !had || int(t.InstrumentID) < old {
				out[k] = int(t.InstrumentID)
			}
		}
	}
	return out, nil
}

// readCloses reads the trades and adjusted_last closes of the entry and settle days of the lots; a day the store
// lacks stays absent.
func readCloses(ctx context.Context, bars bar.PriceBarReader, idOf map[string]int64, lots []replayapi.Lot, mkt *replayapi.Market) error {
	type span struct {
		from, to contracts.Day
		days     map[contracts.Day]bool
	}
	spans := map[string]*span{}
	for _, l := range lots {
		s := spans[l.Underlying]
		if s == nil {
			s = &span{from: l.EntryDay, to: l.SettleDay, days: map[contracts.Day]bool{}}
			spans[l.Underlying] = s
		}
		s.from, s.to = min(s.from, l.EntryDay), max(s.to, l.SettleDay)
		s.days[l.EntryDay], s.days[l.SettleDay] = true, true
	}
	for _, name := range sortedKeys(spans) {
		id, ok := idOf[name]
		if !ok {
			continue
		}
		s := spans[name]
		for _, basis := range []string{basisTrades, basisAdjusted} {
			rows, err := bars.PriceBars(ctx, id, basis, s.from, s.to)
			if err != nil {
				return errors.New("read price bars failed")
			}
			dst := mkt.Closes
			if basis == basisAdjusted {
				dst = mkt.AdjustedCloses
			}
			for _, r := range rows {
				day := contracts.Day(r.SessionDay.Format(dayLayout))
				if !s.days[day] || r.Close == "" {
					continue
				}
				if dst[name] == nil {
					dst[name] = map[contracts.Day]string{}
				}
				dst[name][day] = r.Close
			}
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// readQuotes reads option_eod of each underlying and keeps per contract and day the ok row of the latest
// received_at (equal times: the larger capture id). It also returns the underlying name of each instrument and
// the instruments seen, any status, sorted.
func readQuotes(ctx context.Context, rd markReaders, idOf map[string]int64,
	from, to contracts.Day) (map[eodKey]contracts.OptionEod, map[int]string, []int64, error) {
	best := map[eodKey]contracts.OptionEod{}
	nameOf := map[int]string{}
	var instruments []int64
	for _, name := range sortedKeys(idOf) {
		rows, err := rd.eod.OptionEod(ctx, idOf[name], from, to)
		if err != nil {
			return nil, nil, nil, errors.New("read option_eod failed")
		}
		for _, r := range rows {
			if _, seen := nameOf[r.InstrumentID]; !seen {
				instruments = append(instruments, int64(r.InstrumentID))
			}
			nameOf[r.InstrumentID] = name
			if r.Status != eodOK {
				continue
			}
			k := eodKey{r.InstrumentID, contracts.Day(r.SessionDay.Format(dayLayout))}
			if old, had := best[k]; !had || newerQuote(r, old) {
				best[k] = r
			}
		}
	}
	slices.Sort(instruments)
	return best, nameOf, instruments, nil
}

// addQuotes puts the chosen rows of one instrument, those with an ask, into quotes under the instrument id text.
func addQuotes(quotes map[string]map[contracts.Day]replayapi.Quote, id string, inst int, best map[eodKey]contracts.OptionEod) {
	if _, done := quotes[id]; done {
		return
	}
	for k, r := range best {
		ask := decimalOf(r.Ask)
		if k.instrument != inst || ask == "" {
			continue
		}
		if quotes[id] == nil {
			quotes[id] = map[contracts.Day]replayapi.Quote{}
		}
		quotes[id][k.day] = replayapi.Quote{Bid: decimalOf(r.Bid), Ask: ask}
	}
}
