package main

import (
	"context"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/companies_page"
	"github.com/andrewrong/optdesk/internal/pages/market_state_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// Design: docs/design/valuation.md#valuation-wire-us (Decision 13).

// valuationFuncs are the seven compute functions of valuationapi, the real ones from internal/compute/valuation in
// cmd/ot and fakes in a test. One StandardOf serves the figures job and the page.
type valuationFuncs struct {
	StandardOf    valuationapi.StandardOfFunc
	BuildFilings  valuationapi.BuildFilingsFunc
	Band          valuationapi.BandFunc
	ImpliedGrowth valuationapi.ImpliedGrowthFunc
	ReturnAtPrice valuationapi.ReturnAtPriceFunc
	Questions     valuationapi.QuestionsFunc
	Diagnosis     valuationapi.DiagnosisFunc
}

// companiesInputs are the readers of companies_page that entry 24 leaves nil. A nil reader leaves its field of
// the page Config unset, which the page shows as a gap.
type companiesInputs struct {
	Facts   store.FactReader
	Series  store.SeriesReader
	Catalog store.OptionEodCatalog
	Names   store.InstrumentNameReader
	Eod     store.OptionEodReader
	Terms   store.OptionTermsReader
	Stocks  market_state_page.StockIDReader
	Bars    bar.PriceBarReader
}

// quoteLookbackDays is how far before the page day a quote or an option_eod row is looked for.
const quoteLookbackDays = 7

// spotLookbackDays is how far before the page day a daily close is looked for.
const spotLookbackDays = 14

// nameBatch is the most ids one InstrumentNames call takes.
const nameBatch = 500

// symbolIDs finds the instrument id of an underlying symbol: the catalog lists the ids with option_eod rows, the
// name reader names them, the id whose name equals the symbol is kept (a stock before any other type). A symbol
// the catalog does not give is asked of the optional stock reader by name, and its id is used only when exactly
// one row answers. The catalog answer is read once per resolver; a stock answer is kept per symbol.
type symbolIDs struct {
	catalog store.OptionEodCatalog
	names   store.InstrumentNameReader
	stocks  market_state_page.StockIDReader
	once    sync.Once
	ids     map[string]int64
	err     error
	mu      sync.Mutex
	byStock map[string]int64 // 0 is a symbol the stock reader did not give once
}

func (r *symbolIDs) load(ctx context.Context) {
	r.ids = map[string]int64{}
	list, err := r.catalog.OptionEodLatest(ctx, "")
	if err != nil {
		r.err = err
		return
	}
	isStock := map[string]bool{}
	underlying := make([]int64, 0, len(list))
	for _, e := range list {
		underlying = append(underlying, e.UnderlyingID)
	}
	r.err = inBatches(underlying, func(ids []int64) error {
		got, err := r.names.InstrumentNames(ctx, ids)
		for _, n := range got {
			stock := n.Type == "stock"
			if _, seen := r.ids[n.Name]; seen && (isStock[n.Name] || !stock) {
				continue
			}
			r.ids[n.Name], isStock[n.Name] = n.InstrumentID, stock
		}
		return err
	})
}

// inBatches calls fn with consecutive slices of items of at most nameBatch, and stops at the first error.
func inBatches[T any](items []T, fn func([]T) error) error {
	for i := 0; i < len(items); i += nameBatch {
		if err := fn(items[i:min(i+nameBatch, len(items))]); err != nil {
			return err
		}
	}
	return nil
}

// ID returns the instrument id of symbol; ok is false for a symbol with no match.
func (r *symbolIDs) ID(ctx context.Context, symbol string) (id int64, ok bool, err error) {
	r.once.Do(func() { r.load(ctx) })
	if r.err != nil {
		return 0, false, r.err
	}
	if id, ok = r.ids[symbol]; ok || r.stocks == nil {
		return id, ok, nil
	}
	return r.stockID(ctx, symbol)
}

// stockID asks the stock reader for symbol; only exactly one row gives an id.
func (r *symbolIDs) stockID(ctx context.Context, symbol string) (int64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, seen := r.byStock[symbol]; seen {
		return id, id != 0, nil
	}
	got, err := r.stocks.StockIDs(ctx, []string{symbol})
	if err != nil {
		return 0, false, err
	}
	var id int64
	if len(got) == 1 {
		id = got[0].InstrumentID
	}
	if r.byStock == nil {
		r.byStock = map[string]int64{}
	}
	r.byStock[symbol] = id
	return id, id != 0, nil
}

func dayBefore(day contracts.Day, n int) (contracts.Day, error) {
	t, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return "", err
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format(dayLayout)), nil
}

// spotOf returns the Spot function: the newest daily close (basis trades) in the two weeks up to the day, as the
// decimal text the bar stores, with the session day of that bar. A symbol with no instrument or no bar gives ok
// false, a gap.
func spotOf(res *symbolIDs, bars bar.PriceBarReader) func(context.Context, string, contracts.Day) (companies_page.Spot, bool, error) {
	return func(ctx context.Context, symbol string, day contracts.Day) (companies_page.Spot, bool, error) {
		id, ok, err := res.ID(ctx, symbol)
		if err != nil || !ok {
			return companies_page.Spot{}, false, err
		}
		from, err := dayBefore(day, spotLookbackDays)
		if err != nil {
			return companies_page.Spot{}, false, err
		}
		rows, err := bars.PriceBars(ctx, id, "trades", from, day)
		if err != nil || len(rows) == 0 {
			return companies_page.Spot{}, false, err
		}
		last := rows[len(rows)-1]
		return companies_page.Spot{Price: last.Close, Day: contracts.Day(last.SessionDay.Format(dayLayout))}, true, nil
	}
}

// offersOf returns the Offers function: the strikes of the puts of the latest option_eod session in the week up
// to the day that have not expired, ascending. The option_eod row has no strike; the terms reader gives it.
func offersOf(res *symbolIDs, eod store.OptionEodReader, terms store.OptionTermsReader) func(context.Context, string, contracts.Day) ([]string, error) {
	return func(ctx context.Context, symbol string, day contracts.Day) ([]string, error) {
		id, ok, err := res.ID(ctx, symbol)
		if err != nil || !ok {
			return nil, err
		}
		from, err := dayBefore(day, quoteLookbackDays)
		if err != nil {
			return nil, err
		}
		rows, err := eod.OptionEod(ctx, id, from, day)
		if err != nil {
			return nil, err
		}
		var last string
		for _, r := range rows {
			if d := r.SessionDay.Format(dayLayout); d > last {
				last = d
			}
		}
		seen := map[int64]bool{}
		var optIDs []int64
		for _, r := range rows {
			if r.SessionDay.Format(dayLayout) == last && !seen[int64(r.InstrumentID)] {
				seen[int64(r.InstrumentID)] = true
				optIDs = append(optIDs, int64(r.InstrumentID))
			}
		}
		// A strike stays the text the terms give: it is deduplicated as text and ordered as a decimal.
		have := map[string]*big.Rat{}
		err = inBatches(optIDs, func(ids []int64) error {
			got, err := terms.OptionTerms(ctx, ids)
			for _, t := range got {
				k, ok := new(big.Rat).SetString(t.Strike)
				if t.Right == "P" && ok && string(t.ExpirationDate) > string(day) {
					have[t.Strike] = k
				}
			}
			return err
		})
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(have))
		for k := range have {
			out = append(out, k)
		}
		sort.Slice(out, func(i, j int) bool {
			if c := have[out[i]].Cmp(have[out[j]]); c != 0 {
				return c < 0
			}
			return out[i] < out[j]
		})
		return out, nil
	}
}
